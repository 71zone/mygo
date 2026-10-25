//go:build !js

package e2e

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestStorageID: pages with the same StorageID share their cookies and
// local storage, windows, web views and the windows a page opens alike;
// pages with another StorageID, or none, share none of them with them.
func TestStorageID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<!doctype html><title>storage</title><body>storage</body>")
	}))
	defer server.Close()
	// Every run of the test makes data stores of its own.
	run := strconv.FormatInt(time.Now().UnixNano(), 36)
	idA, idB := "e2e-a-"+run, "e2e-b-"+run
	const state = "(document.cookie.match(/(?:^|; )who=([^;]*)/) || [])[1] + '|' + localStorage.getItem('who')"
	const set = "document.cookie = 'who=%[1]s; path=/; max-age=3600'; localStorage.setItem('who', '%[1]s')"
	const empty = "undefined|null"

	var mu sync.Mutex
	var shown []*mygo.WebView
	var frames atomic.Int32
	w := newWindow(t, mygo.WindowOptions{Title: "Storage", Width: 600, Height: 200, Content: ui.View(func(c *ui.Context) {
		frames.Add(1)
		mu.Lock()
		views := slices.Clone(shown)
		mu.Unlock()
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			for i, v := range views {
				ui.WebView(c.Key(strconv.Itoa(i)), v).Grow(1)
			}
		})
	})})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	open := func(id string) *mygo.WebView {
		t.Helper()
		v, err := w.NewWebView(mygo.WebViewOptions{URL: server.URL, Page: mygo.PageOptions{StorageID: id}})
		if errors.Is(err, errors.ErrUnsupported) {
			t.Skip(err)
		}
		if err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		shown = append(shown, v)
		mu.Unlock()
		w.Invalidate()
		waitForPage(t, v.Page(), "document.title === 'storage'")
		return v
	}
	check := func(what string, p *mygo.Page, want string) {
		t.Helper()
		got, err := mygo.EvalAs[string](p, state)
		if err != nil || got != want {
			t.Errorf("%s: cookie|localStorage %q (%v), want %q", what, got, err, want)
		}
	}

	a, b := open(idA), open(idB)
	check("a fresh store", a.Page(), empty)
	if _, err := a.Page().Eval(fmt.Sprintf(set, "A")); err != nil {
		t.Fatal(err)
	}
	check("the store of A", a.Page(), "A|A")
	check("the store of B after A wrote", b.Page(), empty)
	if _, err := b.Page().Eval(fmt.Sprintf(set, "B")); err != nil {
		t.Fatal(err)
	}
	check("the store of A after B wrote", a.Page(), "A|A")
	check("the store of B", b.Page(), "B|B")

	check("a web view of A made later", open(idA).Page(), "A|A")
	check("a web view with no StorageID", open("").Page(), empty)

	// A window shows a page of a store as a web view does.
	win := newWindow(t, mygo.WindowOptions{Title: "Storage B", Width: 300, Height: 200, URL: server.URL, Page: mygo.PageOptions{StorageID: idB}})
	waitFor(t, win, "document.title === 'storage'")
	check("a window of B", win.Page(), "B|B")

	// A window the page opens shares its store, whatever its options say.
	created := make(chan *mygo.Window, 4)
	defer mygo.App.OnWindowCreated(func(w *mygo.Window) { created <- w })()
	a.Page().SetWindowOpenHandler(func(mygo.WindowOpenRequest) *mygo.WindowOptions {
		return &mygo.WindowOptions{Title: "Popup of A", Width: 300, Height: 200, Page: mygo.PageOptions{StorageID: idB}}
	})
	if _, err := a.Page().Eval("window.open('" + server.URL + "/popup'), 1"); err != nil {
		t.Fatal(err)
	}
	popup := <-created
	t.Cleanup(popup.Destroy)
	waitFor(t, popup, "document.title === 'storage'")
	check("a window A opened", popup.Page(), "A|A")
}
