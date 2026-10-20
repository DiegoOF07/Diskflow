package engine

import (
	"path/filepath"
	"testing"
)

func TestRootKey(t *testing.T) {
	tests := []struct {
		name            string
		a, b            string
		caseInsensitive bool
		same            bool
	}{
		{"trailing separator and dots", "work/./proj/", "work/proj", false, true},
		{"case differs, case-sensitive FS", "Work/Proj", "work/proj", false, false},
		{"case differs, case-insensitive FS", "Work/Proj", "work/proj", true, true},
		{"different dirs", "work/a", "work/b", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ka, err := rootKey(tt.a, tt.caseInsensitive)
			if err != nil {
				t.Fatal(err)
			}
			kb, _ := rootKey(tt.b, tt.caseInsensitive)
			if (ka == kb) != tt.same {
				t.Fatalf("rootKey(%q)=%q, rootKey(%q)=%q, same=%v want %v", tt.a, ka, tt.b, kb, ka == kb, tt.same)
			}
			if !filepath.IsAbs(ka) {
				t.Fatalf("key %q is not absolute", ka)
			}
		})
	}
}
