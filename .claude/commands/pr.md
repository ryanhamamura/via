Create a PR on Gitea, wait for CI, and squash-merge it. Push code to both remotes.

1. If in a worktree (working directory contains `.claude/worktrees/`), you are already on a feature branch — do NOT create a new one. Otherwise, create a new branch from main with a descriptive name.
2. Stage and commit all changes with a clean, semantic commit message. No Claude attribution lines.
3. Fetch latest main and rebase: `git fetch gitea main && git rebase gitea/main`.
   - If conflicts occur, abort the rebase (`git rebase --abort`), analyze the conflicting files, write a plan to resolve them, and present the plan to the user before proceeding.
4. Push the branch to both remotes: `git push -u gitea <branch> && git push origin <branch>` (use `--force-with-lease` if already pushed).
5. Create a Gitea PR: `tea pr create --login gitea --repo ryan/via --head <branch> --base main`. Reference related issues with `#X`. Only use `Closes #X` if the PR fully resolves the issue.
6. Wait for CI to pass: poll Gitea CI status. If CI fails, report the failure and stop — do not merge.
7. Once CI passes, squash-merge on Gitea: `tea pr merge --login gitea --repo ryan/via <index> --style squash` with a clean, semantic commit message including the PR number. No Claude attribution lines.
8. Update local main and push to both remotes. If in a worktree, `main` is checked out in the primary tree, so run from there: `cd <primary-worktree> && git pull gitea main && git push origin main` (the primary worktree path is the repo root without `.claude/worktrees/…`). If not in a worktree: `git checkout main && git pull gitea main && git push origin main`.
9. Clean up remote branches: `git push gitea --delete <branch> && git push origin --delete <branch>`.
10. Prune refs: `git remote prune gitea && git remote prune origin`.
11. Report the merged PR URL.
