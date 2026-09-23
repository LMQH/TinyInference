# Implementation plan: public model name mapping

1. **PRD and ADR** → Confirm strict replacement, one model, durable name, and admitted-request behavior in `docs/product/02-model-name-mapping.md` and ADR-0005.
2. **Contracts and migration** → Define the admin mutation, snapshot fields, dynamic public name, fenced singleton storage, backup grant, and schema version 7. Inspect contract hashes and migration path.
3. **Implementation** → Read the name at public request start, capture it for response/metadata, add the admin save endpoint and console form, and proxy only that exact endpoint.
4. **QA** → Use Docker/DMR and a real browser for the PRD acceptance cases. Record any unavailable environment gate without claiming runtime acceptance.
5. **Independent review** → A reviewer separate from implementation checks correctness and security/compliance against the PRD, ADR, and technical specification before release acceptance.
