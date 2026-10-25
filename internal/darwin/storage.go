//go:build darwin

package darwin

import (
	"crypto/sha1"
	"fmt"

	"github.com/egoist/mygo/internal/platform"
)

// A page with a StorageID (platform.WindowOptions) keeps its website data
// in the WKWebsiteDataStore of an identifier (macOS 14), which WebKit
// writes under ~/Library/WebKit/<bundle id>/WebsiteDataStore/<identifier>
// and finds again at the next launch. The identifier is an NSUUID, so the
// StorageID becomes one, the same each time: a version 5 UUID of it, in a
// namespace of MyGo's own.

// storageNamespace is the namespace of the UUIDs of StorageIDs,
// 9f1c5a7e-3b0d-4c52-8a61-6d2e40b7f3a9. Changing it would lose the data
// stores of every app.
var storageNamespace = [16]byte{0x9f, 0x1c, 0x5a, 0x7e, 0x3b, 0x0d, 0x4c, 0x52, 0x8a, 0x61, 0x6d, 0x2e, 0x40, 0xb7, 0xf3, 0xa9}

// storageUUID returns the UUID, in its text form, of the data store of a
// StorageID.
func storageUUID(storageID string) string {
	h := sha1.New()
	h.Write(storageNamespace[:])
	h.Write([]byte(storageID))
	u := h.Sum(nil)[:16]
	u[6] = u[6]&0x0f | 0x50 // version 5
	u[8] = u[8]&0x3f | 0x80 // the variant of RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
}

// checkStorage fails with ErrUnsupported when this macOS cannot keep the
// data of a StorageID apart: data stores of an identifier came with macOS
// 14.
func checkStorage(storageID string) error {
	if storageID == "" || respondsTo(class("WKWebsiteDataStore"), "dataStoreForIdentifier:") {
		return nil
	}
	return fmt.Errorf("mygo: PageOptions.StorageID needs macOS 14 or newer: %w", platform.ErrUnsupported)
}

// dataStore returns the data store of a StorageID, autoreleased.
// checkStorage said it exists.
func dataStore(storageID string) id {
	uuid := send(send(class("NSUUID"), "alloc"), "initWithUUIDString:", uintptr(nsString(storageUUID(storageID))))
	defer release(uuid)
	return send(class("WKWebsiteDataStore"), "dataStoreForIdentifier:", uintptr(uuid))
}
