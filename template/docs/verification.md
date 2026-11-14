# Verification

Run checks from the module root. `prepare` downloads and builds pinned tools explicitly; later commands use the Go cache. Quick checks include gofmt, vet, strict golangci-lint, and all module tests. The linter checks unused code, errors, spelling, justified exemptions, dependencies, duplication (100 tokens), and cyclomatic complexity (10).

Full checks require race tests, a nonempty atomic coverage profile for every production file under internal/, and 100% covered statements. The initial example establishes this baseline; keep the threshold and file completeness check when extending it. The CLI is tested through its compiled executable, including stdout, stderr, and failure exit code. Verification tools have their own behavioral tests and do not inflate application coverage.

Full checks verify Go module hashes, require the reviewed dependency graph (initially only this module), and run govulncheck against current Go vulnerability data. A new dependency requires explicit source and license review before changing that policy. The MIT notice covers scaffold code; the Go distribution and development tools retain their own licenses.

Milestones run full checks, five seconds of fuzzing, and Gremlins with 100% efficacy and mutant-coverage thresholds. Review the number of tested mutants: an empty report is a failure. A killed mutant must be compilable; invalid mutations are not proof. Fuzzing and mutation runs use disposable independent source/build directories.

Review these four architecture health rules at milestones: pure-core imports, command-to-core dependency direction, Go's internal-package encapsulation, and separation of application and verification coverage. Check document links, exceptions, dependency inventory, duplication, complexity, test strength, rework, review findings, regressions, and duplicate-code growth. Record unknown metrics as unknown.

Use an independent non-author reviewer for nontrivial changes and meaningful incorrect implementations to verify critical protections. Save commands, versions, exit codes, raw output, and input identity in ignored artifacts/ or CI artifacts. Missing or failing checks block integration. Hosting administrators configure and test required checks and branch protection for each project.
