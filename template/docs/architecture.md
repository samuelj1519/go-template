# Architecture

The module path in go.mod is the import prefix for packages in this project. Use the repository's module path when creating the project and inspect it with `go list -m`. Package identifiers describe their responsibility: `main` is the executable entry and `text` owns the example rule. They do not need to match the repository or destination directory name.

`cmd/app` parses process arguments, invokes `internal/text.Normalize`, and writes stdout or stderr. Empty or whitespace-only input returns the exported `ErrEmpty`; success lowercases words and joins Unicode whitespace with one ASCII space. Failed stdout writes exit 1; stderr diagnostics are best effort.

`internal/text` imports only `errors` and `strings`. Golangci-lint's depguard checks the boundary. Put new transport or storage responsibilities outside this package and extend the architecture deliberately when requirements change.

`tools/check` is a standard-library verification CLI. It launches pinned tools through `go run module@version`; those modules are development tools, separate from the application's module graph. Tool versions are centralized in tools/check/main.go. Tool downloads use Go's checksum database.

Application dependencies start empty. Adding one requires a project-reviewed source and license policy change and corresponding verification updates.
