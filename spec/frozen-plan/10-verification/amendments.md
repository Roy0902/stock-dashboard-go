# Amendment Log: stock-watchlist

Records approved changes to locked acceptance files. The frozen plan is unchanged
(`planSha256` `7ed3f26e8c4015c81d96d1e35e4b777363759b41d1f9d315afc023ca2d955773`).
Each change below was approved by the user on 2026-11-17.

## AMD-001 (2026-11-17)

| Scenario | File | Defect in the locked test | Approved change |
| :-- | :-- | :-- | :-- |
| SCN-003 | `tests/api/stock-watchlist.spec.ts` | The outline examples shared one app, so `aapl` after `AAPL` returned 409. | Each example starts a fresh app. The expectation (uppercase, accepted) is unchanged. |
| SCN-019 | `tests/specs/stock-watchlist.spec.ts` | `getByText('0.00')` is a substring match and also matched `100.00` and `0.00%`. | `getByText('0.00', { exact: true })` compares the whole change value. |
| SCN-027 | `tests/specs/stock-watchlist.spec.ts` | `page.clock.fastForward('60s')` is not a valid argument. | `page.clock.fastForward(60_000)`. |
| SCN-053 | `tests/specs/stock-watchlist.spec.ts` | `not.toContainText('at ')` is an over-broad substring check, unrelated to a stack trace. It failed although the rendered text contains no `at `, and the cause was not isolated. | Stack-trace pattern on the main text: `not.toMatch(/^\s*at\s+\S+.*\(/m)`. |

Hashes:

| File | Before (AMD-001) | After (AMD-001) |
| :-- | :-- | :-- |
| `tests/api/stock-watchlist.spec.ts` | `ccd164c40f133135…` | `05b813cb6127683f…` |
| `tests/specs/stock-watchlist.spec.ts` | `e834f57b4becaa2a…` | `44b07431e7760db5…` |

Process: the manifest's previous test hashes and lock time were kept under `amendments`. The
manifest's `tests` field was reset, and `npm run ai:lock-tests -- stock-watchlist` recomputed the
hashes. `npm run ai:verify -- stock-watchlist` reports OK.

Not covered by this amendment: SCN-047 and SCN-050 remain blocked (`GAP-001`, `GAP-002`). Those
need a follow-up plan, not an amendment.

Note on SCN-053: the original failure was not isolated. The amended assertion passes against the
current app, but the reason the original failed is unexplained.
