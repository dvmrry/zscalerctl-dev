# Adversarial Review Artifacts

High-risk changes must commit an approved review artifact in this directory.
The gate `scripts/verify-adversarial-review.sh` requires one whenever CLI
surface, machine-contract, schema, redaction/projection, resource wiring,
generated-artifact, golden fixture, skill-sync, workflow, or review-process
surfaces change.

Use one file per change. Keep tenant data and secrets out of these artifacts.
Each artifact must include:

- `# Builder Handoff`
- `# Adversarial Review`
- `Fresh-context reviewer:`
- `Verdict: approve` or `Verdict: approve with nits`

Do not use an artifact to paper over a failed review. A `request changes`
verdict means the change is not ready to accept.
