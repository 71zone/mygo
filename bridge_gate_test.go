//go:build !js

package mygo

import (
	"net/http"
	"testing"
)

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
