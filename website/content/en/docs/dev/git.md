---
title: Git tips
weight: 30
---

## Squashing Commits

To combine multiple commits into one (recommended unless your PR covers multiple topics):

```bash
# Adjust the number based on how many commits you want to squash
git rebase -i HEAD~3
```

In the interactive editor that appears:
1. Keep the first commit as `pick`
2. Change subsequent commits from `pick` to `fixup` (short form`f`). You may also choose `squash` (`s`), however, `fixup` is recommended to keep the commit message clean.
3. Save and close the editor to proceed

Example:
```
pick aaaaaaa First commit message
pick bbbbbbb Second commit message
pick ccccccc Fix typo
```

To:
```
pick aaaaaaa First commit message
f bbbbbbb Second commit message
f ccccccc Fix typo
```

## Rebasing onto Upstream Master

To update your branch with the latest changes from upstream:

```bash
git remote add upstream https://github.com/lima-vm/lima.git  # Only needed once
git fetch upstream
git rebase upstream/master
```

## Troubleshooting

If you encounter issues during rebase:

```bash
git rebase --abort  # Cancel the rebase and return to original state
git status          # Check current state
```

For merge conflicts during rebase:
1. Resolve the conflicts in the files
2. `git add` the resolved files
3. `git rebase --continue`
