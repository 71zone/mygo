//go:build !js

package e2e

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

// TestCancelledNavigationIsNoLoadFailure: a navigation the app prevents in
// OnWillNavigate, one another navigation replaces, and one Stop ends are no
// failed loads, and a host that does not resolve, a server that is not
// there, or one that drops the connection, is.
func TestCancelledNavigationIsNoLoadFailure(t *testing.T) {
	// /slow answers late, so that a navigation to it is under way.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
		}
		if r.URL.Path == "/drop" {
			// A server that drops the connection once the request arrived.
			if c, _, err := w.(http.Hijacker).Hijack(); err == nil {
				c.Close()
			}
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>load</title><p>"+r.URL.Path+"</p>")
	}))
	defer srv.Close()
	w := newWindow(t, mygo.WindowOptions{Title: "Load failure", Width: 300, Height: 200, URL: srv.URL + "/ready"})
	waitFor(t, w, "location.pathname === '/ready'")

	var blocked atomic.Int32
	var mu sync.Mutex
	var failed []*mygo.LoadError
	w.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
		if strings.Contains(e.URL, "blocked") {
			blocked.Add(1)
			e.PreventDefault()
		}
	})
	w.Page().OnDidFailLoad(func(e *mygo.LoadError) {
		mu.Lock()
		failed = append(failed, e)
		mu.Unlock()
	})
	failures := func() []*mygo.LoadError {
		mu.Lock()
		defer mu.Unlock()
		return append([]*mygo.LoadError(nil), failed...)
	}
	settle := func() { time.Sleep(1500 * time.Millisecond) }
	// underway returns once the navigation to /slow has started.
	underway := func() {
		t.Helper()
		eventually(t, "the navigation to /slow to start", w.Page().IsLoading)
		time.Sleep(300 * time.Millisecond) // and its request to be on its way
	}

	// Prevented by the app.
	if _, err := w.Page().Eval("location.href = 'http://blocked.invalid/'"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the cancelled navigation to ask OnWillNavigate", func() bool { return blocked.Load() == 1 })
	settle()
	if got := failures(); len(got) != 0 {
		t.Fatalf("a navigation the app prevented failed to load: %+v", got)
	}

	// Replaced by the app's, and by the page's own, navigation.
	w.Page().LoadURL(srv.URL + "/slow")
	underway()
	w.Page().LoadURL(srv.URL + "/replaced")
	waitFor(t, w, "location.pathname === '/replaced'")
	settle()
	if got := failures(); len(got) != 0 {
		t.Fatalf("a navigation the app replaced failed to load: %+v", got)
	}
	if w.Page().IsLoading() {
		t.Error("the page is still loading after the navigation that replaced the other ended")
	}
	if _, err := w.Page().Eval("location.href = '" + srv.URL + "/slow'; setTimeout(() => location.href = '" + srv.URL + "/by-page', 700)"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "location.pathname === '/by-page'")
	settle()
	if got := failures(); len(got) != 0 {
		t.Fatalf("a navigation the page replaced failed to load: %+v", got)
	}

	// Ended by Stop.
	w.Page().LoadURL(srv.URL + "/slow")
	underway()
	w.Page().Stop()
	eventually(t, "Stop to end the load", func() bool { return !w.Page().IsLoading() })
	settle()
	if got := failures(); len(got) != 0 {
		t.Fatalf("a navigation Stop ended failed to load: %+v", got)
	}

	// A lookup that fails, a server that is not there, and one that drops the
	// connection are failed loads. (The web view tries a failed lookup again
	// by itself, and that fails too: the count of failures is not the
	// test's to check.)
	// macOS leaves the URL of a failed load empty.
	failedLoad := func(host string, since int) bool {
		for _, f := range failures()[since:] {
			if f.URL == "" || strings.Contains(f.URL, host) {
				return true
			}
		}
		return false
	}
	step := func(what, host string, navigate func()) {
		t.Helper()
		since := len(failures())
		navigate()
		eventually(t, "the failure of "+what, func() bool { return failedLoad(host, since) })
	}
	step("the unresolvable host", "nonexistent.invalid", func() {
		if _, err := w.Page().Eval("location.href = 'http://nonexistent.invalid/'"); err != nil {
			t.Fatal(err)
		}
	})
	if runtime.GOOS == "windows" { // the web view's own ends of a load: CONNECTION_ABORTED is among them
		step("a server that is not there", "127.0.0.1:1/", func() { w.Page().LoadURL("http://127.0.0.1:1/") })
		step("a server that drops the connection", "/drop", func() { w.Page().LoadURL(srv.URL + "/drop") })
	}
	t.Logf("failed loads: %+v", failures())
}
