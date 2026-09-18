## Agent skills

### Issue tracker

Issues live in this repo's GitHub Issues; use the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Canonical roles map 1:1 to tracker labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Conventional commits

Agents must use [Conventional Commits](https://www.conventionalcommits.org/) for every git commit: `<type>(optional-scope): <description>` (imperative, lowercase, no trailing period). Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `build`, `ci`, `perf`, `style`. Breaking changes use `!` after the type/scope or a `BREAKING CHANGE:` footer. Author is the repo's configured user only: no `Co-authored-by` trailers, no agent name in the message.

### Ticket implementation

Claim, branch from `main`, implement, PR gates, resume, or cleanup: see `docs/agents/implementation-flow.md`.
