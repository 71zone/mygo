//go:build !js

package e2e

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
)

// A navigation the app cancels in OnWillNavigate is no failed load, and one
// to a host that does not resolve is.
func TestCancelledNavigationIsNoLoadFailure(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Load failure", Width: 300, Height: 200})
	w.Page().LoadHTML(`<p id="ready">load failure</p>`, "")
	waitFor(t, w, "document.getElementById('ready')")

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

	// Events come in order: a failure the cancelled navigation reported
	// would come before the one of the unresolvable host.
	if _, err := w.Page().Eval("location.href = 'http://blocked.invalid/'"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the cancelled navigation to ask OnWillNavigate", func() bool { return blocked.Load() == 1 })
	if _, err := w.Page().Eval("location.href = 'http://nonexistent.invalid/'"); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the failure of the unresolvable host", func() bool { return len(failures()) > 0 })
	got := failures()
	if len(got) != 1 {
		t.Fatalf("OnDidFailLoad fired %d times (%+v), want once, for the unresolvable host", len(got), got)
	}
	// macOS leaves the URL of a failed load empty.
	if got[0].URL != "" && !strings.Contains(got[0].URL, "nonexistent.invalid") {
		t.Errorf("the failed load is of %q (%v), want the unresolvable host's", got[0].URL, got[0])
	}
}
