# Source specifications

These are the frozen inputs and extension specifications that the Go dashboard
implements. They are copied here so the repository is self-contained; the original
TypeScript prototype is not included.

| Path | Contents |
| :-- | :-- |
| `implementation-spec.md` | Append-only implementation spec, v1.6 (sections 1–15). |
| `unit-test-catalogue.md` | Unit-test catalogue for the original plan. |
| `frozen-plan/` | Original frozen plan, scenarios, freeze manifests and review material. |
| `user-auth/` | Authentication extension spec, including `user-auth.md` and its freeze manifest. |

Behaviour is defined by the frozen plan and `docs/contracts.md` / `docs/contracts-auth.md`.
The Go implementation lives at the repository root (`server/`, `web/`, `e2e/`).
