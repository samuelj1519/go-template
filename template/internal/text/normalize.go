// Package text implements transport-independent text rules.
package text

import (
	"errors"
	"strings"
)

// ErrEmpty means the input contains no words.
var ErrEmpty = errors.New("input must contain a word")

// Normalize lowercases words and joins Unicode whitespace with a single space.
func Normalize(input string) (string, error) {
	words := strings.Fields(input)
	if len(words) == 0 {
		return "", ErrEmpty
	}
	return strings.ToLower(strings.Join(words, " ")), nil
}
