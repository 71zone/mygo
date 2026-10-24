//go:build windows && (amd64 || arm64)

package windows

import "testing"

// TestNavigationCancelled checks which COREWEBVIEW2_WEB_ERROR_STATUS ends
// a navigation as cancelled, not as a failed load: OPERATION_CANCELED (14),
// which a navigation the app prevented or one another replaced gets, and
// not HOST_NAME_NOT_RESOLVED (13), a failed lookup.
func TestNavigationCancelled(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int32
		want   bool
	}{
		{"OPERATION_CANCELED", webErrorOperationCanceled, true},
		{"HOST_NAME_NOT_RESOLVED", webErrorHostNameNotResolved, false},
		{"UNKNOWN", 0, false},
		{"CONNECTION_ABORTED", 9, false},
	} {
		if got := navigationCancelled(c.status); got != c.want {
			t.Errorf("navigationCancelled(%s = %d) = %v, want %v", c.name, c.status, got, c.want)
		}
	}
}
