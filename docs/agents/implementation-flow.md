# Ticket implementation

How an agent turns a GitHub **ticket** into a PR. This clone checks out one ticket branch at a time.

## Language

**Ticket**:
A `ready-for-agent` GitHub issue. The only implementable. May stand alone or be a child of a spec.

**Spec**:
A parent GitHub issue that groups tickets. Not implemented and not merged. Closed when every child ticket is closed.

**Claim**:
`gh issue edit <n> --add-assignee @me`. An assigned open ticket is taken; other sessions skip it.

**Branch**:
`ticket/<n>-<slug>`, created from `origin/main`.

## Loop

### 1. Session start

`git fetch origin`. Delete local `ticket/*` branches whose ticket is closed or whose PR is merged. Leave branches with an open PR or unpushed commits.

Done when every stale merged `ticket/*` branch is gone.

### 2. Pick and claim

Take one unblocked `ready-for-agent` ticket (blockers merged to `main`). Claim it before any git write.

Done when `gh issue view <n>` shows the assignee is `@me`.

### 3. Branch

If `ticket/<n>-<slug>` already exists locally, check it out (resume).

Otherwise: `git fetch origin main` and `git checkout -b ticket/<n>-<slug> origin/main`.

This clone holds one ticket branch at a time. A blocked ticket waits until its blockers are on `main`.

Done when `git branch --show-current` is `ticket/<n>-<slug>` and the branch point is `origin/main` (new) or the existing ticket branch (resume).

### 4. Implement

Use `/tdd` at agreed seams when the ticket has product behavior. Run typechecks and focused tests during the work.

Done when the ticket's acceptance criteria are met in this branch.

### 5. Gates

Before a PR exists:

1. Relevant tests pass (full suite when the change can affect it).
2. `/code-review` against `main`; fix findings.
3. Lefthook `pre-commit` is green (`gofmt`, `golangci-lint`, `go build ./...`, `go test ./...`). Commit with Conventional Commits; fix hook failures and commit again until the hook passes.

Done when HEAD includes the work, the hook succeeded on the last commit, and review findings are fixed.

### 6. PR

Push `-u` and open a ready PR into `main` (not a draft at start). Title is a Conventional Commit. Body includes `Closes #<ticket>`. Add `Part of #<spec>` only when a spec exists. Ticket PRs close the ticket, never the spec.

You merge. The agent never merges.

Done when `gh pr view` shows an open, non-draft PR targeting `main`.

### 7. After the PR

`git checkout main`. The ticket branch stays locally until merge + cleanup.

Done when the working tree is on `main`.

### 8. CI or review comments

Check out the existing `ticket/<n>-<slug>` branch. Same claim, same branch. Fix, push, re-run gates. Still no merge.

Done when the new commits are on the PR and gates pass.

### 9. Close the spec

When every child ticket of a spec is closed, comment on the spec with the merged PR links and close it. No extra PR.

Done when `gh issue view <spec>` is closed.
