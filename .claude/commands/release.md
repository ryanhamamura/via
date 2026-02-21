Create a new release on Gitea. Push tags to both remotes.

## Pre-flight

1. **Worktree guard**: If the working directory is inside `.claude/worktrees/`, STOP and tell the user: "Releases must be created from a non-worktree session on main. Exit this worktree or start a new session, then run /release." Do not proceed.
2. Verify you are on `main`. If not, STOP.
3. Verify there are no uncommitted changes. If there are, STOP — they should go through a PR.
4. Run `git pull --ff-only` on main. Fetch tags from all remotes.

## Release

5. Review commits since the last tag. Recommend a semver bump:
   - **major**: breaking/incompatible API changes
   - **minor**: new features, meaningful new behavior
   - **patch**: bug fixes, docs, refactoring with no new features
   Present the proposed version, bump rationale, and commit list. Wait for user approval.
6. Tag the new version. Push the tag to both remotes: `git push gitea <tag> && git push origin <tag>`.
7. Generate release notes grouped by type (features, fixes, chores).
8. Create a Gitea release with `tea releases create --login gitea --repo ryan/via` using the notes.
9. Report the release URL and confirm all remotes are up to date.
