# Review reports

This folder holds independent code and UX review reports, for example
`review-<topic>-<date>.md`.

Review reports are **inputs to implementation, not product specifications**. A finding is
advisory until it is accepted and implemented: the reviewer may be wrong, may not know the
frozen plan, and may propose changes that conflict with a product-owner decision.

Rules for using them:

- Behaviour is defined by `docs/contracts.md`, `docs/contracts-auth.md` and the frozen plan
  referenced in `AGENTS.md`; a report never overrides them.
- Decide every finding explicitly (fix, keep with rationale, or defer) and record the
  disposition where the project already tracks it: `docs/verification.md` for review
  follow-ups, `docs/decisions.md` for decisions that change behaviour.
- Do not weaken a lint rule, test or accessibility invariant because a report suggests it.
  Explain the disagreement instead.
- Keep reports as written. Corrections belong in the disposition, not in an edited report.

Current reports (2026-11-18):

- [`go-code-review.md`](go-code-review.md) — Go backend code smells and portability.
- [`web-code-review.md`](web-code-review.md) — React/TypeScript and UI quality.
- [`security-review.md`](security-review.md) — auth, session and deployment risk.
- [`final-review.md`](final-review.md) — independent post-implementation verification.
- [`web-copy-proposals.md`](web-copy-proposals.md) — UI microcopy introduced by fixes.

Dispositions for earlier reviews are recorded in [`../verification.md`](../verification.md).
