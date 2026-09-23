# QA addendum: public model name mapping

Use Docker Compose, the approved DMR runtime, and a real browser. Record versions and keep request bodies and API credentials out of evidence.

1. On a clean migration-7 database, inspect the console and authenticated `/v1/models`: both show the original public name.
2. Save a custom name in the console. Confirm exactly one catalog entry, new-name acceptance for chat and completions, and old-name `400 unsupported_value` with `param=model`.
3. Exercise streaming and non-streaming calls across a name change. Responses and request metadata retain each request's admitted name; internal DMR ref and model count remain fixed.
4. Save the original name again and confirm strict replacement. Reject malformed, duplicate-field, long, non-ASCII, and stale concurrent changes; failed writes must preserve the prior name.
5. Restart API and inspect the name. Confirm backup includes the singleton and restore reproduces it. Verify the admin mutation is reachable only through the exact console route and never on port 8888 or directly from LAN to port 8889.
6. Inspect the browser form, error handling, keyboard use, and visible warning that previous names stop working. Report defects to the implementation owner; QA does not repair implementation.
7. Record the observed latency of the catalog and generation admission before and after a name edit; this MVP has no performance pass threshold.
