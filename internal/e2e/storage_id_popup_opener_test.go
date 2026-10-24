//go:build !js

package e2e

import "runtime"

// With the windows a page opens kept in relation to it (window.opener, on
// Windows through the web view the opener's event is given), the popups of
// TestWindowOpenKeepsStorage have an opener on every platform with a
// StorageID.
func init() { popupKeepsOpener = runtime.GOOS != "linux" }
