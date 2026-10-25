//go:build windows && (amd64 || arm64)

package windows

import "testing"

// TestNavigationCancelled checks which ends of a navigation are no failed
// load: OPERATION_CANCELED (14), which a navigation the app prevented gets,
// and a load Stop ended; not HOST_NAME_NOT_RESOLVED (13), a failed lookup.
func TestNavigationCancelled(t *testing.T) {
	for _, c := range []struct {
		name    string
		status  int32
		stopped bool
		want    bool
	}{
		{"OPERATION_CANCELED", webErrorOperationCanceled, false, true},
		{"HOST_NAME_NOT_RESOLVED", webErrorHostNameNotResolved, false, false},
		{"UNKNOWN", 0, false, false},
		{"CONNECTION_ABORTED", webErrorConnectionAborted, false, false},
		{"CONNECTION_ABORTED by Stop", webErrorConnectionAborted, true, true},
	} {
		if got := navigationCancelled(c.status, c.stopped); got != c.want {
			t.Errorf("navigationCancelled(%s = %d, stopped %v) = %v, want %v", c.name, c.status, c.stopped, got, c.want)
		}
	}
}
