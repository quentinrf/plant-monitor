# Claude Code Directives for plant-monitor

## Plan tracking

After completing a task from `plans/high-level-plan.md`, update its status row
from `❌` to `✅` in that file and include the plan update in the same commit as
the implementation work.

## Before touching dependency or toolchain versions

Do NOT downgrade `go` directives, module versions, or any other dependency
versions without first confirming with the user. If a build fails due to a
toolchain mismatch or missing network access, investigate the root cause and
ask before changing any version pins.

## Commit discipline

- One logical change per commit.
- Commit message format: `type(scope): short description` (conventional commits).
- Always push to the designated `claude/...` branch; never push to main/master
  without explicit permission.

## Branch naming

Development branches follow the pattern `claude/<description>-<sessionId>`.
Always verify the target branch before committing.
