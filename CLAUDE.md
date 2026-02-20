# Via Project Instructions

## Workflow

All changes go through PRs:

1. Enter a worktree (`EnterWorktree`) at session start.
2. Make changes, commit with semantic messages.
3. `/pr` to push, open a PR, wait for CI, and squash-merge.

## Releasing

Run `/release` from a **non-worktree session on main**. It tags and publishes
what is already on main — it does not commit new changes.

## Worktree Usage

Always enter a worktree at the start of a session using the `EnterWorktree`
tool. This prevents parallel Claude Code sessions from interfering with each
other.
