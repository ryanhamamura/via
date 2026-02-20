Create a PR from the current branch on both GitHub and Gitea.

1. If in a worktree (working directory contains `.claude/worktrees/`), you are already on a feature branch — do NOT create a new one. Otherwise, create a new branch from main with a descriptive name.
2. Stage and commit all changes with a clean, semantic commit message. No Claude attribution lines.
3. Fetch latest main: `git fetch origin main`.
4. Rebase onto main: `git rebase origin/main`.
   - If conflicts occur, abort the rebase (`git rebase --abort`), analyze the conflicting files, write a plan to resolve them, and present the plan to the user before proceeding.
5. Push the branch to origin with `-u` (use `--force-with-lease` if the branch was already pushed).
6. Push the branch to gitea: `git push gitea <branch>`.
7. Create a GitHub PR: `gh pr create`. Reference related issues with `#X`. Only use `Closes #X` if the PR fully resolves the issue.
8. Create a Gitea PR: `tea pr create --head <branch> --base main` with the same title and description.
9. Report both PR URLs.
