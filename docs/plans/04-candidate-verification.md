# Plan: isolated candidate verification

1. **Contract** → Accept ADR-0006 and define manifest states, isolation, evidence binding, and recovery in the technical specification. Verify the production `verified` gate remains strict.
2. **Implementation** → Add candidate-only startup branch, isolated Compose override, manifest preparation and preflight. Verify generated topology and hashes before touching services.
3. **Candidate QA** → Prove old deployment recovery, stop its API/controller, run migration 7 only on the disposable database, then obtain Docker/DMR, browser, privacy, backup/restore, and failure evidence. Restore the old deployment on candidate failure.
4. **Review and promotion** → Independent QA, code, and security reviewers assess evidence and changes. Only then promote the exact build set and perform the ordinary local rollout gates.
