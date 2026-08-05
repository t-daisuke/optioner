// Command optioner serves a decision page for a spec JSON and shuts
// down when the browser tab goes away. It must never leave a zombie.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/t-daisuke/optioner/internal/server"
	"github.com/t-daisuke/optioner/internal/spec"
)

var version = "0.1.0"

func main() { os.Exit(run()) }

func run() int {
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	// 2m, not 30s: browsers throttle background-tab timers to a ~60s grid,
	// and reading option links in another tab is the designed flow.
	hbTimeout := flag.Duration("heartbeat-timeout", 2*time.Minute, "shut down after this long without a browser heartbeat")
	showVersion := flag.Bool("version", false, "print version and exit")
	// One usage text for every way of getting it wrong: -h, an unknown flag
	// (printed by flag itself) and the checks below. The stock text
	// names the binary by its full path, which is noise for a tool that is
	// usually run through npx or go run.
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "usage: optioner [flags] <spec.json>")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return 0
	}
	// Flags must precede the positional arg (Go convention). Anything left over
	// means a flag was silently ignored, which would quietly disable -no-open
	// or -heartbeat-timeout; fail loudly instead.
	if flag.NArg() != 1 {
		flag.Usage()
		return 2
	}
	// A zero or negative timeout would shut the server down on the first
	// heartbeat — the page would die under the human's hands the moment it
	// loads. Nobody means that, so reject it instead of quietly clamping to
	// the default, and do it before anything is listening or opened.
	if *hbTimeout <= 0 {
		fmt.Fprintf(os.Stderr, "invalid -heartbeat-timeout %s: must be positive\n", *hbTimeout)
		flag.Usage()
		return 2
	}

	data, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	s, err := spec.Load(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	// Listen before printing: the URL on stdout line 1 is a promise that the
	// port is already accepting connections, so a caller may connect at once.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)
	fmt.Println(url)

	sv := server.New(s, *hbTimeout)
	httpSrv := &http.Server{Handler: sv.Handler()}
	go func() { _ = httpSrv.Serve(ln) }()

	if !*noOpen {
		openBrowser(url)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sv.Done():
	case <-sig:
	}

	// Graceful, not Close: /api/close writes its 204 before asking for
	// shutdown, and a hard Close would truncate that response mid-flight.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	// The second delivery path, borrowed from difit: submitted answers also go
	// to stdout, so an agent running optioner in the background can read them
	// straight out of the exit output instead of the clipboard.
	if text := sv.Clipboard(); text != "" {
		fmt.Print("\n" + text)
	}
	return 0
}

// openBrowser launches the page and, whichever way that fails, says so with
// the URL to open by hand. A silent failure would be the one reachable way to
// strand this process: nothing loads the page, no heartbeat ever arms the
// shutdown clock, and the port is held forever (the no-zombie rule).
//
// Start, never Run: a wedged "open" must not stall startup, so the exit status
// is collected in a goroutine — which is also what reaps the child instead of
// leaving it defunct for as long as optioner lives.
func openBrowser(url string) {
	cmd := exec.Command("open", url)
	if err := cmd.Start(); err != nil {
		warnNoBrowser(url, err)
		return
	}
	go func() {
		if err := cmd.Wait(); err != nil {
			warnNoBrowser(url, err)
		}
	}()
}

func warnNoBrowser(url string, err error) {
	fmt.Fprintf(os.Stderr, "could not open a browser (%v); open %s manually\n", err, url)
}
