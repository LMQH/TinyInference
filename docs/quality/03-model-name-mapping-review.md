# Model-name mapping verification record

- **Date:** 2026-09-23
- **Feature acceptance:** Accepted by the user on 2026-09-23 with the completed candidate QA and independent reviews. The user directed us to stop further QA. This does not record a production deployment or change the `unresolved` production manifest.
- **Scope:** Public model-name mapping and its isolated candidate path. Existing unrelated working-tree changes were preserved.

## Completed evidence

- The final candidate used API image `sha256:72fb8a125ca845e815c17d63256e10f75b52d641cc3ab222d4ebdc56283dbae6` with the pinned DMR/model and loopback-only candidate web/API ports. Production web/API/controller were paused during candidate QA and restored with their original container/image identities. After candidate shutdown, all five production containers were healthy and DMR was unloaded. No live migration, image push, or remote deployment occurred.
- The disposable candidate database reached schema 7. The console and authenticated model catalog initially showed the original name. Independent QA saved a custom name in a real browser, verified the catalog had exactly one entry, old names returned `400 unsupported_value` with `param=model`, invalid/stale writes were rejected without changing the name, and reset restored the original name.
- Two requests admitted before a name change (one active, one FIFO waiting) completed with their admitted old name in the response and request metadata. A custom name remained the sole catalog/console name after a candidate API restart. Independent QA verified keyboard save, validation messages, and the old-name warning in the browser.
- QA found that DMR attaches completion usage to the terminal choice. The gateway now validates that form and emits a separate usage chunk. The final candidate passed non-stream and stream `/v1/completions` with reasoning both on and off: HTTP 200, correct public name, one terminal usage chunk, `usage:null` on ordinary SSE chunks, no error event, and `[DONE]`. Chat and strict old-name rejection also passed. The focused Go package test passed in the pinned Go Docker image.
- The final candidate produced a schema-7 backup and completed an isolated restore with the same custom public model name and content-safe token/schema assertions. Candidate backup and database paths remained separate from production. The public inference port rejected admin routes and missing authentication; the candidate admin port was not published.
- Independent code and security reviews found no remaining definite implementation blocker after fixes to pause/restore ordering, backup symlink checks, restore-name comparison, and completion SSE normalization. Their review does not constitute release approval.
- Observed catalog latency was 2.68–4.65 ms after first access (first access 14.26–14.94 ms); chat stream first byte was 23.8–60.9 ms in two samples. There is no MVP performance threshold.

## Privacy evidence and limit

- On the final candidate, two controlled privacy runs observed normal, reasoning, and tool-call canaries in their intended responses. Application logs, DMR request history/logs, and repository files had zero canary matches during requests, immediately after requests, and after a same-version DMR restart. A candidate backup and isolated restored database were also decoded and scanned with zero matches.
- Independent browser QA checked the same Chrome candidate tab's DOM text/HTML, URL, Console, localStorage, sessionStorage, IndexedDB, and Cache Storage. The first run had zero matches during requests, immediately after, and after DMR restart, but the tab had opened after the request began. The second run had an observed pre-request baseline, during-request scan, and immediate-after scan, all zero matches. Its post-restart browser scan did not complete because the DevTools Console prompt became unavailable. These observations must not be reported as a complete three-timepoint browser proof for the second run.
- At the user's direction, no further QA was run. The live deployment still uses the previous images and schema, without model-name mapping. The production compatibility manifest and privacy gate remain `unresolved`; any production rollout is a separate operation.

## Operational limit

ADR-0006's candidate guard is a startup check, not a cross-project lock. During a candidate window the single operator must not externally unpause the recorded production containers or run production Compose startup. `ops/candidate/check.py` verifies their paused state and identities; the start and stop scripts check quiescence and DMR unload before switching authority.
