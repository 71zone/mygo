//go:build !js

package e2e

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo"
)

// Page.SendMouse, SendKey and InsertText reach the page as the system's
// input: every event it hears has isTrusted true, which a script's
// dispatchEvent never gives, and a page that rejects the others takes them.
func TestPageInput(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("WebKitGTK has no way to send input to a page")
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Page input", Width: 480, Height: 320})
	w.Page().LoadHTML(`<body style="margin:0">
<textarea id="t" style="position:absolute;left:20px;top:20px;width:300px;height:80px"></textarea>
<script>
window.log = [];
const trust = (e) => window.log.push(e.type + ':' + e.isTrusted);
for (const type of ['mousedown', 'mouseup', 'click', 'keydown', 'keyup', 'beforeinput', 'input']) {
  document.addEventListener(type, trust, true);
}
document.getElementById('t').addEventListener('keydown', (e) => {
  if (e.key === 'Enter' && e.isTrusted) { e.preventDefault(); window.sent = e.target.value; }
});
</script></body>`, "")
	waitFor(t, w, "window.log && document.getElementById('t')")
	p := w.Page()

	for _, typ := range []mygo.MouseEventType{mygo.MouseMove, mygo.MouseDown, mygo.MouseUp} {
		if err := p.SendMouse(mygo.MouseEvent{Type: typ, X: 100, Y: 50}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, w, "document.activeElement === document.getElementById('t')")
	if err := p.InsertText("xin chào"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "document.getElementById('t').value === 'xin chào'")
	if err := p.SendKey(mygo.KeyEvent{Key: "Enter"}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.sent === 'xin chào'")

	got, err := mygo.EvalAs[[]string](p, "window.log")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range got {
		if line[len(line)-5:] != ":true" {
			t.Errorf("an event of the page's was not trusted: %v", got)
			break
		}
	}
	for _, want := range []string{"mousedown:true", "mouseup:true", "click:true", "beforeinput:true", "input:true", "keydown:true"} {
		found := false
		for _, line := range got {
			found = found || line == want
		}
		if !found {
			t.Errorf("the page never heard %s: %v", want, got)
		}
	}
}
