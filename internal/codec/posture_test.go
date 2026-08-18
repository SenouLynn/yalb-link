package codec

import (
	"strings"
	"testing"
)

func TestPostureWarning(t *testing.T) {
	for _, tc := range []struct {
		name    string
		bind    string
		key     string
		wantFor string
	}{
		{name: "unauthenticated bind warns", bind: "0.0.0.0:14550", wantFor: "0.0.0.0:14550"},
		{name: "signing key silences it", bind: "0.0.0.0:14550", key: "secret"},
		{name: "disabled socket has nothing to warn about", bind: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PostureWarning(tc.bind, tc.key)

			if tc.wantFor == "" {
				if got != "" {
					t.Errorf("warned when it should not: %q", got)
				}

				return
			}

			// The warning has to name the bind address. "Signing is off" with
			// no address does not tell an operator which box is exposed.
			if !strings.Contains(got, tc.wantFor) {
				t.Errorf("warning %q does not name the bind address %q", got, tc.wantFor)
			}
		})
	}
}

// Unset and explicitly-empty are different instructions. Treating them alike
// is how a bridge ends up silently deaf on a box nobody configured.
func TestResolveBind(t *testing.T) {
	if got := ResolveBind("", false); got != DefaultUDPBind {
		t.Errorf("unset resolved to %q, want %q", got, DefaultUDPBind)
	}

	if got := ResolveBind("", true); got != "" {
		t.Errorf("explicit empty resolved to %q, want the socket disabled", got)
	}

	if got := ResolveBind("127.0.0.1:14555", true); got != "127.0.0.1:14555" {
		t.Errorf("explicit address resolved to %q", got)
	}
}
