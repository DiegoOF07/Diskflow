package model

import "testing"

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindFile, "file"},
		{KindDir, "dir"},
		{KindLink, "link"},
		{KindOther, "other"},
		{Kind(99), "other"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
