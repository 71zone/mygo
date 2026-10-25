//go:build !js

package mygo

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// TestDeliveryNamesTheDocument: what the page is sent goes to the document
// whose dom-ready the core took, which any other document ignores.
func TestDeliveryNamesTheDocument(t *testing.T) {
	ev := newEventForTest[string](t, "gate:doc")
	bindForTest(t, "Doc", &testService{})
	w, fw := testWindow(t, WindowOptions{})
	docs := func() []string {
		var out []string
		for _, s := range fw.Scripts() {
			if d, msgs, ok := delivery(t, s); ok {
				for _, m := range msgs {
					if m["n"] == "gate:doc" {
						out = append(out, d+":"+m["p"].(string))
					}
				}
			}
		}
		return out
	}
	// A reply before the page's dom-ready names no document: it names its own.
	if m := call(t, fw, 1, "Doc.Hello", "x"); m["k"] != "tok" {
		t.Errorf("reply: %v", m)
	}
	for _, s := range fw.Scripts() {
		if d, _, ok := delivery(t, s); ok && d != "" {
			t.Errorf("a delivery before dom-ready names the document %q", d)
		}
	}

	page(fw, `{"t":"dom-ready","k":"doc-1"}`)
	_ = ev.Emit(w, "one")
	received(t, fw, func(m map[string]any) bool { return m["p"] == "one" })
	if got := docs(); !slices.Equal(got, []string{"doc-1:one"}) {
		t.Errorf("events of the first document: %q", got)
	}

	// A navigation: the next document is another, and what was held for it
	// goes to it once it is ready.
	onMain(func() { fw.H.NavigationCommitted("mygo://localhost/next") })
	_ = ev.Emit(w, "two")
	page(fw, `{"t":"dom-ready","k":"doc-2"}`)
	received(t, fw, func(m map[string]any) bool { return m["p"] == "two" })
	if got := docs(); !slices.Equal(got, []string{"doc-1:one", "doc-2:two"}) {
		t.Errorf("events of both documents: %q", got)
	}
}

// TestNavigationDropsTheOutbox: what was queued for a page that is ready is
// not evaluated in the page the window shows after it: an event emitted
// as the navigation committed, which flush has not sent yet.
func TestNavigationDropsTheOutbox(t *testing.T) {
	ev := newEventForTest[string](t, "gate:outbox")
	w, fw := testWindow(t, WindowOptions{})
	page(fw, `{"t":"dom-ready","k":"doc-1"}`)
	fw.ClearScripts()
	onMain(func() {
		_ = ev.Emit(w, "FOR-THE-PAGE-THAT-WENT")
		fw.H.NavigationCommitted("https://evil.example/")
	})
	onMain(func() {})
	for _, s := range fw.Scripts() {
		if strings.Contains(s, "FOR-THE-PAGE-THAT-WENT") {
			t.Errorf("an event of the page that went was evaluated after the navigation: %s", s)
		}
	}
	// What waits for the next page's dom-ready is for that page.
	onMain(func() { fw.H.NavigationCommitted("mygo://localhost/") })
	_ = ev.Emit(w, "FOR-THE-NEXT-PAGE")
	page(fw, `{"t":"dom-ready","k":"doc-2"}`)
	received(t, fw, func(m map[string]any) bool { return m["p"] == "FOR-THE-NEXT-PAGE" })
}

type gateSlow struct{ ran atomic.Int32 }

func (s *gateSlow) Secret(context.Context) string { s.ran.Add(1); return "RESULT-OF-A-CALL" }

// TestReplyOfAGonePageIsNotQueued: a method that returns after its page went
// has nobody to reply to: the page that came after it is another, and
// may not be trusted.
func TestReplyOfAGonePageIsNotQueued(t *testing.T) {
	svc := &gateSlow{}
	bindForTest(t, "GateSlow", svc)
	w, fw := testWindow(t, WindowOptions{})
	raw := `{"t":"call","id":1,"k":"tok","m":"GateSlow.Secret","a":[]}`

	// The control: a call of the page showing is answered.
	handleCall(w, w.pageContext(), raw, true)
	m := received(t, fw, func(m map[string]any) bool { return m["t"] == "reply" && m["id"] == float64(1) })
	if m["v"] != "RESULT-OF-A-CALL" {
		t.Fatalf("the reply of the page's own call: %v", m)
	}
	fw.ClearScripts()

	// The call runs as the window navigates; its context is the page's.
	ctx, cancel := context.WithCancel(w.pageContext())
	cancel() // NavigationCommitted cancels it
	onMain(func() { fw.H.NavigationCommitted("https://evil.example/") })
	handleCall(w, ctx, raw, true)
	onMain(func() {})
	if svc.ran.Load() != 2 {
		t.Fatalf("the method ran %d times, want 2", svc.ran.Load())
	}
	for _, s := range fw.Scripts() {
		if strings.Contains(s, "RESULT-OF-A-CALL") {
			t.Errorf("the reply of a page that went was evaluated: %s", s)
		}
	}
}

// TestProtocolChangeIsJudgedAgain: the decision for the last sender is kept
// only while the schemes the app serves are the same.
func TestProtocolChangeIsJudgedAgain(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	effects := watchEffects(w)
	if err := Protocol.Handle("gatescheme", http.NotFoundHandler()); err != nil {
		t.Fatal(err)
	}
	const sender = "gatescheme://localhost/"
	onMain(func() { fw.H.NavigationCommitted(sender) })
	pageAt(fw, sender, `{"t":"dom-ready"}`)
	onMain(func() {})
	if effects.domReady.Load() != 1 {
		t.Fatalf("a page of a scheme the app serves: OnDOMReady ran %d times", effects.domReady.Load())
	}
	Protocol.Unhandle("gatescheme")
	pageAt(fw, sender, `{"t":"dom-ready"}`)
	onMain(func() {})
	if effects.domReady.Load() != 1 {
		t.Errorf("a sender of a scheme the app stopped serving was still trusted: OnDOMReady ran %d times", effects.domReady.Load())
	}
}

// TestEmptySenderIsNotTrusted: a message without a sender URL is nobody's,
// whatever came before it, also where TrustedOrigins trusts every page.
func TestEmptySenderIsNotTrusted(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Page: PageOptions{TrustedOrigins: []string{"*"}}})
	effects := watchEffects(w)
	onMain(func() { fw.H.NavigationCommitted("https://anything.example/") })
	for _, from := range []string{"", "https://anything.example/", ""} {
		pageAt(fw, from, `{"t":"dom-ready"}`)
	}
	onMain(func() {})
	if n := effects.domReady.Load(); n != 1 {
		t.Errorf("OnDOMReady ran %d times for three messages, one with a URL", n)
	}
}
