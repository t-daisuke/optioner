package e2e

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "optioner-e2e")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "optioner")
	out, err := exec.Command("go", "build", "-o", binPath, "../cmd/optioner").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// startServer は optioner を起動し、URL(stdout 1 行目)と
// 「プロセス終了後に残りの stdout を返す関数」を返す。
func startServer(t *testing.T, args ...string) (*exec.Cmd, string, func() string) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Stderr = os.Stderr
	url, rest := start(t, cmd)
	return cmd, url, rest
}

// start は組み立て済みの cmd を起動する。stderr や環境変数を差し替えたい
// テストのために startServer から分けてある。
func start(t *testing.T, cmd *exec.Cmd) (string, func() string) {
	t.Helper()
	// StdoutPipe は使わない: cmd.Wait() が「プロセス終了を見た時点で」パイプを
	// 閉じるため、終了時の stdout エコー(TestAnswerFlowAndCleanShutdown の手順 5)を
	// 取りこぼす競合がある。自前の os.Pipe なら書き込み端を持つのは子プロセスだけで、
	// 終了 = EOF となり、ドレインゴルーチンが必ず全量を読み切れる。
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = pw
	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		t.Fatal(err)
	}
	pw.Close() // 親側の複製は閉じる(書き込み端を持つのは子だけ)
	// テストが途中で落ちてもサーバプロセスを残さない((ゾンビ厳禁ルール))。
	// 正常系では既に終了済みなので Kill はエラーになるだけで無害。
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	urlCh := make(chan string, 1)
	var rest strings.Builder
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		defer pr.Close()
		sc := bufio.NewScanner(pr)
		if sc.Scan() {
			urlCh <- sc.Text()
		}
		close(urlCh)
		for sc.Scan() {
			rest.WriteString(sc.Text())
			rest.WriteByte('\n')
		}
	}()
	restFn := func() string { <-drained; return rest.String() }
	select {
	case u, ok := <-urlCh:
		if !ok || !strings.HasPrefix(u, "http://127.0.0.1:") {
			t.Fatalf("expected URL on first stdout line, got %q", u)
		}
		return u, restFn
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("timed out waiting for URL on stdout")
		return "", nil
	}
}

// waitExit はプロセスが timeout 以内に終了することを検証し、exit code を返す。
func waitExit(t *testing.T, cmd *exec.Cmd, timeout time.Duration) int {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return cmd.ProcessState.ExitCode()
	case <-time.After(timeout):
		cmd.Process.Kill()
		t.Fatal("process did not exit in time (zombie server!)")
		return -1
	}
}

func TestInvalidSpecFailsFast(t *testing.T) {
	cmd := exec.Command(binPath, "testdata/invalid-type.json")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for invalid spec")
	}
	if code := cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(string(out), "questions[0].type") {
		t.Fatalf("expected error message to point at questions[0].type, got:\n%s", out)
	}
}

// フラグは位置引数より前に置く規約(Go 慣習)。位置引数の後ろに置かれたフラグを
// 黙って無視する(= -no-open や -heartbeat-timeout が効かない)ことは許さず、
// usage error として exit 2 で落とす。spec の中身は読まれる前に落ちるので何でもよい。
func TestTrailingArgsAreUsageError(t *testing.T) {
	cmd := exec.Command(binPath, "testdata/invalid-type.json", "-no-open")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit when flags follow the positional arg")
	}
	if code := cmd.ProcessState.ExitCode(); code != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %d, output:\n%s", code, out)
	}
}

// -heartbeat-timeout 0(や負値)は「最初のビートで即シャットダウン」= 人間が
// 答える前にページが死ぬ設定で、意図した指定ではありえない。黙って既定値に
// 丸めず、listen もブラウザ起動もする前に usage error で落とす。
// spec は正しいものを渡す: 落ちる理由がフラグだけであることを確かめるため。
// (-no-open はガードが壊れたときに実ブラウザを開かせないための保険)
func TestNonPositiveHeartbeatTimeoutIsUsageError(t *testing.T) {
	cmd := exec.Command(binPath, "-no-open", "-heartbeat-timeout", "0s", "../examples/db-choice.json")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit for -heartbeat-timeout 0s")
	}
	if code := cmd.ProcessState.ExitCode(); code != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %d, output:\n%s", code, out)
	}
	if !strings.Contains(string(out), "heartbeat-timeout") {
		t.Fatalf("the message must name the flag that was wrong, got:\n%s", out)
	}
}

func TestAnswerFlowAndCleanShutdown(t *testing.T) {
	cmd, url, restOfStdout := startServer(t, "-no-open", "-heartbeat-timeout", "30s", "../examples/db-choice.json")

	// 1) top page renders the spec
	res, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	page, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.Contains(string(page), "Choose a database") {
		t.Fatalf("top page broken: status %d, body:\n%s", res.StatusCode, page)
	}

	// 2) submit answers, clipboard text comes back
	res, err = http.Post(url+"/api/answers", "application/json",
		strings.NewReader(`{"answers": {"db": "SQLite"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Clipboard string `json:"clipboard"`
	}
	err = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || !strings.Contains(out.Clipboard, "SQLite") {
		t.Fatalf("answers broken: status %d, clipboard:\n%s", res.StatusCode, out.Clipboard)
	}

	// 3) clipboard text is re-servable and identical
	res, err = http.Get(url + "/api/clipboard")
	if err != nil {
		t.Fatal(err)
	}
	clip, err := io.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 || string(clip) != out.Clipboard {
		t.Fatalf("clipboard endpoint must serve the same text (status %d)", res.StatusCode)
	}

	// 4) close → clean exit (no zombie)
	resp, err := http.Post(url+"/api/close", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if code := waitExit(t, cmd, 5*time.Second); code != 0 {
		t.Fatalf("expected clean exit 0 after close, got %d", code)
	}

	// 5) 提出済み回答は終了時に stdout にもエコーされる(エージェントが直接読める第二経路)。
	// stdout エコーは「クリップボードと同一テキスト」が契約なので、部分一致では
	// なく out.Clipboard そのものが含まれることを見る(前置の改行等は許容)。
	if echoed := restOfStdout(); !strings.Contains(echoed, out.Clipboard) {
		t.Fatalf("submitted answers must be echoed to stdout on exit.\nwant (clipboard text):\n%s\ngot stdout:\n%s", out.Clipboard, echoed)
	}
}

// ブラウザが開けない環境(headless / SSH / 既定ブラウザなし)で失敗を黙って
// 飲み込むと、誰もページを開かない → ハートビートが始まらない → ポートを掴んだまま
// プロセスが永久に残る((ゾンビ厳禁ルール違反))。PATH から open を消せば
// 「-no-open なしの起動」を headless に再現できるので、警告が URL 付きで出ること、
// そして起動自体は続行してクリーンに終われることを見る。
func TestBrowserOpenFailureIsReported(t *testing.T) {
	cmd := exec.Command(binPath, "-heartbeat-timeout", "30s", "../examples/db-choice.json")
	// os/exec は環境の重複キーを後勝ちで畳むので、append で PATH を上書きできる。
	cmd.Env = append(os.Environ(), "PATH=/nonexistent")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	url, _ := start(t, cmd)

	// 起動は続く: ページは普通に配信されている(URL さえ分かれば人が開ける)。
	res, err := http.Get(url + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("server must keep serving when the browser cannot be opened, got %d", res.StatusCode)
	}

	resp, err := http.Post(url+"/api/close", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if code := waitExit(t, cmd, 5*time.Second); code != 0 {
		t.Fatalf("expected clean exit 0, got %d", code)
	}
	// cmd.Wait() 済みなのでバッファを読んでよい(コピーゴルーチンは合流済み)。
	if msg := stderr.String(); !strings.Contains(msg, "could not open a browser") || !strings.Contains(msg, url) {
		t.Fatalf("a browser that fails to open must be reported on stderr with the URL to open by hand.\ngot stderr:\n%s", msg)
	}
}

func TestHeartbeatLossKillsServer(t *testing.T) {
	cmd, url, _ := startServer(t, "-no-open", "-heartbeat-timeout", "500ms", "../examples/db-choice.json")
	resp, err := http.Post(url+"/api/heartbeat", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	// 以降ハートビートを送らない = タブが閉じた状況
	if code := waitExit(t, cmd, 5*time.Second); code != 0 {
		t.Fatalf("expected clean exit 0 after heartbeat loss, got %d", code)
	}
}
