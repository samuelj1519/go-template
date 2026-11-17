package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMaintenanceFormatting(t *testing.T) {
	inputs, err := files(".")
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range inputs {
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		formatted, err := format.Source(data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, formatted) {
			t.Fatalf("%s needs gofmt; run gofmt -w from tools/template-tests", name)
		}
	}
}

func repository(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestBootstrap(t *testing.T) {
	if err := bootstrap(repository(t)); err != nil {
		t.Fatal(err)
	}
}

func TestReverseBootstrap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "template"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "template", "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"extra.go", "nested/artifacts/extra.go", "nested/tools/template-tests/extra.go"} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("unexpected"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := bootstrap(root); err == nil {
			t.Fatalf("accepted extra file %s", name)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorktreeBootstrap(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "template"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"go.mod", "template/go.mod"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("module example.com/app\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: /synthetic/worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "extra.go"), []byte("package extra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(root); err == nil {
		t.Fatal("worktree .git file hid an unowned root source")
	}
}

func TestPackReproducible(t *testing.T) {
	root := repository(t)
	a, b := t.TempDir(), t.TempDir()
	for _, dir := range []string{a, b} {
		if err := pack(filepath.Join(root, "template"), dir); err != nil {
			t.Fatal(err)
		}
	}
	aFiles, err := files(a)
	if err != nil {
		t.Fatal(err)
	}
	bFiles, err := files(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(aFiles) != len(bFiles) {
		t.Fatal("proxy file count differs")
	}
	for name, data := range aFiles {
		if !bytes.Equal(data, bFiles[name]) {
			t.Fatalf("nondeterministic proxy %s", name)
		}
	}
}

func TestPackExcludesNonSourceInputs(t *testing.T) {
	root := t.TempDir()
	inputs := map[string]string{
		"go.mod":                    "module " + module + "\n",
		"nested/artifacts/keep.txt": "ordinary nested source fixture",
		".env":                      "FAKE_TOKEN=synthetic",
		".env.local":                "FAKE_TOKEN=synthetic",
		"nested/.env":               "FAKE_TOKEN=synthetic",
		"artifacts/coverage.out":    "synthetic runtime output",
		"bin/app":                   "synthetic binary",
		".worktrees/review/go.mod":  "synthetic worktree",
		"cpu.test":                  "synthetic test binary",
		"nested/cpu.test":           "synthetic test binary",
		".git/config":               "synthetic repository metadata",
		".handoff":                  "synthetic maintainer state",
	}
	writeFiles(t, root, inputs)
	proxy := filepath.Join(t.TempDir(), "proxy")
	if err := pack(root, proxy); err != nil {
		t.Fatal(err)
	}
	got := packedFiles(t, proxy)
	for name := range inputs {
		if name != "go.mod" && name != "nested/artifacts/keep.txt" && got[name] {
			t.Errorf("packed non-source input %s", name)
		}
	}
	for _, name := range []string{"go.mod", "nested/artifacts/keep.txt"} {
		if !got[name] {
			t.Errorf("omitted source input %s", name)
		}
	}
	if len(got) != 2 {
		t.Errorf("archive contains %d files, want 2", len(got))
	}
}

func writeFiles(t *testing.T, root string, inputs map[string]string) {
	t.Helper()
	for name, data := range inputs {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func packedFiles(t *testing.T, proxy string) map[string]bool {
	t.Helper()
	outputs, err := files(proxy)
	if err != nil {
		t.Fatal(err)
	}
	var archiveData []byte
	for name, data := range outputs {
		if strings.HasSuffix(name, ".zip") {
			archiveData = data
		}
	}
	archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]bool)
	for _, entry := range archive.File {
		remainder := strings.TrimPrefix(entry.Name, module+"@")
		_, name, ok := strings.Cut(remainder, "/")
		if !ok {
			t.Fatalf("invalid archive entry %q", entry.Name)
		}
		result[name] = true
	}
	return result
}

func TestTemplateLinksRejected(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("missing", filepath.Join(root, "link")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("Windows runner lacks symlink privilege")
		}
		t.Fatal(err)
	}
	if _, err := files(root); err == nil {
		t.Fatal("accepted dangling template link")
	}
}

func command(t *testing.T, dir string, env []string, name string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	data, err := cmd.CombinedOutput()
	t.Logf("cwd=%s command=%s %v\n%s", dir, name, args, data)
	return data, err
}

func generate(t *testing.T, base, proxy, name, destination string) error {
	t.Helper()
	tool := os.Getenv("GONEW_BIN")
	args := []string{module + "@" + version, name, destination}
	if tool == "" {
		tool = "go"
		args = append([]string{"run", gonew}, args...)
	}
	env := append(os.Environ(), "GOPROXY="+proxy+",https://proxy.golang.org", "GONOSUMDB="+module, "GOWORK=off", "GOMODCACHE="+filepath.Join(base, "mod"), "GOFLAGS=-modcacherw")
	_, err := command(t, base, env, tool, args...)
	return err
}

func TestNativeGeneration(t *testing.T) {
	root := repository(t)
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	proxy := filepath.Join(base, "proxy with spaces")
	if err := pack(filepath.Join(root, "template"), proxy); err != nil {
		t.Fatal(err)
	}
	proxyPath := proxyURL(proxy)
	name := "example.com/acme/my-project/v2"
	a, b := filepath.Join(base, "project-a"), filepath.Join(base, "project-b")
	for _, dst := range []string{a, b} {
		if err := generate(t, base, proxyPath, name, dst); err != nil {
			t.Fatal(err)
		}
	}
	assertGeneratedFiles(t, root, name, a, b)
	assertPrivateFilesAbsent(t, a)
	checkGeneratedProjects(t, base, a, b)
	assertDestinationSafety(t, base, proxyPath, name, a)
	assertSourceIdentity(t, base, name, a)
	assertImports(t, base, name, a)
}

func TestModuleNames(t *testing.T) {
	root := repository(t)
	for _, name := range []string{"github.com/acme/orders", "github.com/acme/my-project", rootModule} {
		t.Run(strings.TrimPrefix(name, "github.com/"), func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			proxy := filepath.Join(base, "proxy")
			if err := pack(filepath.Join(root, "template"), proxy); err != nil {
				t.Fatal(err)
			}
			project := filepath.Join(base, "destination")
			if err := generate(t, base, proxyURL(proxy), name, project); err != nil {
				t.Fatal(err)
			}
			assertSourceIdentity(t, base, name, project)
			assertImports(t, base, name, project)
			if name == rootModule {
				if err := compareBootstrap(root, readFiles(t, project)); err != nil {
					t.Fatal(err)
				}
			}
			env := append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build"))
			if _, err := command(t, project, env, "go", "test", "./..."); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func assertImports(t *testing.T, base, name, project string) {
	t.Helper()
	env := append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build"))
	data, err := command(t, project, env, "go", "list", "-f", "{{.Name}} {{.ImportPath}} {{join .Imports \" \"}}", "./cmd/app", "./internal/text")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(module+"/")) {
		t.Fatalf("stale template import: %s", data)
	}
	for _, want := range []string{"main " + name + "/cmd/app", "text " + name + "/internal/text"} {
		if !bytes.Contains(data, []byte(want)) {
			t.Fatalf("package identity missing %q: %s", want, data)
		}
	}
}

func TestBootstrapIdentityMapping(t *testing.T) {
	path := module + "/internal/text"
	source := []byte("package example\n// " + path + " stays in a comment\nimport alias \"" + path + "\"\nvar note = \"" + path + "\"\n")
	want := bytes.Replace(source, []byte("import alias \""+path+"\""), []byte("import alias \""+rootModule+"/internal/text\""), 1)
	got, err := bootstrapFile("example.go", source)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("parsed identity mapping: %s (%v)", got, err)
	}
	got, err = bootstrapFile("README.md", source)
	if err != nil || !bytes.Equal(got, source) {
		t.Fatal("rewrote a non-Go file")
	}
	if _, err := bootstrapFile("broken.go", []byte("invalid syntax")); err == nil {
		t.Fatal("accepted invalid Go source")
	}
}

func assertGeneratedFiles(t *testing.T, root, name, a, b string) {
	t.Helper()
	aFiles := readFiles(t, a)
	bFiles := readFiles(t, b)
	inputs, err := templateFiles(filepath.Join(root, "template"))
	if err != nil {
		t.Fatal(err)
	}
	if len(aFiles) != len(inputs) || len(bFiles) != len(inputs) {
		t.Fatalf("export file counts differ: input %d, a %d, b %d", len(inputs), len(aFiles), len(bFiles))
	}
	for path, want := range inputs {
		if strings.HasSuffix(path, ".go") || path == "go.mod" {
			want = bytes.ReplaceAll(want, []byte(module), []byte(name))
		}
		if !bytes.Equal(aFiles[path], want) || !bytes.Equal(aFiles[path], bFiles[path]) {
			t.Fatalf("generated file differs: %s", path)
		}
	}
}

func readFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	result, err := files(root)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertPrivateFilesAbsent(t *testing.T, project string) {
	t.Helper()
	for _, name := range []string{"CONTRIBUTING.md", "tools/template-tests/main.go", ".github/workflows/template.yml", ".git/config", ".handoff", "artifacts"} {
		if _, err := os.Stat(filepath.Join(project, filepath.FromSlash(name))); !os.IsNotExist(err) {
			t.Fatalf("exported maintainer/runtime path: %s (%v)", name, err)
		}
	}
}

func checkGeneratedProjects(t *testing.T, base, a, b string) {
	t.Helper()
	env := append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build-a"))
	if _, err := command(t, a, env, "go", "run", "./tools/check", "full"); err != nil {
		t.Fatal(err)
	}
	if _, err := command(t, b, append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build-b")), "go", "test", "./..."); err != nil {
		t.Fatal(err)
	}
}

func assertDestinationSafety(t *testing.T, base, proxyPath, name, project string) {
	t.Helper()
	before, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(t, base, proxyPath, name, project); err == nil {
		t.Fatal("overwrote occupied destination")
	}
	after, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("occupied project changed")
	}
	if err := generate(t, base, proxyPath, "invalid path", filepath.Join(base, "invalid")); err == nil {
		t.Fatal("accepted invalid module")
	}
}

func assertSourceIdentity(t *testing.T, base, name, project string) {
	t.Helper()
	env := append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build-a"))
	metadata, err := command(t, project, env, "go", "list", "-m", "-json")
	if err != nil {
		t.Fatal(err)
	}
	var info struct{ Path, Dir string }
	if err := json.Unmarshal(metadata, &info); err != nil {
		t.Fatal(err)
	}
	if info.Path != name || filepath.Clean(info.Dir) != filepath.Clean(project) {
		t.Fatalf("wrong source identity: %+v", info)
	}
}

func TestProxyURL(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/tmp/my proxy", "file:///tmp/my%20proxy"},
		{`C:\Users\runner\my proxy`, "file:///C:/Users/runner/my%20proxy"},
		{"C:/Users/runner/proxy", "file:///C:/Users/runner/proxy"},
	} {
		if got := proxyURL(tc.path); got != tc.want {
			t.Fatalf("proxyURL(%q) = %q; want %q", tc.path, got, tc.want)
		}
	}
}

func TestBehaviorMutations(t *testing.T) {
	root := repository(t)
	input, err := os.ReadFile(filepath.Join(root, "template/internal/text/normalize.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, new string }{
		{"accept-empty", "len(words) == 0", "len(words) < 0"},
		{"keep-case", "strings.ToLower(strings.Join(words, \" \"))", "strings.Join(words, \" \")"},
		{"wrong-separator", "strings.Join(words, \" \")", "strings.Join(words, \"_\")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			source := filepath.Join(base, "source")
			copyTemplate(t, root, source)
			if bytes.Count(input, []byte(tc.old)) != 1 {
				t.Fatal("mutation anchor changed")
			}
			changed := bytes.Replace(input, []byte(tc.old), []byte(tc.new), 1)
			if err := os.WriteFile(filepath.Join(source, "internal/text/normalize.go"), changed, 0o644); err != nil {
				t.Fatal(err)
			}
			env := append(os.Environ(), "GOWORK=off", "GOCACHE="+filepath.Join(base, "build"))
			if _, err := command(t, source, env, "go", "test", "-run=^$", "./internal/..."); err != nil {
				t.Fatalf("mutation does not compile: %v", err)
			}
			out, err := command(t, source, env, "go", "test", "-count=1", "./internal/...")
			if err == nil || !bytes.Contains(out, []byte("--- FAIL: TestNormalize")) {
				t.Fatalf("mutation survived or failed outside assertions: %s (%v)", out, err)
			}
		})
	}
}

func copyTemplate(t *testing.T, root, source string) {
	t.Helper()
	inputs, err := templateFiles(filepath.Join(root, "template"))
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range inputs {
		dst := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func Example_generationCommand() {
	fmt.Printf("gonew %s@%s example.com/my-app\n", module, version)
	// Output: gonew github.com/samuelj1519/go-template/template@latest example.com/my-app
}
