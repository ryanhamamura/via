Create a PR, wait for CI, and squash-merge it on both GitHub and Gitea. This is the standard single-command workflow.

1. If in a worktree (working directory contains `.claude/worktrees/`), you are already on a feature branch — do NOT create a new one. Otherwise, create a new branch from main with a descriptive name.
2. Stage and commit all changes with a clean, semantic commit message. No Claude attribution lines.
3. Fetch latest main and rebase: `git fetch origin main && git rebase origin/main`.
   - If conflicts occur, abort the rebase (`git rebase --abort`), analyze the conflicting files, write a plan to resolve them, and present the plan to the user before proceeding.
4. Push the branch to origin with `-u` (use `--force-with-lease` if already pushed). Also push to gitea.
5. Create a GitHub PR: `gh pr create`. Reference related issues with `#X`. Only use `Closes #X` if the PR fully resolves the issue.
6. Create a Gitea PR: `tea pr create --head <branch> --base main` with the same title and description.
7. Wait for CI to pass: poll with `gh pr checks` or `gh run watch`. If CI fails, report the failure and stop — do not merge.
8. Once CI passes, squash-merge on GitHub: `gh pr merge --squash` with a clean, semantic commit message including the PR number. No Claude attribution lines.
9. Squash-merge on Gitea: `tea pr merge <index> --style squash` with the same message.
10. Push main to gitea: `git push gitea main`.
11. Clean up remote branches: `git push origin --delete <branch> && git push gitea --delete <branch>`.
12. Prune refs: `git remote prune origin && git remote prune gitea`.
13. Report both merged PR URLs.
