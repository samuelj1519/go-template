package text_test

import (
	"errors"
	"testing"

	"github.com/samuelj1519/go-template/internal/text"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"words", "  Hello\tWORLD  ", "hello world"},
		{"unicode", "\u2003CAFÉ\nGo\u00a0", "café go"},
		{"single", "Go", "go"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := text.Normalize(tc.input)
			if err != nil || got != tc.want {
				t.Fatalf("Normalize(%q) = %q, %v; want %q, nil", tc.input, got, err, tc.want)
			}
		})
	}
}

func TestNormalizeEmpty(t *testing.T) {
	for _, input := range []string{"", " \t\n\u2003"} {
		got, err := text.Normalize(input)
		if got != "" || !errors.Is(err, text.ErrEmpty) {
			t.Fatalf("Normalize(%q) = %q, %v; want empty and ErrEmpty", input, got, err)
		}
	}
}

func FuzzNormalize(f *testing.F) {
	f.Add("  Hello\tWORLD ")
	f.Add("\u2003")
	f.Fuzz(func(t *testing.T, input string) {
		got, err := text.Normalize(input)
		if err != nil {
			if !errors.Is(err, text.ErrEmpty) || got != "" {
				t.Fatalf("invalid error result: %q, %v", got, err)
			}
			return
		}
		again, err := text.Normalize(got)
		if err != nil || got != again {
			t.Fatalf("normalization is not idempotent: %q -> %q (%v)", got, again, err)
		}
	})
}
