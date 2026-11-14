# Maintaining the Go template

This repository maintains a runnable Go project and its generation input. `template/` is authoritative. The root module is `github.com/samuelj1519/go-template`; the distributed input is the nested module `github.com/samuelj1519/go-template/template`. Shared structure, policy, and behavior are identical. Bootstrap checks allow only the module declaration and parsed Go import paths to reflect the root identity; every other byte and file mode must match. Edit shared input first, then update the root counterpart. Root-only files are this guide, tools/template-tests/, and .github/workflows/template.yml. Runtime artifacts/ and .git/ are excluded by exact relative path.

Template maintenance adopts dev-standard 1.0 draft at `e9ddb3f7862314d9675efc0316b86b4d6fd3d111`, entry /Users/shushu/code/dev-standard/standard/README.md. Read that revision before governed work. Each generated project selects its own development standard and permissions.

## Create a project

Install Go, then run one command from the directory that will contain the new project:

```sh
go run golang.org/x/tools/cmd/gonew@v0.52.0 github.com/samuelj1519/go-template/template@main github.com/acme/orders ./orders
```

Go automatically downloads, builds, and caches the pinned `gonew` tool and its dependencies. It also downloads the template. The first uncached run needs network access; a separate `go install` is unnecessary. Gonew remains a maintenance tool rather than an application dependency.

The generated module is `github.com/acme/orders`, imports use that prefix, and files are written to `./orders`. Supply your actual repository module path. Function-oriented package names such as `main` and `text` remain unchanged. Hyphenated module paths and major-version suffixes such as `example.com/team/orders/v2` are supported; pass the directory explicitly to keep its name independent of that suffix. Choose a new destination path. Gonew accepts empty directories and follows normal filesystem symlink behavior.

`@main` selects the current template branch. Pin its commit or a published module version for reproducible creation. For a nested-module release `v0.1.0`, use repository tag `template/v0.1.0`.

The Go team's gonew tool is experimental, so this project pins its version and tests actual generation and module rewriting.

## Test an unpublished local input

Prepare the local module proxy:

```sh
go -C tools/template-tests run . pack
```

From this repository, create a project in one command (POSIX shell):

```sh
GOPROXY="file://$PWD/artifacts/proxy,https://proxy.golang.org" GONOSUMDB=github.com/samuelj1519/go-template/template go run golang.org/x/tools/cmd/gonew@v0.52.0 github.com/samuelj1519/go-template/template@latest example.com/my-app ../my-app
```

On PowerShell, set the same two environment variables using `$env:GOPROXY = ([Uri](Join-Path $PWD 'artifacts/proxy')).AbsoluteUri + ',https://proxy.golang.org'` and `$env:GONOSUMDB = 'github.com/samuelj1519/go-template/template'`, then run the same `go run` command. Restore previous environment values after the local test.

The local proxy is generated from template/ for maintenance tests. Repack after changes; its content-derived pseudo-version keeps Go's immutable module cache correct. `@latest` resolves that local input. Local pseudo-versions are fixtures, not published releases. Public generation uses the command above without proxy overrides.

## Verify maintenance

```sh
go run ./tools/check prepare
go run ./tools/check milestone
go -C tools/template-tests vet ./...
go -C tools/template-tests run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run --config ../../.golangci.yml
go -C tools/template-tests test -count=1 -v ./...
```

The contract suite invokes pinned gonew through `go run`, builds and checks independent generated projects, and verifies ordinary, hyphenated, and versioned module identities, package names, directory safety, privacy, deterministic exports, bootstrap parity, and compilable incorrect implementations. GONEW_BIN can select an already-prepared absolute executable path. The default needs no preinstalled gonew binary. The local packager uses the Go module proxy protocol; it does not rewrite project source.

## Runtime artifacts

`artifacts/` is ignored by Git and excluded from generation inputs. Full checks create coverage.out; milestones create mutations.json. Maintainer packing creates proxy/. Acceptance logs, review reports, source identities, and small synthetic reproductions can be stored under acceptance/ and archived to CI or review attachments. Store reproduction source as .go.txt or compressed archives: Git ignore rules do not exclude directories from Go's recursive package discovery. Archive necessary evidence before cleanup. These records are not project documentation or generated-project input.

Optional Git hook activation creates artifacts/hooks/. Preserve that directory while core.hooksPath points to it; disable or relocate the hook before cleaning it. Other generated projects create their own reports when their checks run.

Use a new independent non-author session to review nontrivial changes and test differences. Record the fixed candidate, actual commands, failures and remaining checks in the task or review record. Keep sole file ownership during parallel work. Run normal Git hooks and verify existing authorization for commits, integrations, publication, and remote changes.
