//go:build !js

package mygo

import (
	"sync/atomic"
	"testing"
)

// forgeries are the messages a page that has the secret and a way to the
// native channel could post besides calls.
var forgeries = []string{
	`{"t":"dom-ready"}`,
	`{"t":"drag"}`,
	`{"t":"dblclick"}`,
	`{"t":"drop","x":1,"y":2}`,
	`{"t":"chan-ack","c":5,"k":"tok","n":1}`,
	`{"t":"chan-close","c":5,"k":"tok"}`,
	`{"t":"escape"}`, // last: a panel it closes takes the others' effects with it
}

// pageEffects counts what the messages of forgeries did.
type pageEffects struct {
	domReady, drops atomic.Int32
}

func watchEffects(w *Window) *pageEffects {
	e := &pageEffects{}
	w.Page().OnDOMReady(func() { e.domReady.Add(1) })
	w.OnFileDrop(func(*FileDropEvent) { e.drops.Add(1) })
	return e
}

// TestUntrustedMessagesAreIgnored: only trusted pages may post what is not
// a call, whatever they know: the messages do nothing for a page that is
// not trusted, nor for a sender that is not, while the page showing is.
func TestUntrustedMessagesAreIgnored(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Panel: true, Page: PageOptions{TrustedOrigins: []string{"https://partner.example"}}})
	effects := watchEffects(w)
	post := func(from string) {
		for _, msg := range forgeries {
			fw.Dropped = []string{"/tmp/file"}
			pageAt(fw, from, msg)
		}
		onMain(func() {}) // what the messages posted to the main thread has run
	}
	noEffect := func(what string) {
		t.Helper()
		if n := effects.domReady.Load(); n != 0 {
			t.Errorf("%s: OnDOMReady ran %d times", what, n)
		}
		if n := effects.drops.Load(); n != 0 {
			t.Errorf("%s: OnFileDrop ran %d times", what, n)
		}
		if fw.DragStarts != 0 || fw.TitleBarDoubleClicks != 0 {
			t.Errorf("%s: window drags %d, double clicks %d", what, fw.DragStarts, fw.TitleBarDoubleClicks)
		}
		if w.IsDestroyed() {
			t.Errorf("%s: Escape closed the panel", what)
		}
		if len(fw.Dropped) != 1 {
			t.Errorf("%s: the dropped files were taken", what)
		}
	}

	// The page is not trusted.
	onMain(func() { fw.H.NavigationCommitted("https://evil.example/") })
	post("https://evil.example/")
	noEffect("an untrusted page")

	// The page showing is trusted, the sender is not: a frame, or a page
	// that navigated away.
	onMain(func() { fw.H.NavigationCommitted("https://partner.example/") })
	post("https://evil.example/")
	post("")
	noEffect("an untrusted sender")

	// A trusted sender gets what the messages do.
	post("https://partner.example/")
	if effects.domReady.Load() != 1 || effects.drops.Load() != 1 || fw.DragStarts != 1 || fw.TitleBarDoubleClicks != 1 {
		t.Errorf("the messages of a trusted page: ready %d, drops %d, drags %d, double clicks %d",
			effects.domReady.Load(), effects.drops.Load(), fw.DragStarts, fw.TitleBarDoubleClicks)
	}
	if !w.IsDestroyed() {
		t.Error("Escape in a trusted page did not close the panel")
	}
}

// TestUntrustedChannelMessages: a page that is not trusted can neither
// close the channel of another page nor close a channel before it opens.
func TestUntrustedChannelMessages(t *testing.T) {
	s := &streamer{sent: make(chan error, 16), stopped: make(chan error, 1)}
	bindForTest(t, "UntrustedStream", s)
	w, fw := testWindow(t, WindowOptions{})
	page(fw, `{"t":"call","id":1,"k":"tok","m":"UntrustedStream.Wait","a":[9]}`)
	received(t, fw, func(m map[string]any) bool { return m["c"] == float64(9) && m["p"] == "ready" })

	pageAt(fw, "https://evil.example/", `{"t":"chan-close","c":9,"k":"tok"}`)
	pageAt(fw, "https://evil.example/", `{"t":"chan-close","c":12,"k":"tok"}`)
	onMain(func() {})
	select {
	case err := <-s.stopped:
		t.Fatalf("a page that is not trusted closed a channel: %v", err)
	default:
	}
	onMain(func() {
		if len(w.closedEarly) != 0 {
			t.Errorf("a channel was closed before it opened: %v", w.closedEarly)
		}
	})
	page(fw, `{"t":"chan-close","c":9,"k":"tok"}`)
	<-s.stopped
}

// TestUntrustedCallsFail: a call from a sender that is not trusted fails,
// whatever page is showing.
func TestUntrustedCallsFail(t *testing.T) {
	svc := &testService{}
	bindForTest(t, "SenderTrust", svc)
	_, fw := testWindow(t, WindowOptions{})
	pageAt(fw, "https://evil.example/", `{"t":"call","id":1,"k":"tok","m":"SenderTrust.Hello","a":["x"]}`)
	m := received(t, fw, func(m map[string]any) bool { return m["t"] == "reply" && m["id"] == float64(1) })
	if m["ok"] != false || svc.calls.Load() != 0 {
		t.Errorf("a call from another origin: %v, ran %d times", m, svc.calls.Load())
	}
	if m := call(t, fw, 2, "SenderTrust.Hello", "x"); m["ok"] != true {
		t.Errorf("the page's own call: %v", m)
	}
}
