//go:build darwin

package darwin

import (
	"errors"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// The UUIDs are the data stores' names on disk: they must never change, or
// the apps that kept data under a StorageID lose it. The values are what
// uuid.uuid5 of Python gives for the namespace 9f1c5a7e-3b0d-4c52-8a61-6d2e40b7f3a9.
func TestStorageUUID(t *testing.T) {
	for id, want := range map[string]string{
		"acct-a":                "d5d43e82-a33c-55fa-99a4-9fdecd191d2b",
		"acct-b":                "12dc7b02-7ab6-59fc-8f34-25742f6cb51f",
		"Acct-A":                "7fde0f76-9a1d-5787-924b-0217333e404d",
		strings.Repeat("x", 64): "abcc2270-37b5-5bc1-824e-0b232c8855e7",
	} {
		if got := storageUUID(id); got != want {
			t.Errorf("storageUUID(%q) = %s, want %s", id, got, want)
		}
	}
}

// A StorageID gets the same data store each time and another ID another,
// where this macOS has them.
func TestDataStore(t *testing.T) {
	load()
	err := checkStorage("acct-a")
	if errors.Is(err, platform.ErrUnsupported) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := checkStorage(""); err != nil {
		t.Errorf("the default store: %v", err)
	}
	withPool(func() {
		a, b := dataStore("acct-a"), dataStore("acct-b")
		if a == 0 || b == 0 || a == b {
			t.Errorf("data stores of acct-a %v and acct-b %v", a, b)
		}
		if again := dataStore("acct-a"); again != a {
			t.Errorf("acct-a gave %v, then %v", a, again)
		}
		if !sendBool(a, "isPersistent") || sendBool(a, "isPersistent") != sendBool(b, "isPersistent") {
			t.Error("a data store of a StorageID is not persistent")
		}
	})
}
