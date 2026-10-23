//go:build !js

package mygo

import (
	"math"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestPageInput(t *testing.T) {
	w := NewWindow(WindowOptions{Hidden: true, URL: "https://example.com/"})
	t.Cleanup(w.Destroy)
	fw := fb.Windows()[len(fb.Windows())-1]
	p := w.Page()

	// Enter, and a capital letter, which is its letter with Shift held.
	for _, e := range []KeyEvent{{Key: "Enter"}, {Type: KeyDown, Key: "A", Modifiers: ModCtrl}, {Type: KeyUp, Key: "7"}} {
		if err := p.SendKey(e); err != nil {
			t.Fatalf("SendKey(%+v): %v", e, err)
		}
	}
	if len(fw.Keys) != 3 {
		t.Fatalf("keys %+v", fw.Keys)
	}
	if k := fw.Keys[0]; k.Kind != platform.PageKeyPress || k.Key.Name != "Enter" || k.Key.Mac != 36 || k.Key.Win != 0x0D || k.Key.Text != "\r" {
		t.Errorf("Enter reached the backend as %+v", k)
	}
	if k := fw.Keys[1]; k.Kind != platform.PageKeyDown || k.Key.Name != "a" || k.Key.Code != "KeyA" || k.Key.Win != 'A' || k.Mods != platform.ModShift|platform.ModCtrl {
		t.Errorf("Ctrl+A reached the backend as %+v", k)
	}
	if k := fw.Keys[2]; k.Kind != platform.PageKeyUp || k.Key.Name != "7" || k.Key.Code != "Digit7" || k.Key.Mac != 26 {
		t.Errorf("7 reached the backend as %+v", k)
	}

	// A click defaults to a single click; a move has no click count to give.
	for _, typ := range []MouseEventType{MouseMove, MouseDown, MouseUp} {
		if err := p.SendMouse(MouseEvent{Type: typ, X: 12.5, Y: 40}); err != nil {
			t.Fatalf("SendMouse: %v", err)
		}
	}
	if len(fw.Mouse) != 3 || fw.Mouse[1] != (platform.PageMouseEvent{Kind: platform.PageMouseDown, X: 12.5, Y: 40, ClickCount: 1}) {
		t.Errorf("mouse %+v", fw.Mouse)
	}

	if err := p.InsertText("xin chào"); err != nil || !slices.Equal(fw.Inserted, []string{"xin chào"}) {
		t.Errorf("InsertText: %v, inserted %q", err, fw.Inserted)
	}

	// Input the page cannot take is an error, not a no-op.
	if err := p.SendKey(KeyEvent{Key: "F13"}); err == nil {
		t.Error("SendKey of an unknown key succeeded")
	}
	if err := p.SendKey(KeyEvent{Type: 9, Key: "a"}); err == nil {
		t.Error("SendKey of an unknown type succeeded")
	}
	if err := p.SendMouse(MouseEvent{Type: MouseDown, X: math.NaN()}); err == nil {
		t.Error("SendMouse at NaN succeeded")
	}
	if len(fw.Keys) != 3 || len(fw.Mouse) != 3 {
		t.Errorf("refused events reached the backend: keys %d, mouse %d", len(fw.Keys), len(fw.Mouse))
	}

	w.Destroy()
	if err := p.SendKey(KeyEvent{Key: "Enter"}); err == nil {
		t.Error("SendKey to a destroyed window succeeded")
	}
}
