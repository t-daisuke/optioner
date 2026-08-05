package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/t-daisuke/optioner/internal/spec"
)

func mustLoadSpec(t *testing.T, src string) *spec.Spec {
	t.Helper()
	s, err := spec.Load([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func fixtureSpec(t *testing.T) *spec.Spec {
	t.Helper()
	return mustLoadSpec(t, `{
		"optioner": 1,
		"title": "Choose a database",
		"context": "some **context**",
		"questions": [
			{"id": "db", "type": "single_choice", "prompt": "Which DB?",
			 "options": [{"label": "SQLite", "recommended": true}, {"label": "Postgres"}], "allow_other": true}
		]
	}`)
}

func newTestServer(t *testing.T, hb time.Duration) (*Server, *httptest.Server) {
	t.Helper()
	return newTestServerFor(t, fixtureSpec(t), hb)
}

func newTestServerFor(t *testing.T, s *spec.Spec, hb time.Duration) (*Server, *httptest.Server) {
	t.Helper()
	sv := New(s, hb)
	ts := httptest.NewServer(sv.Handler())
	t.Cleanup(ts.Close)
	return sv, ts
}

// getPage は "/" を GET してボディを返す。描画のテストはどれもここから始まる。
func getPage(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	res, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// mustContainAll は欠けているものを全部報告してから、ページを一度だけ出す。
// 「1 つ直しては次で落ちる」を避けたいので Fatalf ではなく Errorf を使う。
func mustContainAll(t *testing.T, body string, wants ...string) {
	t.Helper()
	missing := false
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("page missing %q", want)
			missing = true
		}
	}
	if missing {
		t.Logf("page was:\n%s", body)
	}
}

func TestIndexRendersSpec(t *testing.T) {
	_, ts := newTestServer(t, time.Minute)
	mustContainAll(t, getPage(t, ts), "Choose a database", "Which DB?", "SQLite", "Postgres")
}

func TestIndexRendersFormControls(t *testing.T) {
	_, ts := newTestServer(t, time.Minute)
	// id= の並びは app.js との契約。JS は getElementById で決め打ちに掴むので、
	// テンプレート側で 1 つ消える/綴りが変わるだけで「ボタンが効かない」だけの
	// 壊れ方をする(例外は出ても人間には見えない)。掴んでいる id を全部載せる。
	mustContainAll(t, getPage(t, ts),
		`type="radio"`, `name="db"`, `value="SQLite"`, // single_choice → radio
		`data-role="other"`,        // allow_other → other 入力欄
		`id="answer-form"`,         // submit ハンドラの取り付け先
		`id="copy-button"`,         // コピー動線(ラベルの差し替え先でもある)
		`id="done-button"`,         // 明示 close 動線
		`id="result"`,              // 提出後に開く結果セクション
		`id="clipboard-preview"`,   // コピー対象テキストの置き場
		`src="/assets/app.js"`,     // JS 配線
		`Recommended`,              // recommended バッジ
		`<strong>context</strong>`, // context は render.Markdown を通って HTML になる
		`id="submit-error"`,        // 送信失敗の表示先(JS が getElementById する)
		`id="result-error"`,        // コピー失敗の表示先(同上)
	)
}

// 4 つの質問タイプがそれぞれ違うコントロールになることを見る(このタスクの本体)。
func TestIndexRendersEveryQuestionType(t *testing.T) {
	_, ts := newTestServerFor(t, mustLoadSpec(t, `{
		"optioner": 1,
		"title": "All types",
		"questions": [
			{"id": "why", "type": "free_text", "prompt": "Why?"},
			{"id": "ship", "type": "yes_no", "prompt": "Ship it?"},
			{"id": "db", "type": "single_choice", "prompt": "Which DB?",
			 "options": [{"label": "SQLite"}, {"label": "Postgres"}]},
			{"id": "langs", "type": "multi_choice", "prompt": "Which languages?",
			 "options": [{"label": "Go"}, {"label": "Rust"}]}
		]
	}`), time.Minute)
	mustContainAll(t, getPage(t, ts),
		`data-type="free_text"`, `<textarea name="why"`,
		`data-type="yes_no"`, `name="ship" value="yes"`, `name="ship" value="no"`,
		`data-type="single_choice"`, `type="radio" name="db" value="Postgres"`,
		`data-type="multi_choice"`, `type="checkbox" name="langs" value="Rust"`,
	)
}

// /assets/ は embed FS の根で、テンプレート原本も同居している。ページが要る
// 2 ファイルだけを公開し、それ以外(= サーバの内部)は出さない。
func TestAssetsExposeOnlyPageFiles(t *testing.T) {
	_, ts := newTestServer(t, time.Minute)
	for path, want := range map[string]int{
		"/assets/app.js":         http.StatusOK,
		"/assets/style.css":      http.StatusOK,
		"/assets/page.html.tmpl": http.StatusNotFound,
	} {
		res, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("GET %s: status %d, want %d", path, res.StatusCode, want)
		}
	}
}

func TestAnswersRoundTrip(t *testing.T) {
	_, ts := newTestServer(t, time.Minute)
	res, err := http.Post(ts.URL+"/api/answers", "application/json",
		strings.NewReader(`{"answers": {"db": "SQLite"}}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		Clipboard string `json:"clipboard"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Clipboard, "SQLite") {
		t.Fatalf("clipboard missing answer:\n%s", out.Clipboard)
	}

	res2, err := http.Get(ts.URL + "/api/clipboard")
	if err != nil {
		t.Fatal(err)
	}
	defer res2.Body.Close()
	second, err := io.ReadAll(res2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != out.Clipboard {
		t.Fatal("GET /api/clipboard must serve the same text")
	}
}

func TestCloseSignalsDone(t *testing.T) {
	sv, ts := newTestServer(t, time.Minute)
	res, err := http.Post(ts.URL+"/api/close", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	select {
	case <-sv.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() not closed after /api/close")
	}
}

// ハートビート系は testing/synctest の仮想時計で決定的にテストする。
// synctest バブル内では実ネットワークが使えないので httptest.NewRecorder で直接ハンドラを叩く。

func postHeartbeat(t *testing.T, sv *Server) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/heartbeat", nil)
	req.Host = "127.0.0.1" // NewRequest の既定 Host は example.com で、Host 検証に弾かれる
	rec := httptest.NewRecorder()
	sv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestHeartbeatLossSignalsDone(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sv := New(fixtureSpec(t), 30*time.Second)
		if rec := postHeartbeat(t, sv); rec.Code != http.StatusNoContent {
			t.Fatalf("heartbeat status %d", rec.Code)
		}
		time.Sleep(30*time.Second + time.Second) // 仮想時間でタイムアウトを跨ぐ
		synctest.Wait()
		select {
		case <-sv.Done():
		default:
			t.Fatal("Done() not closed after heartbeat loss")
		}
	})
}

func TestHeartbeatKeepsServerAlive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sv := New(fixtureSpec(t), 30*time.Second)
		for range 10 { // ビートを打ち続ける限り死なない
			postHeartbeat(t, sv)
			time.Sleep(29 * time.Second)
		}
		synctest.Wait()
		select {
		case <-sv.Done():
			t.Fatal("server must stay alive while heartbeats continue")
		default:
		}
	})
}

func TestNoHeartbeatMeansNoTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sv := New(fixtureSpec(t), 30*time.Second)
		time.Sleep(24 * time.Hour) // 一度もビートが来なければ永遠に待つ
		synctest.Wait()
		select {
		case <-sv.Done():
			t.Fatal("server must not shut down before the first heartbeat")
		default:
		}
	})
}

func TestRejectsNonLoopbackHost(t *testing.T) {
	sv := New(fixtureSpec(t), time.Minute)
	req := httptest.NewRequest("GET", "http://evil.example/", nil) // DNS リバインディング相当
	rec := httptest.NewRecorder()
	sv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-loopback Host, got %d", rec.Code)
	}
}

func TestRejectsCrossOriginPOST(t *testing.T) {
	sv := New(fixtureSpec(t), time.Minute)
	req := httptest.NewRequest("POST", "/api/answers", strings.NewReader(`{"answers":{}}`))
	req.Host = "127.0.0.1"
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	sv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-origin POST, got %d", rec.Code)
	}
}
