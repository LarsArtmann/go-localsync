# Branching Strategy

> **Last Updated:** 2026-03-27
> **Project:** go-localsync

---

## Overview

This document defines the branching strategy for go-localsync. Given the project's nature as an SDK with a small team, we use a **Trunk-Based Development** approach with short-lived feature branches.

---

## Branch Model

### Primary Branches

| Branch       | Purpose                            | Protection  |
| ------------ | ---------------------------------- | ----------- |
| `master`     | Main branch, production-ready code | ✅ Required |
| `feature/*`  | Feature development                | Optional    |
| `fix/*`      | Bug fixes                          | Optional    |
| `refactor/*` | Code refactoring                   | Optional    |

### Branch Lifecycle

```
master ──────────────────────────────────────────────────────────►
    │        │              │          │            │
    │        ▼              │          │            │
    │   feature/x ───────► │          │            │
    │        │              │          │            │
    │        └──────────────┘          │            │
    │                                  ▼            │
    │                             fix/y ───────►    │
    │                                  │            │
    │                                  └────────────┘
    │                                              │
    └──────────────────────────────────────────────┘
```

---

## Branch Naming

### Conventions

| Type     | Pattern                                | Example                              |
| -------- | -------------------------------------- | ------------------------------------ |
| Feature  | `feature/<ticket>-<short-description>` | `feature/42-add-gitlab-provider`     |
| Bug Fix  | `fix/<ticket>-<short-description>`     | `fix/99-fix-rate-limit-handling`     |
| Refactor | `refactor/<area>-<description>`        | `refactor/storage-interface-cleanup` |
| Chore    | `chore/<description>`                  | `chore/update-dependencies`          |

### Rules

- Use kebab-case for branch names
- Keep descriptions concise (max 5 words)
- Include ticket number when applicable
- No personal branches (e.g., `lars/...`)

---

## Workflow

### 1. Starting Work

```bash
# Always start from master
git checkout master
git pull origin master

# Create feature branch
git checkout -b feature/42-add-gitlab-provider
```

### 2. Making Changes

```bash
# Make changes, commit frequently
git add .
git commit -m "feat: add GitLab provider interface"

# Push branch
git push -u origin feature/42-add-gitlab-provider
```

### 3. Code Review

1. Open Pull Request against `master`
2. Fill PR template (see below)
3. Request review from maintainers
4. Address feedback
5. Squash and merge

### 4. Merging

```bash
# Squash and merge (preferred for clean history)
# Or merge commit (for complex features with multiple logical commits)
```

---

## Pull Request Template

```markdown
## Summary

<!-- What does this PR do? -->

## Type

- [ ] Feature
- [ ] Bug Fix
- [ ] Refactor
- [ ] Documentation
- [ ] Chore

## Test Plan

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing performed

## Checklist

- [ ] Code follows project style
- [ ] Linter passes (`golangci-lint run`)
- [ ] Tests pass (`go test ./...`)
- [ ] Documentation updated (if needed)
```

---

## CI/CD Pipeline

### Branch Protection

| Check      | Required           |
| ---------- | ------------------ |
| Tests      | ✅ Yes             |
| Lint       | ✅ Yes             |
| Build      | ✅ Yes             |
| PR Reviews | 1 approval minimum |

### Pipeline Stages

```
┌─────────────┐
│   Checkout  │
└──────┬──────┘
       ▼
┌─────────────┐
│  Setup Go   │
└──────┬──────┘
       ▼
┌─────────────┐     ┌─────────────┐
│    Test     │────►│    Lint     │
└──────┬──────┘     └──────┬──────┘
       │                    │
       ▼                    ▼
┌─────────────┐     ┌─────────────┐
│    Build    │◄────┤   Release   │
└─────────────┘     └─────────────┘
     (tags)
```

### Release Process

1. Create tag: `git tag v1.2.3`
2. Push tag: `git push origin v1.2.3`
3. CI builds binaries and creates GitHub release

---

## Hotfix Process

For critical production issues:

```bash
# Create hotfix branch from master
git checkout master
git pull origin master
git checkout -b fix/critical-security-issue

# Fix, commit, push
git commit -m "fix: critical security issue"
git push -u origin fix/critical-security-issue

# Open PR, get expedited review
# Merge to master
# Delete branch
```

---

## Best Practices

### Do's

✅ Keep branches short-lived (max 3-5 days)
✅ Commit early and often
✅ Write descriptive commit messages
✅ Rebase on master before merging
✅ Delete branches after merge

### Don'ts

❌ Don't commit directly to master
❌ Don't merge with conflicts
❌ Don't leave stale branches
❌ Don't skip CI checks
❌ Don't force push to master

---

## Rollback Procedure

### If a release causes issues:

```bash
# Revert to previous version
git revert <commit-hash>
git push origin master

# Or rollback tag
git checkout <previous-tag>
git tag -f latest
git push --force origin latest
```

---

## Reference

- [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow)
- [Trunk-Based Development](https://trunkbaseddevelopment.com/)
- [Conventional Commits](https://www.conventionalcommits.org/)
