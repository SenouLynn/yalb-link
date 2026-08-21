package recording

import "testing"

func TestResolveEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		set   bool
		want  bool
	}{
		{name: "unset", value: "true", set: false, want: false},
		{name: "true", value: "true", set: true, want: true},
		{name: "case insensitive", value: "TRUE", set: true, want: true},
		{name: "empty", value: "", set: true, want: false},
		{name: "other", value: "1", set: true, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveEnabled(test.value, test.set); got != test.want {
				t.Errorf("ResolveEnabled(%q, %v) = %v, want %v", test.value, test.set, got, test.want)
			}
		})
	}
}

func TestResolveDBPath(t *testing.T) {
	if got := ResolveDBPath("", false); got != DefaultDBPath {
		t.Errorf("unset path = %q", got)
	}
	if got := ResolveDBPath("", true); got != DefaultDBPath {
		t.Errorf("empty path = %q", got)
	}
	if got := ResolveDBPath("custom.db", true); got != "custom.db" {
		t.Errorf("custom path = %q", got)
	}
}
