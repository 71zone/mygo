//go:build !js

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/wait"
)

// TestWindowOpenKeepsOpener checks that a window the open handler allows
// keeps its relation to the page that opened it, as in a browser: sign-in
// pages answer through window.opener and postMessage, and close themselves.
// The pages come from a web server, as a sign-in's do: on Windows, pages a
// custom scheme serves open in independent windows. Linux opens independent
// pages.
func TestWindowOpenKeepsOpener(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("windows opened by a page are independent pages on Linux")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>page</title><script>window.run = true</script>")
	}))
	defer srv.Close()
	opener := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	created := make(chan *mygo.Window, 4)
	off := mygo.App.OnWindowCreated(func(c *mygo.Window) { created <- c })
	defer off()
	opener.Page().SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		return &mygo.WindowOptions{Width: 320, Height: 240}
	})
	if err := opener.Page().LoadURL(srv.URL + "/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, opener, "window.run")
	if _, err := opener.Page().Eval(`window.got = [];
		window.addEventListener('message', (e) => window.got.push(e.data + '@' + (e.origin === location.origin)));
		localStorage.setItem('popup-store', 'shared')`); err != nil {
		t.Fatal(err)
	}

	// open runs script in the opener, which opens a window, and returns it
	// once its page has loaded.
	open := func(script, search string) *mygo.Window {
		t.Helper()
		if _, err := opener.Page().Eval(script); err != nil {
			t.Fatal(err)
		}
		var child *mygo.Window
		select {
		case child = <-created:
		case <-time.After(wait.Guard):
			t.Fatalf("%q opened no window", script)
		}
		t.Cleanup(child.Destroy)
		waitFor(t, child, "window.run && location.search === '"+search+"'")
		return child
	}
	check := func(w *mygo.Window, what, expr string) {
		t.Helper()
		if ok, err := mygo.EvalAs[bool](w.Page(), expr); err != nil || !ok {
			t.Errorf("%s: %v, %v", what, ok, err)
		}
	}

	child := open("void (window.popup = window.open('/?kid=1', 'kid'))", "?kid=1")
	check(child, "window.opener is the opener's window", "window.opener !== null && window.opener.mygo.windowId === "+strconv.Itoa(opener.ID()))
	check(child, "the child's own bridge", "mygo.windowId === "+strconv.Itoa(child.ID()))
	check(child, "window.opener.location.origin", "window.opener.location.origin === location.origin")
	check(child, "window.name", "window.name === 'kid'")
	check(child, "the opener's store", "localStorage.getItem('popup-store') === 'shared'")
	check(opener, "window.open returned the window", "window.popup !== null && window.popup.closed === false && window.popup.name === 'kid'")

	// Messages go both ways.
	if _, err := child.Page().Eval(`window.inbox = [];
		window.addEventListener('message', (e) => window.inbox.push(e.data));
		window.opener.postMessage('from the child', '*')`); err != nil {
		t.Fatal(err)
	}
	waitFor(t, opener, "window.got.length === 1 && window.got[0] === 'from the child@true'")
	if _, err := opener.Page().Eval("window.popup.postMessage('from the opener', '*')"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, child, "window.inbox.length === 1 && window.inbox[0] === 'from the opener'")

	// A page closes the window it opened.
	if _, err := child.Page().Eval("setTimeout(() => window.close(), 0)"); err != nil {
		t.Log(err) // the window may go before it answers
	}
	eventually(t, "window.close() to close the window", child.IsDestroyed)
	waitFor(t, opener, "window.popup.closed")

	// Links with rel=opener keep the relation, and noopener loses it, as
	// window.open does with the feature.
	if _, err := opener.Page().Eval(`document.body.insertAdjacentHTML('beforeend',
		'<a id="keep" target="_blank" rel="opener" href="/?kid=2">a</a>' +
		'<a id="lose" target="_blank" rel="noopener" href="/?kid=3">b</a>')`); err != nil {
		t.Fatal(err)
	}
	kept := open("document.getElementById('keep').click()", "?kid=2")
	check(kept, "rel=opener link: window.opener", "window.opener !== null && window.opener.mygo.windowId === "+strconv.Itoa(opener.ID()))
	lost := open("document.getElementById('lose').click()", "?kid=3")
	check(lost, "rel=noopener link: window.opener", "window.opener === null")
	none := open("void (window.none = window.open('/?kid=4', '_blank', 'noopener'))", "?kid=4")
	check(none, "noopener feature: window.opener", "window.opener === null")
	check(opener, "noopener feature: window.open returns null", "window.none === null")
}
