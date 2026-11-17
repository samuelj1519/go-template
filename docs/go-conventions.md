# Go conventions

The linter config is the enforced subset. A rule with no checker name is the default practice and is not checked mechanically. Behavior below was measured with the pinned `golangci-lint` v2.15.0.

## Libraries and applications

A library is a package another module may import. Keep the exported surface small and stable, document it, and return concrete types. Wrap an error at that boundary with `%w` when this function knows something the caller cannot see.

An application is `cmd` and `internal`. `main` may print a diagnostic and exit. `internal` is not an API for other modules. The error, context, naming, and concurrency rules are the same. `internal/text` stays on `errors` and `strings` only (`depguard`).

`revive` `exported` checks every importable package, including `internal`. `package main` needs a package comment and is not an imported API.

## Enforced

- Handle a returned error (`errcheck`). `govet` `unusedresult` covers a smaller set.
- Use `errors.Is` and `errors.As`. When `fmt.Errorf` formats an error, use `%w` (`errorlint`). `govet` `errorsas` checks the pointer passed to `errors.As`.
- Do not capitalize an error string or end it with punctuation (`staticcheck` ST1005). A single-word string is not capitalization-checked.
- Name a sentinel `Err…` and an error type `…Error` (`errname`).
- A `//nolint` directive names one linter and gives a reason (`nolintlint`).
- Pass `context.Context` as the first parameter. `*testing.T` may come before it (`revive` `context-as-argument`). Do not store a context in a struct (`containedctx`). Do not replace a parent context with `context.Background` or `context.TODO` (`contextcheck`). Do not use a built-in type as a context key (`staticcheck` SA1029).
- Use the comma-ok form of a type assertion (`forcetypeassert`).
- Do not add an `init` function (`gochecknoinits`).
- Do not add a package-level variable (`gochecknoglobals`). The checker allows an `Err`/`err` sentinel and `regexp.MustCompile`. Do not reassign another package's `Err*` or `EOF` (`reassign`). Reassignment of a sentinel inside its own package is not checked.
- Use one short receiver name for a type, not `this` or `self` (`revive` `receiver-naming`). Do not mix pointer and value receivers (`recvcheck`).
- Document every exported name in an importable package (`revive` `exported`). Give every package a package comment (`revive` `package-comments`). `package main` may start with `Command`.
- Do not put a unit suffix on a `time.Duration` name (`revive` `time-naming`).
- A test helper calls `t.Helper()` (`thelper`).
- Do not return `any` or a named interface (`ireturn`). `error`, an anonymous interface, and a standard-library interface are allowed.
- Unused code, ineffective assignments, spelling, cyclomatic complexity 10, and a duplication threshold of 100 stay on (`unused`, `ineffassign`, `misspell`, `cyclop`, `dupl`). `gofmt` is required.

## Default practice

- Do not panic for an ordinary error. A panic for a broken invariant is separate. Neither is linted.
- Do not compare `err.Error()` to a string. Use `errors.Is`. String comparison is not linted.
- Do not wrap a standard-library error that already names the failure. `wrapcheck` is off: it flags `os`, `json`, and `exec` returns that are already specific. Add `%w` when this function has new context.
- A goroutine has an owner and an exit: cancel its context, close its channel, or wait on its `WaitGroup`. This is not linted. `noctx` is off, so a process-scoped command may call `exec.Command`. Pass a context when a caller can cancel.
- Package names are short, lower case, and singular. Do not use `util`, `common`, or `base`. This is not linted.
- A type should be useful at its zero value. This is not linted.
- Copy a slice or map before storing or returning it if the caller can mutate it. This is not linted.
- Put `defer` next to the call that acquires a file, body, or lock. `errcheck` requires the error to be handled. HTTP and SQL closers are not linted because this module has neither.
- Take a `time.Duration`, not an `int` plus a unit. `time-naming` only flags a unit suffix on a `time.Duration`. An `int` named `timeoutSeconds` is not linted.
- Prefer table-driven tests when cases share a shape. The table shape is not linted.
- Define a small interface at the consumer. A library does not declare an interface only so it can implement it. Placement and size are not linted. Returning `any` or a named interface is linted.
- `staticcheck` ST1000, ST1003, ST1016, ST1020, ST1021, and ST1022 stay at the `golangci-lint` default, which is off. Package comments and exported docs are the `revive` rules above. ST1000 ignores `package main`, and ST1020 does not require a comment to exist.

## Sources

Not fetched in this change. Entry names were not re-checked online.

- Google Go Style Guide, <https://google.github.io/styleguide/go/>. Guide: errors (strings, handling, `%w`, do not panic for ordinary errors), naming (package names, receiver names), commentary. Decisions: contexts (first parameter, do not store in a struct), receiver names. Best practices: goroutine lifetimes, small interfaces defined by the consumer, return concrete types, avoid `init`, avoid mutable package state, zero values, copy slices and maps at a boundary.
- Uber Go Style Guide, <https://github.com/uber-go/guide>. Error wrapping, sentinel names, do not panic, avoid `init`, avoid mutable globals, context as the first parameter and not in a struct, `time.Duration`, check type assertions, table-driven tests.
- Effective Go, <https://go.dev/doc/effective_go>. Errors, `init`, goroutines, commentary.
- Go Code Review Comments, <https://go.dev/wiki/CodeReviewComments>. Error strings, handle errors, receiver names, receiver type, contexts, package comments, package names, goroutine lifetimes, do not panic, `time.Duration` names.

Changes from those sources: `wrapcheck` and `noctx` are not enabled, for the reasons above. `gochecknoglobals` allows sentinels instead of banning every package variable. `ireturn` allows `error`, anonymous interfaces, and standard-library interfaces. `context-as-argument` allows `*testing.T` before the context. `nolintlint` already required a specific linter and a reason.
