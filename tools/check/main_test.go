package main

import (
	"os"
	"path/filepath"
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
