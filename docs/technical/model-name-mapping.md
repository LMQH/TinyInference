# Technical specification: public model name mapping

This refines [the approved PRD addendum](../product/02-model-name-mapping.md) and [ADR-0005](../adr/0005-public-model-name-mapping.md).

## Name and state

- A singleton PostgreSQL row holds `public_model_id`; migration initializes it to `openbmb/MiniCPM5-2B-Q4_K_M` and never changes the immutable DMR ref.
- A valid value matches `^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$` (ASCII, case-sensitive, 1–128 bytes). Whitespace, empty values, controls, and Unicode are rejected. Saving the original value resets the mapping.
- The `mini_api` role reads a restricted view and invokes a `SECURITY DEFINER` function requiring the current authority holder and epoch. The function compares the caller's expected name, updates the singleton, and publishes a snapshot event in the same transaction. A stale expected name returns conflict; a failed transaction leaves the prior name active.
- Durable request admission checks the current name while holding the authority lock and model-name row lock. A name change cannot commit between that check and admission; an admission using a replaced name returns `400 unsupported_value`.

## HTTP contract

- `GET /admin/v1/snapshot` adds `model.public_model_id` and `model.default_public_model_id`. Both are metadata, not DMR identity.
- `POST /admin/v1/model/name` accepts exactly one UTF-8 JSON object with `public_model_id` and `expected_public_model_id`; rejects unknown/duplicate fields, invalid names, query strings, and bodies over 1 KiB. The web proxy preserves the caller's `Content-Type` so the API can reject non-JSON browser submissions. Success returns the saved `public_model_id` and new `snapshot_version`. Stale expected name returns 409; storage/authority failure returns 503. The endpoint exists only on the Compose admin listener and the exact web proxy allowlist.
- `GET /v1/models` lists exactly the current `public_model_id`. Both generation endpoints require exactly that name and reject the previous name with `400 unsupported_value`, `param=model`. Non-streaming `model` and every SSE chunk's `model` equal the name captured for that request. Request metadata records the same name.
- A public read that cannot obtain the current name fails with 503; it never falls back to a stale or default value. A name edit does not cancel admitted requests. DMR calls always use the fixed `AI_MODEL_NAME`.

## Rollout and verification

Add migration 7, update schema-version gates and backup grants, then ship the API, proxy, and console contracts together. Verify default behavior, change/reset, old-name rejection, in-flight response consistency, restart and restore persistence, failed writes, proxy boundaries, and real browser use. No external network endpoint is added.
