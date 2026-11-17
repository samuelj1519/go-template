// Command check runs the project's quality gates from the module root.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	linter  = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
	vuln    = "golang.org/x/vuln/cmd/govulncheck@v1.9.0"
	mutator = "github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0"
)

func main() {
	if err := check(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(dir string, args ...string) error {
	fmt.Printf("+ go %s\n", strings.Join(args, " "))
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, os.Stdout, os.Stderr
	return cmd.Run()
}

func output(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir, cmd.Stderr = dir, os.Stderr
	return cmd.Output()
}

func check(args []string) error {
	mode, err := checkMode(args)
	if err != nil {
		return err
	}
	if err := os.Setenv("GOWORK", "off"); err != nil {
		return err
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from the module root: %w", err)
	}
	if mode == "prepare" {
		return prepare()
	}
	if err := quick(); err != nil {
		return err
	}
	if mode == "quick" {
		return nil
	}
	if err := full(); err != nil {
		return err
	}
	if mode == "milestone" {
		return milestone()
	}
	return nil
}

func checkMode(args []string) (string, error) {
	if len(args) != 1 {
		return "", errors.New("usage: go run ./tools/check prepare|quick|full|milestone")
	}
	switch args[0] {
	case "prepare", "quick", "full", "milestone":
		return args[0], nil
	default:
		return "", fmt.Errorf("unknown check %q; use prepare, quick, full, or milestone", args[0])
	}
}

func prepare() error {
	for _, tool := range []string{linter, vuln, mutator} {
		if err := run(".", "run", tool, "--help"); err != nil {
			return err
		}
	}
	return nil
}

func quick() error {
	if err := formatting(); err != nil {
		return err
	}
	for _, cmd := range [][]string{
		{"vet", "./..."}, {"run", linter, "run"}, {"test", "-count=1", "./..."},
	} {
		if err := run(".", cmd...); err != nil {
			return err
		}
	}
	return nil
}

func formatting() error {
	data, err := output(".", "list", "-json", "./...")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var pkg struct {
			Dir                                                          string
			GoFiles, CgoFiles, IgnoredGoFiles, TestGoFiles, XTestGoFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			return err
		}
		files := append(pkg.GoFiles, pkg.CgoFiles...)
		files = append(files, pkg.IgnoredGoFiles...)
		files = append(files, pkg.TestGoFiles...)
		files = append(files, pkg.XTestGoFiles...)
		for _, name := range files {
			path := filepath.Join(pkg.Dir, name)
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			formatted, err := format.Source(source)
			if err != nil {
				return err
			}
			if !bytes.Equal(source, formatted) {
				return fmt.Errorf("%s needs gofmt; run 'go fmt ./...'", path)
			}
		}
	}
	return nil
}

func full() error {
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		return err
	}
	if err := run(".", "test", "-race", "-count=1", "./..."); err != nil {
		return err
	}
	if err := applicationCoverage(); err != nil {
		return err
	}
	if err := smoke(); err != nil {
		return err
	}
	if err := run(".", "mod", "verify"); err != nil {
		return err
	}
	modules, err := output(".", "list", "-m", "-json", "all")
	if err != nil {
		return err
	}
	if err := dependencies(modules); err != nil {
		return err
	}
	return run(".", "run", vuln, "./...")
}

func applicationCoverage() error {
	if err := run(".", "test", "-covermode=atomic", "-coverprofile=artifacts/coverage.out", "./internal/..."); err != nil {
		return err
	}
	profile, err := os.ReadFile("artifacts/coverage.out")
	if err != nil {
		return err
	}
	data, err := output(".", "list", "-json", "./internal/...")
	if err != nil {
		return err
	}
	var expected []string
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var pkg struct {
			ImportPath        string
			GoFiles, CgoFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			return err
		}
		for _, name := range append(pkg.GoFiles, pkg.CgoFiles...) {
			expected = append(expected, pkg.ImportPath+"/"+name)
		}
	}
	return coverage(profile, expected)
}

func dependencies(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	count := 0
	for decoder.More() {
		var module struct {
			Main    bool
			Path    string
			Replace json.RawMessage
		}
		if err := decoder.Decode(&module); err != nil {
			return err
		}
		if !module.Main || len(module.Replace) != 0 {
			return fmt.Errorf("unreviewed dependency %s: review source and license policy", module.Path)
		}
		count++
	}
	if count != 1 {
		return errors.New("dependency inventory must contain exactly the main module")
	}
	return nil
}

func coverage(profile []byte, expected []string) error {
	lines := strings.Split(strings.TrimSpace(string(profile)), "\n")
	if len(lines) < 2 || lines[0] != "mode: atomic" || len(expected) == 0 {
		return errors.New("empty or invalid application coverage")
	}
	files := make(map[string]bool)
	for _, file := range expected {
		files[file] = false
	}
	statements := 0
	for _, line := range lines[1:] {
		file, count, err := coverageRow(line)
		if err != nil {
			return err
		}
		if _, ok := files[file]; !ok {
			return fmt.Errorf("unexpected coverage source %s", file)
		}
		statements += count
		files[file] = true
	}
	return coverageComplete(files, statements)
}

func coverageComplete(files map[string]bool, statements int) error {
	for name, seen := range files {
		if !seen {
			return fmt.Errorf("missing coverage source %s", name)
		}
	}
	if statements == 0 {
		return errors.New("no covered statements")
	}
	fmt.Printf("Application coverage: %d statements, 100%%\n", statements)
	return nil
}

func coverageRow(line string) (string, int, error) {
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid coverage row: %q", line)
	}
	file, _, ok := strings.Cut(parts[0], ":")
	if !ok {
		return "", 0, errors.New("missing coverage source")
	}
	count, err := strconv.Atoi(parts[1])
	if err != nil || count <= 0 {
		return "", 0, errors.New("invalid statement count")
	}
	hits, err := strconv.Atoi(parts[2])
	if err != nil || hits <= 0 {
		return "", 0, fmt.Errorf("coverage below 100%% at %s", parts[0])
	}
	return file, count, nil
}

func smoke() (result error) {
	dir, err := os.MkdirTemp("", "go-cli-smoke-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(dir)) }()
	name := "app"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(dir, name)
	if err := run(".", "build", "-o", binary, "./cmd/app"); err != nil {
		return err
	}
	for _, tc := range []struct {
		arg, out, stderr string
		code             int
	}{
		{"  Hello\tWORLD ", "hello world\n", "", 0},
		{"\u2003", "", "input must contain a word\n", 1},
	} {
		cmd := exec.Command(binary, tc.arg)
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				return err
			}
			code = exit.ExitCode()
		}
		if stdout.String() != tc.out || stderr.String() != tc.stderr || code != tc.code {
			return fmt.Errorf("CLI mismatch: stdout=%q stderr=%q exit=%d", &stdout, &stderr, code)
		}
	}
	return smokeWriteFailure(binary, dir)
}

func smokeWriteFailure(binary, dir string) (result error) {
	path := filepath.Join(dir, "read-only-output")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	cmd := exec.Command(binary, "Hello")
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = file, &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || stderr.Len() == 0 {
		return fmt.Errorf("CLI output failure must report an error and exit 1: stderr=%q error=%s", &stderr, fmt.Sprint(err))
	}
	return nil
}

func milestone() (result error) {
	base, err := os.MkdirTemp("", "go-quality-")
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.RemoveAll(base)) }()
	source := filepath.Join(base, "source")
	if err := copySource(source); err != nil {
		return err
	}
	if err := os.Setenv("GOCACHE", filepath.Join(base, "build")); err != nil {
		return err
	}
	if err := run(source, "test", "./internal/text", "-run=^$", "-fuzz=FuzzNormalize", "-fuzztime=5s"); err != nil {
		return err
	}
	report := filepath.Join(base, "mutations.json")
	if err := run(source, "run", mutator, "unleash", "--timeout-coefficient=100", "--output", report, "--threshold-efficacy=100", "--threshold-mcover=100", "./internal"); err != nil {
		return err
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return err
	}
	if err := os.WriteFile("artifacts/mutations.json", data, 0o644); err != nil {
		return err
	}
	return mutationReport(data)
}

func mutationReport(data []byte) error {
	var report struct {
		Total  int `json:"mutants_total"`
		Killed int `json:"mutants_killed"`
		Files  []struct {
			Mutations []struct {
				Status string `json:"status"`
			} `json:"mutations"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	if report.Total <= 0 || report.Killed != report.Total {
		return errors.New("mutation gate needs nonempty, entirely killed mutants; inspect artifacts/mutations.json")
	}
	count := 0
	for _, file := range report.Files {
		for _, mutation := range file.Mutations {
			if mutation.Status != "KILLED" {
				return fmt.Errorf("mutation status %s is not a killed compilable mutant", mutation.Status)
			}
			count++
		}
	}
	if count != report.Total {
		return errors.New("mutation report counts do not match")
	}
	return nil
}

func copySource(dst string) error {
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return err
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := copyFile(name, filepath.Join(dst, name)); err != nil && (name != "go.sum" || !errors.Is(err, os.ErrNotExist)) {
			return err
		}
	}
	data, err := output(".", "list", "-json", "./internal/...")
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var pkg struct {
			Dir                                                                                                                                                                             string
			GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles, TestGoFiles, XTestGoFiles, EmbedFiles, TestEmbedFiles, XTestEmbedFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			return err
		}
		if err := copyPackage(dst, pkg.Dir, [][]string{pkg.GoFiles, pkg.CgoFiles, pkg.CFiles, pkg.CXXFiles, pkg.MFiles, pkg.HFiles, pkg.FFiles, pkg.SFiles, pkg.SwigFiles, pkg.SwigCXXFiles, pkg.SysoFiles, pkg.TestGoFiles, pkg.XTestGoFiles, pkg.EmbedFiles, pkg.TestEmbedFiles, pkg.XTestEmbedFiles}); err != nil {
			return err
		}
	}
	return nil
}

func copyPackage(dst, dir string, groups [][]string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("package outside source root: %s", dir)
	}
	for _, group := range groups {
		for _, name := range group {
			if err := copyFile(filepath.Join(dir, name), filepath.Join(dst, rel, name)); err != nil {
				return err
			}
		}
	}
	return copyTestdata(dir, filepath.Join(dst, rel))
}

func copyTestdata(dir, dst string) error {
	testdata := filepath.Join(dir, "testdata")
	if _, err := os.Stat(testdata); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(testdata, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("source link rejected: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(dst, name))
	})
}

func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("source is not a regular file: %s", src)
	}
	name := filepath.Base(src)
	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return fmt.Errorf("environment file is not a milestone fixture: %s", src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}
