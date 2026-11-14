package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/samuelj1519/go-template/internal/text"
)

func main() {
	result, err := text.Normalize(strings.Join(os.Args[1:], " "))
	if err == nil {
		_, err = fmt.Fprintln(os.Stdout, result)
	}
	if err != nil {
		// Diagnostics are best effort after the operation has already failed.
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
