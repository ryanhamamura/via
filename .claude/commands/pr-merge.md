PR checks pass. Squash and merge the PR on both GitHub and Gitea.

1. Squash-merge on GitHub: `gh pr merge --squash` with a clean, semantic commit message including the PR number. No Claude attribution lines.
2. Squash-merge on Gitea: `tea pr merge <index> --style squash` with the same message.
3. Push main to gitea to keep commits in sync: `git push gitea main`.
4. Delete the remote feature branch on origin: `git push origin --delete <branch>`.
5. Delete the remote feature branch on gitea: `git push gitea --delete <branch>`.
6. Prune remote tracking refs: `git remote prune origin && git remote prune gitea`.
7. If in a worktree, leave the local branch alone — Claude Code handles worktree cleanup on session exit. If NOT in a worktree, delete the local feature branch and switch to main.
