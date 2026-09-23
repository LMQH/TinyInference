# PRD addendum: configurable public model name

- **Status:** Approved by the product owner's 2026-09-23 request and clarification.
- **Scope:** One public name for the existing single model; no model selection or additional runtime.

## User outcome

The operator can set a custom model name in the web console and give that name to OpenAI-compatible clients. The console shows the current public name and the fixed original name. The configured name survives service restart and backup/restore.

## Requirements

1. The initial public name is `openbmb/MiniCPM5-2B-Q4_K_M`. The operator can replace it with one custom name or restore that original name in the console.
2. The change takes effect when the save operation succeeds. `GET /v1/models` lists exactly the current public name. New chat and completion requests accept only that name; the previous name fails explicitly. This strict replacement was confirmed by the product owner.
3. A request admitted before a name change retains its accepted name in its eventual response and request metadata, including when it was waiting in the queue. A name change never interrupts inference or changes the DMR model identity.
4. The UI clearly states that saving a new name makes the old name invalid for new clients. Invalid names and failed saves show an error without claiming success.
5. The feature remains limited to the existing private LAN/VPN console and authenticated inference API. It does not add another model, alias list, fallback name, or public network endpoint.

## Acceptance

- From the console, set a custom name; observe it in the console and sole `/v1/models` entry. Requests with the new name succeed and the old name returns `unsupported_value` for `model`.
- Observe the selected name in non-streaming and streaming responses and request metadata. Requests already admitted before a change keep their original accepted name.
- Restart the application and perform backup/restore proof; the configured name remains. Set the name back to the original and confirm strict replacement again.
- Invalid names, stale concurrent edits, and database failures do not publish a partial or false success. Browser and Docker/DMR runtime evidence are required before release acceptance.
