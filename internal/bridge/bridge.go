// Package bridge embeds the renderer runtime that MyGo injects into pages.
//
// bridge.js is built from ../../packages/bridge by `bun run build` and
// committed, so building a MyGo application never requires Bun.
package bridge

import (
	_ "embed"
	"encoding/json"

	"github.com/egoist/mygo/internal/platform"
)

//go:embed bridge.js
var source string

// Config is passed to the bridge as __MYGO_CONFIG__.
type Config struct {
	Platform string `json:"platform"`
	WindowID int    `json:"windowId"`
	Version  string `json:"version"`
	// Secret prefixes every message the bridge posts.
	Secret string `json:"secret"`
	// Locale is the language the app shows, e.g. "zh-Hans", and Dir the
	// direction it is written in, "ltr" or "rtl", which pages read as
	// mygo.locale and mygo.dir from document start.
	Locale string `json:"locale"`
	Dir    string `json:"dir"`
	// Trust says which pages get the bridge: the others find none of it
	// (neither window.mygo nor the channel to Go).
	Trust Trust `json:"trust"`
	// Find installs only what Page.FindInPage needs, in a page that has no
	// bridge, and takes it away with StopFindInPage.
	Find bool `json:"find,omitempty"`
}

// Trust is the rule of the core's isTrusted, as the bridge applies it to
// its page at document start. The slices are never nil, for the bridge
// reads them as arrays.
type Trust struct {
	// Schemes are the schemes whose pages are trusted, whatever the host.
	Schemes []string `json:"schemes"`
	// Origins are the lower case origins, scheme://host[:port], of trusted
	// pages; "*" trusts every page.
	Origins []string `json:"origins"`
	// Loopback trusts the pages of http and https on localhost, 127.0.0.1
	// and ::1.
	Loopback bool `json:"loopback"`
}

// Script returns the bridge source configured for one window.
func Script(cfg Config) string {
	b, _ := json.Marshal(cfg)
	return "(function(__MYGO_CONFIG__){\n" + source + "})(" + string(b) + ");"
}

// HideScript hides the channel to Go from the pages in the frames of a
// window: Script does it in the top frame, where the bridge takes the
// channel, and this script in the frames inside it, which have no bridge but
// find the channel as well. It is for every frame, and only does anything
// in those that are not the top one. The channel of WebKit is the message
// handler, which `webkit.messageHandlers` reaches through the prototype of
// `webkit`; that of WebView2 is `chrome.webview`.
const HideScript = `if(window!==window.top){try{delete Object.getPrototypeOf(window.webkit).messageHandlers}catch(e){}try{delete window.chrome.webview}catch(e){}}`

// TitleBarEvent tells a page of a window with a hidden title bar the room
// the window controls take. The bridge keeps it in the --mygo-titlebar-*
// CSS variables.
const TitleBarEvent = "mygo:title-bar"

// LocaleEvent tells the pages of a window that the language the app shows
// changed, with a Locale. The bridge configuration tells pages the
// language at document start; the event is sent as a page loads too, for
// one that started loading as the language changed, and the bridge tells
// its listeners only of a language the page did not have.
const LocaleEvent = "mygo:locale"

// Locale is the payload of LocaleEvent.
type Locale struct {
	Locale string `json:"locale"`
	Dir    string `json:"dir"`
}

// TitleBar is the payload of TitleBarEvent, in CSS pixels.
type TitleBar struct {
	Height float64 `json:"height"`
	Left   float64 `json:"left"`
	Right  float64 `json:"right"`
}

// NewTitleBar converts the room of the window controls to the CSS pixels
// of a page shown at zoom.
func NewTitleBar(tb platform.TitleBar, zoom float64) TitleBar {
	if zoom <= 0 {
		zoom = 1
	}
	return TitleBar{Height: float64(tb.Height) / zoom, Left: float64(tb.Left) / zoom, Right: float64(tb.Right) / zoom}
}

// TitleBarScript delivers TitleBarEvent at document start, after the
// bridge, so that a page lays out around the window controls before it
// first paints.
func TitleBarScript(tb platform.TitleBar, zoom float64) string {
	p, _ := json.Marshal(NewTitleBar(tb, zoom))
	return `window.__mygo&&window.__mygo.receive({"t":"event","n":"` + TitleBarEvent + `","p":` + string(p) + `});`
}

// PreferencesEvent tells a page the accent color and the accessibility
// settings of the desktop, as mygo.ThemePreferences encodes them. The
// bridge keeps the latest for mygo-runtime's preferences(), the accent in
// the --mygo-accent CSS variable and the text scale in --mygo-text-scale.
const PreferencesEvent = "mygo:preferences"

// PreferencesScript delivers PreferencesEvent with the JSON prefs at
// document start, after the bridge has registered its handler.
func PreferencesScript(prefs []byte) string {
	return `window.__mygo&&window.__mygo.receive({"t":"event","n":"` + PreferencesEvent + `","p":` + string(prefs) + `});`
}
