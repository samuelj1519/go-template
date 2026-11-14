// Command template-tests prepares a local Go module proxy for native gonew.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const module = "github.com/samuelj1519/go-template/template"
const rootModule = "github.com/samuelj1519/go-template"
const gonew = "golang.org/x/tools/cmd/gonew@v0.52.0"
const version = "latest"

func main() {
	if len(os.Args) != 2 || os.Args[1] != "pack" {
		fmt.Fprintln(os.Stderr, "usage: go -C tools/template-tests run . pack")
		os.Exit(1)
	}
	if err := pack("../../template", "../../artifacts/proxy"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func files(root string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("template symlink rejected: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("nonregular template input: %s", rel)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = data
		return nil
	})
	return result, err
}

func pack(input, proxy string) error {
	inputs, err := files(input)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errors.New("empty template")
	}
	if !bytes.HasPrefix(inputs["go.mod"], []byte("module "+module+"\n")) {
		return errors.New("template module identity mismatch")
	}
	var names []string
	for name := range inputs {
		names = append(names, name)
	}
	sort.Strings(names)
	localVersion := inputVersion(inputs, names)
	archive, err := moduleArchive(inputs, names, localVersion)
	if err != nil {
		return err
	}
	info, err := json.Marshal(map[string]string{"Version": localVersion, "Time": "2026-11-14T00:00:00Z"})
	if err != nil {
		return err
	}
	outputs := map[string][]byte{localVersion + ".zip": archive, localVersion + ".mod": inputs["go.mod"], localVersion + ".info": info, "list": []byte(localVersion + "\n")}
	if err := writeProxy(proxy, outputs, info); err != nil {
		return err
	}
	fmt.Println("Local module proxy:", filepath.Clean(proxy), localVersion)
	return nil
}

func writeProxy(proxy string, outputs map[string][]byte, info []byte) error {
	path := filepath.Join(proxy, filepath.FromSlash(module), "@v")
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	for name, data := range outputs {
		if err := os.WriteFile(filepath.Join(path, name), data, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(filepath.Dir(path), "@latest"), info, 0o644)
}

func inputVersion(inputs map[string][]byte, names []string) string {
	var data []byte
	for _, name := range names {
		data = fmt.Appendf(data, "%d:%s%d:", len(name), name, len(inputs[name]))
		data = append(data, inputs[name]...)
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("v0.0.0-20261114000000-%x", hash[:6])
}

func moduleArchive(inputs map[string][]byte, names []string, version string) ([]byte, error) {
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := writer.Create(module + "@" + version + "/" + name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(inputs[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

func bootstrap(root string) error {
	inputs, err := files(filepath.Join(root, "template"))
	if err != nil {
		return err
	}
	if err := compareBootstrap(root, inputs); err != nil {
		return err
	}
	return checkRootOwnership(root, inputs)
}

func compareBootstrap(root string, inputs map[string][]byte) error {
	for name, want := range inputs {
		want, err := bootstrapFile(name, want)
		if err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		got, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(got, want) {
			return fmt.Errorf("bootstrap drift in %s (edit template first and copy to root)", name)
		}
		actual, err := os.Stat(path)
		if err != nil {
			return err
		}
		expected, err := os.Stat(filepath.Join(root, "template", filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if actual.Mode().Perm() != expected.Mode().Perm() {
			return fmt.Errorf("bootstrap file mode differs: %s", name)
		}
	}
	return nil
}

func bootstrapFile(name string, data []byte) ([]byte, error) {
	if name == "go.mod" {
		return bytes.Replace(data, []byte("module "+module+"\n"), []byte("module "+rootModule+"\n"), 1), nil
	}
	if !strings.HasSuffix(name, ".go") {
		return data, nil
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, data, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	for i := len(file.Imports) - 1; i >= 0; i-- {
		item := file.Imports[i]
		path, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return nil, err
		}
		if path != module && !strings.HasPrefix(path, module+"/") {
			continue
		}
		start, end := fset.Position(item.Path.Pos()).Offset, fset.Position(item.Path.End()).Offset
		replacement := strconv.Quote(rootModule + strings.TrimPrefix(path, module))
		data = append(append(append([]byte{}, data[:start]...), replacement...), data[end:]...)
	}
	return data, nil
}

func checkRootOwnership(root string, inputs map[string][]byte) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if excluded, err := excludedRootPath(rel, entry); excluded {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		switch rel {
		case "CONTRIBUTING.md", ".github/workflows/template.yml":
			return nil
		}
		if _, ok := inputs[rel]; !ok {
			return fmt.Errorf("unowned root file: %s", rel)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("root link rejected: %s", rel)
		}
		return nil
	})
}

func excludedRootPath(rel string, entry fs.DirEntry) (bool, error) {
	switch rel {
	case ".git", "artifacts", "template", "tools/template-tests", ".worktrees":
		if entry.IsDir() {
			return true, filepath.SkipDir
		}
		if rel == ".git" && entry.Type().IsRegular() {
			return true, nil
		}
		return true, fmt.Errorf("expected a directory at excluded runtime path: %s", rel)
	default:
		return false, nil
	}
}

func proxyURL(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
