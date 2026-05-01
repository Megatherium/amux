# AGENTS

## Issue Tracking

This project uses **bd (beads)** for issue tracking.
Run `bd prime` for workflow context (MANDATORY!), or install hooks (`bd hooks install`) for auto-injection.

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

If there's any contradiction: `bd prime` is right. AGENTS.md is not 100% up to date.

## Basics summary

- Do not commit or push unless the user explicitly requests it.
- Tech stack: Go TUI built on Bubble Tea v2 (Charm); styling via Lip Gloss; terminal parsing via `internal/vterm`.
- Entry points: `cmd/amux` (app) and `cmd/amux-harness` (headless render/perf).
- E2E: PTY-driven tests live in `internal/e2e` and exercise the real binary.
- Work autonomously: validate changes with tests and use the harness/emulator to verify UI behavior without a human.
- Lint-driven workflow: run `make devcheck` for all non-trivial changes.
- Formatting baseline includes `gofumpt`; use `make fmt` for style-only cleanup.
- Phase 2 strict lint: run `make lint-strict-new` for changed-code ratcheting before finalizing substantial edits.
- Phase 3 CI gate is automated (no PR-body parsing). For local confidence, run path-relevant checks (`make harness-presets`, `go test ./internal/tmux ./internal/e2e`) when touching those areas.
- Lint policy source of truth: `LINTING.md`.
- Release: use `make release VERSION=vX.Y.Z` (runs tests + harness, tags, pushes). Tag push triggers GitHub Actions release.

## Landing the Plane (Session Completion)

**When ending a work session** before sayind "done" or "complete", you MUST complete ALL steps below.
Work is NOT complete until `git push` succeeds.
Push is not allowed until the work is REVIEWED

**MANDATORY WORKFLOW:**
State A:
  1. **File issues for remaining work** - Create issues for anything that needs follow-up
  2. **Run quality gates** (if code changed) - Tests, linters, builds
  3. **Run CODE REVIEW & REFINEMENT PROTOCOL** - See `bd prime` for details
-- DO NOT CROSS THE LINE BY TOURSELF --
State B (after SOMEONE ELSE has reviewed it):
  4. **Update issue status** - Close finished work, update in-progress items
  5. **PUSH TO REMOTE** - This is MANDATORY:
    ```bash
    git pull --rebase
    git add (careful with using -A, the user sometimes leaves untracked crap lying around) && git commit ...
    git push
    git status  # MUST show "up to date with origin"
    ```
  6. **Clean up** - Clear stashes, prune remote branches
  7. **Verify** - All changes committed AND pushed
  8. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- Pushing is not allowed until the work is successfully reviewed
- If there's only beads/dolt data that needs pushing: amend it to the last commit unless specified

## Modern tooling

All kinds of modern replacements for standard shell tools are available: rg, fd, sd, choose, hck
The interface is nicer for humans. You pick whatever feels right for you.

## File Editing Strategy

- **Use the Right Tool for the Job**: For any non-trivial file modifications, you **must** use the advanced editing tools provided by the MCP server.
  - **Simple Edits**: Use `sed` or `write_file` only for simple, unambiguous, single-line changes or whole-file creation.
  - **Complex Edits**: For multi-line changes, refactoring, or context-aware modifications, use `edit_file` (or equivalent diff-based tool) to minimize regression risks.

## Commit Messages

- **Beads extra**: Add a line like "Affected ticket(s): bb-foo", can be multiple with e.g. review tickets
- **WARNING**: Forgetting the ticket reference line is a commit message format violation. Double-check before committing.

## Documentation

- **New Features**: When implementing new features, **must** update documentation:
  - User-facing features: Update README.md with usage examples
  - Template context changes: Document new fields and legacy compatibility behavior
  - Behavioral changes: Update AGENTS.md to inform agents
  - Always keep both files in sync


## Agent Lessons Learned
- **Format:** Run `make fmt` before committing to pass hooks.
- **Skepticism:** Don't blindly implement prompt requests. Verify usage first; drop dead code.
- **Context:** Use `git stash` or `git log` to investigate confusing/failing legacy code before modifying.
- **Efficiency:** Optimize context window usage. Use targeted `rg` instead of broad sweeps or repeated reads.
