# Go project

An MIT-licensed Go CLI with a standard-library text normalization example.

Use Go 1.27.4, a C compiler for the race detector, and network access to download pinned development tools and current vulnerability data.

```sh
go run ./cmd/app "  Hello WORLD  "
go run ./tools/check quick
go run ./tools/check full
go run ./tools/check milestone
```

The CLI prints `hello world`. Empty input or a failed stdout write prints a diagnostic to stderr and exits 1.
`quick` checks formatting, vet, lint, and tests. `full` adds race detection, scoped coverage, a real executable smoke test, module integrity, dependency policy, and vulnerability scanning. `milestone` also runs mutation testing and bounded fuzzing. Commands fail when a required tool or measurement fails.

`cmd/app` owns command-line I/O; `internal/text` owns pure rules; `tools/check` owns verification. See [architecture](docs/architecture.md), [verification](docs/verification.md), and [Go conventions](docs/go-conventions.md).

Prepare tools explicitly with `go run ./tools/check prepare`. On POSIX systems, enable optional commit checks:

```sh
mkdir -p artifacts/hooks
cp .githooks/pre-commit artifacts/hooks/pre-commit
chmod +x artifacts/hooks/pre-commit
git config --local core.hooksPath artifacts/hooks
```

Hooks run the same quick checks. New projects initialize their own Git repository and configure their own hosting rules.

[MIT](LICENSE). Preserve the original copyright notice and add notices for your contributions.
