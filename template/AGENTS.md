# Agent instructions

Confirm the directory, branch, existing changes, task scope, authorization, and adopted development standard before editing. Read CONTRIBUTING.md when present.

Use `go run ./tools/check quick` after changes and `full` before integration. Follow docs/verification.md for coverage, security, milestones, and evidence. Preserve existing assertions; quality-policy reductions require an explicit project decision.

Follow docs/go-conventions.md for Go style.

Keep external input and I/O in cmd/app and pure rules in internal/text. Add packages when a concrete responsibility needs them. Assign one writer per file or branch; use separate worktrees for parallel agents and agree shared interfaces before implementation.

Use synthetic inputs and physically separate candidate, mutation, and build directories. Confirm the actual executable and source paths. Obtain a new non-author review for nontrivial changes. Record the tested version, commands, failures, and remaining checks in the task or review. Keep runtime output under ignored artifacts/.

Check existing authorization for dependencies, license policy, installations, commits, integration, remote operations, and real data. Use normal hooks and preserve other contributors' changes.
