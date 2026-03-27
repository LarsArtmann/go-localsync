# Branch Protection Rules

> **Last Updated:** 2026-03-27
> **Project:** go-localsync

---

## Overview

Branch protection rules define who can push to branches and what requirements must be met before merging.

---

## Repository Settings

Configure these rules in GitHub: **Settings → Branches → Branch protection rules**

---

## Protected Branches

### `master`

| Setting                               | Value  | Rationale                           |
| ------------------------------------- | ------ | ----------------------------------- |
| Require a pull request before merging | ✅ Yes | All changes must go through PR      |
| Require approvals                     | 1      | At least one review required        |
| Dismiss stale approvals               | ✅ Yes | Re-review after changes             |
| Require review from Code Owners       | ❌ No  | Solo project for now                |
| Allow force pushes                    | ❌ No  | Prevent history rewriting           |
| Allow deletions                       | ❌ No  | Protect against accidental deletion |
| Block force pushes                    | ✅ Yes | Extra protection                    |
| Require linear history                | ✅ Yes | Cleaner git history                 |
| Require merge queue                   | ❌ No  | Not needed for small team           |
| Do not allow bypassing                | ✅ Yes | Even admins must follow rules       |

### CI/CD Requirements

| Check                             | Required | Description                 |
| --------------------------------- | -------- | --------------------------- |
| Status checks                     | ✅ Yes   | All checks must pass        |
| Require branches to be up to date | ❌ No    | Allow outdated PRs to merge |
| Lock branch                       | ❌ No    | Allow commits               |

---

## Required Status Checks

### Must Pass

```
✅ test
✅ lint
✅ build
```

### Optional (Recommended)

- Code coverage check
- Security scan
- Dependency review

---

## Code Owner Review

For solo projects, skip Code Owner review. As team grows:

```markdown
# CODEOWNERS

- @maintainer/team
  pkg/\* @maintainer/core
```

---

## Emergency Bypass

In case of emergency:

1. Create temporary branch from master
2. Apply fix directly
3. Create expedited PR with `[HOTFIX]` prefix
4. Get verbal approval from maintainer
5. Merge with approval

---

## GitHub CLI Setup

```bash
# Install GitHub CLI
brew install gh

# Login
gh auth login

# View current protection rules
gh api repos/{owner}/{repo}/branches/master/protection

# Update protection rules (requires admin)
gh api repos/{owner}/{repo}/branches/master/protection \
  --method PUT \
  --field required_status_checks='{"strict":true,"contexts":["test","lint","build"]}' \
  --field enforce_admins=true \
  --field required_pull_request_reviews='{"required_approving_review_count":1}'
```

---

## Automation

Consider using [Branch Protection Pro](https://github.com/apps/branch-protection-pro) or similar tools for advanced rule management.

---

## Checklist

- [ ] Require PR before merge
- [ ] Require 1 approval
- [ ] Dismiss stale approvals
- [ ] Block force pushes
- [ ] Require linear history
- [ ] Configure required status checks
- [ ] Enable admin enforcement
- [ ] Set up CODEOWNERS (when team grows)
