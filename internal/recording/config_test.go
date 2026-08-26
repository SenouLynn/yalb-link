package recording

import (
	"testing"
	"time"
)

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

func TestResolveRetentionConfig(t *testing.T) {
	if got := ResolveMaxTotalBytes("", false); got != DefaultMaxTotalBytes {
		t.Errorf("unset bytes = %d", got)
	}
	if got := ResolveMaxTotalBytes("1048576", true); got != 1048576 {
		t.Errorf("parsed bytes = %d", got)
	}
	if got := ResolveMaxTotalBytes("invalid", true); got != DefaultMaxTotalBytes {
		t.Errorf("invalid bytes = %d", got)
	}
	if got := ResolveMaxTotalAge("12h", true); got != 12*time.Hour {
		t.Errorf("parsed age = %s", got)
	}
	if got := ResolveMaxTotalAge("invalid", true); got != DefaultMaxTotalAge {
		t.Errorf("invalid age = %s", got)
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
