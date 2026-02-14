Create a new release for this project. Steps:

1. Fetch tags from all remotes so the version list is current.
2. Check for uncommitted changes. If any exist, commit them with a clean semantic commit message. No Claude attribution lines.
3. Review the commits since the last tag. Based on their content, recommend a semver bump:
   - **major**: breaking/incompatible API changes
   - **minor**: new features, meaningful new behavior
   - **patch**: bug fixes, docs, refactoring with no new features
   Present the proposed version, the bump rationale, and the commit list. Wait for user approval before continuing.
4. Tag the new version and push the tag + commits to all remotes (origin, gitea, etc.).
5. Generate release notes from the commits since the last tag, grouped by type (features, fixes, docs/refactoring).
6. Create a GitHub release using `gh release create`.
7. Create a Gitea release using `tea releases create` with the same notes.
8. Report both release URLs and confirm all remotes are up to date.
