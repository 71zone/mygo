package mygo

import (
	"errors"
	"fmt"
	"math"

	"github.com/egoist/mygo/internal/platform"
)

// Modifiers are the modifier keys held while an input event happens.
type Modifiers uint8

// The modifier keys; combine them with |.
const (
	ModShift = Modifiers(platform.ModShift)
	ModCtrl  = Modifiers(platform.ModCtrl)
	ModAlt   = Modifiers(platform.ModAlt)
	// ModSuper is Command on macOS and the Windows key on Windows.
	ModSuper = Modifiers(platform.ModSuper)
)

// MouseEventType is what a MouseEvent does.
type MouseEventType uint8

// The things a mouse event does.
const (
	// MouseMove moves the pointer to the point.
	MouseMove MouseEventType = iota
	// MouseDown presses the primary button at the point.
	MouseDown
	// MouseUp releases it.
	MouseUp
)

// MouseEvent is the pointer and its primary button acting on a page.
type MouseEvent struct {
	Type MouseEventType
	// X and Y are in CSS pixels from the top-left corner of the page's
	// viewport, where getBoundingClientRect puts the elements.
	X, Y float64
	// ClickCount is how many presses in a row this one is: 1, the
	// default, for a click, 2 for the second press of a double click, as
	// the user's presses count. MouseMove ignores it.
	ClickCount int
	Modifiers  Modifiers
}

// KeyEventType is what a KeyEvent does.
type KeyEventType uint8

// The things a key event does.
const (
	// KeyPress presses the key and releases it: the page gets keydown,
	// keypress for a key that types a character, and keyup.
	KeyPress KeyEventType = iota
	// KeyDown presses the key and holds it.
	KeyDown
	// KeyUp releases it.
	KeyUp
)

// KeyEvent is a key of the keyboard acting on a page.
type KeyEvent struct {
	Type KeyEventType
	// Key is the key's KeyboardEvent.key value, as of a US keyboard:
	// "Enter", "Tab", "Escape", "Backspace", "Delete", "ArrowLeft",
	// "ArrowRight", "ArrowUp", "ArrowDown", "Home", "End", "PageUp",
	// "PageDown", " ", or a letter or a digit. A capital letter is the
	// letter with Shift held. To type other text, use Page.InsertText.
	Key       string
	Modifiers Modifiers
}

var (
	errMouseType  = errors.New("mygo: SendMouse: unknown event type")
	errMousePoint = errors.New("mygo: SendMouse: the point is not a finite number")
	errKeyType    = errors.New("mygo: SendKey: unknown event type")
)

// SendMouse delivers a mouse event to the page as the system delivers the
// mouse's: the page gets mousedown, mouseup, click and the other events
// that follow, with isTrusted true, which a script's dispatchEvent never
// has. A click is a MouseMove, a MouseDown and a MouseUp at one point; a
// double click is a click and a second one with ClickCount 2:
//
//	for _, t := range []mygo.MouseEventType{mygo.MouseMove, mygo.MouseDown, mygo.MouseUp} {
//		err := page.SendMouse(mygo.MouseEvent{Type: t, X: 220, Y: 172})
//	}
//
// Input needs no key window, no active app and no visible window or web
// view: the page gets the event where the point is. X and Y stay CSS pixels
// at any page zoom. SendMouse returns when the page took the event, not when
// the page handled it. The platforms differ in what they hold back:
//
//   - WebKit queues mouse events while one is in flight and takes keys at
//     once, so wait for what a click did (the focus it gave) before
//     SendKey or InsertText.
//   - On macOS a MouseDown gives the keyboard to the web view, as a click
//     does, and WebKit drops a MouseMove at a point that the window's
//     content paints over the web view.
//   - On Linux the web view must be shown, not in a hidden window: WebKitGTK
//     drops the presses of one that is not, so SendMouse returns an error.
//
// In browsers it returns an error matching errors.ErrUnsupported.
func (p *Page) SendMouse(e MouseEvent) error {
	if p.w.content != nil {
		return errNoPage
	}
	if e.Type > MouseUp {
		return errMouseType
	}
	if math.IsNaN(e.X) || math.IsNaN(e.Y) || math.IsInf(e.X, 0) || math.IsInf(e.Y, 0) {
		return errMousePoint
	}
	if e.ClickCount <= 0 {
		e.ClickCount = 1
	}
	pe := platform.PageMouseEvent{Kind: platform.PageMouseKind(e.Type), X: e.X, Y: e.Y, ClickCount: e.ClickCount, Mods: platform.Modifiers(e.Modifiers)}
	return p.input(func(n platform.Window, done func(error)) { n.SendMouse(pe, done) })
}

// SendKey delivers a key event to the page as the system delivers the
// keyboard's: the page gets keydown, keypress and keyup with isTrusted true,
// at the element that has the focus, and the key does what it does in a
// browser: Enter in a text field starts a line, Tab moves the focus.
// Pressing Enter to send a message:
//
//	err := page.SendKey(mygo.KeyEvent{Key: "Enter"})
//
// Like SendMouse it returns when the page took the event, not when the page
// handled it, needs no key window and, except on Linux, no shown window (see
// SendMouse), and gives an error for a key it does not know. The keys are
// those of a US keyboard, with no input method composing, and a shortcut
// with ModSuper is not reliable: macOS answers it as a key equivalent, not
// as a key event. In browsers it returns an error matching
// errors.ErrUnsupported.
func (p *Page) SendKey(e KeyEvent) error {
	if p.w.content != nil {
		return errNoPage
	}
	if e.Type > KeyUp {
		return errKeyType
	}
	k, shift, ok := platform.PageKeyByName(e.Key)
	if !ok {
		return fmt.Errorf("mygo: SendKey: unknown key %q", e.Key)
	}
	mods := platform.Modifiers(e.Modifiers)
	if shift {
		mods |= platform.ModShift
	}
	pe := platform.PageKeyEvent{Kind: platform.PageKeyKind(e.Type), Key: k, Mods: mods}
	return p.input(func(n platform.Window, done func(error)) { n.SendKey(pe, done) })
}

// InsertText inserts text at the caret of the element that has the focus,
// as an input method does when it commits what it composed: the element's
// beforeinput and input events have the inputType "insertText", with
// isTrusted true, and an editor in the page takes it as typed. The text goes
// in as one edit, and newlines in it are not Enter keys. An element that does
// not take text ignores it. It returns when the page took the text, not when
// the page handled it, and needs no key window or shown window. In browsers
// it returns an error matching errors.ErrUnsupported.
func (p *Page) InsertText(text string) error {
	if p.w.content != nil {
		return errNoPage
	}
	return p.input(func(n platform.Window, done func(error)) { n.InsertText(text, done) })
}

// input runs send on the main thread and waits for its done.
func (p *Page) input(send func(n platform.Window, done func(error))) error {
	ch := make(chan error, 1)
	ran := false
	p.w.do(func(n platform.Window) {
		ran = true
		send(n, func(err error) { deliver(ch, err) })
	})
	if !ran {
		return errDestroyed
	}
	return await(ch)
}
