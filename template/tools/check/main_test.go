package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoverage(t *testing.T) {
	const file = "example.com/project/internal/text/normalize.go"
	for _, tc := range []struct {
		name, report string
		files        []string
		valid        bool
	}{
		{"complete", "mode: atomic\n" + file + ":1.1,2.2 2 1\n", []string{file}, true},
		{"empty", "mode: atomic\n", []string{file}, false},
		{"uncovered", "mode: atomic\n" + file + ":1.1,2.2 2 0\n", []string{file}, false},
		{"foreign", "mode: atomic\nother.go:1.1,2.2 2 1\n", []string{file}, false},
		{"missing", "mode: atomic\n" + file + ":1.1,2.2 2 1\n", []string{file, "missing.go"}, false},
		{"no-files", "mode: atomic\n" + file + ":1.1,2.2 2 1\n", nil, false},
		{"invalid-mode", "mode: set\n" + file + ":1.1,2.2 2 1\n", []string{file}, false},
		{"invalid-count", "mode: atomic\n" + file + ":1.1,2.2 -1 1\n", []string{file}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := coverage([]byte(tc.report), tc.files); (err == nil) != tc.valid {
				t.Fatalf("coverage valid=%v, want %v (error=%v)", err == nil, tc.valid, err)
			}
		})
	}
}

func TestDependencyPolicy(t *testing.T) {
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"Main":true,"Path":"example.com/app"}`, true},
		{"", false},
		{`{"Main":true,"Path":"a"} {"Main":false,"Path":"external"}`, false},
		{`{"Main":true,"Replace":{"Path":"../outside"}}`, false},
		{`not json`, false},
	} {
		if err := dependencies([]byte(tc.data)); (err == nil) != tc.valid {
			t.Fatalf("dependencies(%q): %v", tc.data, err)
		}
	}
}

func TestCheckArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"quick", "full"}} {
		if err := check(args); err == nil {
			t.Fatalf("accepted invalid arguments: %v", args)
		}
	}
}

func TestFormattingIncludesBuildVariants(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"cgo", "package example\n\nimport \"C\"\n\nfunc   Cgo() {}\n"},
		{"ignored", "//go:build freebsd\n\npackage example\n\nfunc   Ignored() {}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("GOWORK", "off")
			writeCheckFile(t, root, "go.mod", "module example.com/format\n\ngo 1.27.4\n")
			writeCheckFile(t, root, "active.go", "package example\n")
			writeCheckFile(t, root, tc.name+".go", tc.source)
			err := formatting()
			if err == nil || !strings.Contains(err.Error(), "needs gofmt") {
				t.Fatalf("formatting() error = %v, want gofmt diagnostic", err)
			}
		})
	}
}

func TestApplicationCoverageIncludesCgo(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("GOWORK", "off")
	t.Setenv("CGO_ENABLED", "1")
	writeCheckFile(t, root, "go.mod", "module example.com/cgo\n\ngo 1.27.4\n")
	writeCheckFile(t, root, "internal/example/cgo.go", "package example\n\nimport \"C\"\n\nfunc Answer() int { return 42 }\n")
	writeCheckFile(t, root, "internal/example/cgo_test.go", "package example\n\nimport \"testing\"\n\nfunc TestAnswer(t *testing.T) {\n\tif Answer() != 42 {\n\t\tt.Fatal(\"wrong answer\")\n\t}\n}\n")
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := applicationCoverage(); err != nil {
		t.Fatal(err)
	}
}

func writeCheckFile(t *testing.T, root, name, data string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestMutationReport(t *testing.T) {
	for _, tc := range []struct {
		data  string
		valid bool
	}{
		{`{"mutants_total":1,"mutants_killed":1,"files":[{"mutations":[{"status":"KILLED"}]}]}`, true},
		{`{}`, false},
		{`{"mutants_total":0,"files":[{"mutations":[{"status":"TIMED OUT"}]}]}`, false},
		{`{"mutants_total":1,"mutants_killed":1,"files":[{"mutations":[{"status":"NOT VIABLE"}]}]}`, false},
		{`{"mutants_total":1,"mutants_killed":0,"files":[{"mutations":[{"status":"LIVED"}]}]}`, false},
		{`{"mutants_total":1,"mutants_killed":1}`, false},
	} {
		if err := mutationReport([]byte(tc.data)); (err == nil) != tc.valid {
			t.Fatalf("mutationReport(%s) = %v", tc.data, err)
		}
	}
}

func TestMilestoneCopyIsolation(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for name, data := range map[string]string{
		"go.mod":                              "module example.com/fixture\n\ngo 1.27.4\n",
		"internal/example/rule.go":            "package example\n\nimport _ \"embed\"\n\n//go:embed message.txt\nvar message string\n",
		"internal/example/message.txt":        "synthetic embed",
		"internal/example/testdata/input.txt": "synthetic fixture",
		".env":                                "FAKE_TOKEN=synthetic",
		"bin/app":                             "unrelated runtime artifact",
		"notes.txt":                           "unrelated private notes",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(t.TempDir(), "source")
	if err := copySource(dst); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"go.mod", "internal/example/rule.go", "internal/example/message.txt", "internal/example/testdata/input.txt"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(name))); err != nil {
			t.Fatalf("required input missing %s: %v", name, err)
		}
	}
	for _, name := range []string{".env", "bin/app", "notes.txt"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("copied non-input %s: %v", name, err)
		}
	}
}

func TestCopyRejectsEnvironmentFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	if err := os.WriteFile(path, []byte("FAKE_TOKEN=synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(path, filepath.Join(root, "copied")); err == nil {
		t.Fatal("accepted environment file as a fixture")
	}
}
