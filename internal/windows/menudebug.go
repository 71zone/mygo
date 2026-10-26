//go:build windows && (amd64 || arm64)

package windows

import (
	"log"
	"os"
	"unsafe"
)

// Diagnostics of the popup items' widths (menudraw.go), for a label that
// shows clipped. With MYGO_MENU_DEBUG=1 every item logs how it was measured
// and how it was drawn, to stderr; any other value (not 0) is the path of a
// file to append to, for an app built with -H=windowsgui, which has no
// stderr. To remove them, delete this file and the two calls of menuDebug
// in menudraw.go.
//
// The menu's items all have the width of the one with the longest label, so
// that item alone shows what Windows added to the width it was asked for:
// the smallest windowsAdded of a menu's lines.
var (
	menuDebug = newMenuDebug(os.Getenv("MYGO_MENU_DEBUG"))

	procGetTextMetricsW       = gdi32.NewProc("GetTextMetricsW")
	procGetTextExtentPoint32W = gdi32.NewProc("GetTextExtentPoint32W")

	// Made with the first line logged.
	menuDebugMeasured map[*menuLook]menuDebugItem
	menuDebugSeen     map[menuDebugKey]bool
)

// menuDebugItem is what measureItem answered for an item.
type menuDebugItem struct{ text, width int32 }

// menuDebugKey is a state of an item worth a line: a menu is drawn again at
// every move of the mouse over it.
type menuDebugKey struct {
	l     *menuLook
	width int32
	dpi   int
}

// textMetrics is TEXTMETRICW.
type textMetrics struct {
	Height, Ascent, Descent, InternalLeading, ExternalLeading int32
	AveCharWidth, MaxCharWidth, Weight, Overhang              int32
	DigitizedAspectX, DigitizedAspectY                        int32
	FirstChar, LastChar, DefaultChar, BreakChar               uint16
	Italic, Underlined, StruckOut, PitchAndFamily, CharSet    uint8
}

func newMenuDebug(v string) *log.Logger {
	switch v {
	case "", "0":
		return nil
	case "1":
		return log.New(os.Stderr, "mygo: menu: ", log.Lmicroseconds)
	}
	f, err := os.OpenFile(v, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("mygo: MYGO_MENU_DEBUG: %v", err)
		return nil
	}
	return log.New(f, "menu: ", log.Lmicroseconds)
}

// menuDebugMeasure logs what measureItem measured on the screen's DC dc: the
// width of the label and shortcut in the face f at dpi, and what it answered.
func menuDebugMeasure(l *menuLook, dc uintptr, f menuFonts, dpi int, text, accel, check int32, width uint32) {
	if menuDebugMeasured == nil {
		menuDebugMeasured = map[*menuLook]menuDebugItem{}
	}
	menuDebugMeasured[l] = menuDebugItem{text, int32(width)}
	var lf logFont
	procGetObjectW.Call(f.text, unsafe.Sizeof(lf), uintptr(unsafe.Pointer(&lf)))
	var tm textMetrics
	old, _, _ := procSelectObject.Call(dc, f.text)
	procGetTextMetricsW.Call(dc, uintptr(unsafe.Pointer(&tm)))
	procSelectObject.Call(dc, old)
	menuDebug.Printf("measure %q dpi=%d lfHeight=%d tmHeight=%d tmAveChar=%d variable=%t text=%d accel=%d column=%t submenu=%t check=%d itemWidth=%d",
		l.label, dpi, lf.Height, tm.Height, tm.AveCharWidth, fontFace(f.text, "Segoe UI Variable Text"), text, accel, l.column, l.submenu, check, width)
}

// menuDebugDraw logs how drawItem draws the label: dc is the bitmap's DC the
// label is drawn on, where its width is measured again, and x and right the
// label's rectangle (in the bitmap's coordinates) before the fix's edge.
// Windows' own DC, d.DC, has its menu font selected: the average width of
// its characters tells what Windows adds to an owner-drawn item's width,
// twice that in Wine's tests of Windows and its menus.
func menuDebugDraw(l *menuLook, d *drawItemStruct, dc uintptr, f menuFonts, dpi int, x, right, edge int32) {
	width := d.Item.Right - d.Item.Left
	if menuDebugSeen == nil {
		menuDebugSeen = map[menuDebugKey]bool{}
	}
	k := menuDebugKey{l, width, dpi}
	if menuDebugSeen[k] {
		return
	}
	menuDebugSeen[k] = true
	var size struct{ cx, cy int32 }
	abc := utf16z("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	procGetTextExtentPoint32W.Call(d.DC, uintptr(unsafe.Pointer(&abc[0])), 52, uintptr(unsafe.Pointer(&size)))
	check, _, _ := procGetSystemMetricsForDpi.Call(smCxMenuCheck, uintptr(dpi))
	m := menuDebugMeasured[l]
	drawn := textWidth(dc, f.text, l.label)
	menuDebug.Printf("draw %q dpi=%d rectW=%d itemWidth=%d windowsAdded=%d check=%d 2*avgChar=%d textMeasured=%d textDrawn=%d room=%d short=%d roomNow=%d",
		l.label, dpi, width, m.width, width-m.width, check, 2*((size.cx+26)/52), m.text, drawn, right-x, max(drawn-(right-x), 0), edge-x)
}
