# Screenshots

Every image here is captured from the running application against a live backend: none is mocked
or hand-edited. The root `README.md` references these exact filenames.

## Regenerating

```bash
.\dev.ps1                          # backend :8080 + web :3000
py scripts/capture_screenshots.py  # 1440x900, device_scale_factor 2
```

The script pre-seeds `localStorage` to dismiss the welcome modal and per-route banners, so the
captures show the product rather than its onboarding.

| File | Screen | Route |
|---|---|---|
| `overview.png` | Overview: KPI strip, "Needs attention", chapter hierarchy | `/` |
| `overview-graph.png` | Overview: clause→obligation provenance graph | `/` → Graph |
| `ingest.png` | Regulatory Intake: upload, accepted kinds, intake history | `/ingest` |
| `review.png` | Review Queue: obligations awaiting Ed25519 sign-off | `/review` |
| `workflows.png` | Workflows: task DAG with named owners, all DRAFT | `/workflows` |
| `evidence.png` | Evidence coverage matrix + draft remediation tickets | `/evidence` |
| `blast-radius.png` | Blast Radius: direct and semantic propagation | `/amendments` |
| `policy.png` | Policy: signed obligation compiled to Rego | `/policy` |
| `audit.png` | Compliance lineage graph: six columns, clause to policy | `/audit` |
| `feed.png` | Regulator Feed: schema-validated, with provenance | `/feed` |
| `register.png` | Obligation Register: provenance and confidence | `/register` |
| `regulatory-feed.png` | Regulatory corpus: how each circular arrived | `/regulatory-feed` |
| `company.png` | Company profile: document vault, integrations | `/company` |

## Stale

`sim-inbox.png`, `sim-blast.png`, `sim-approval.png` and `sim-audit.png` document the hardcoded
amendment simulation (`lib/amendment-sim.ts`, `components/amendment/steps.tsx`), which was deleted
when `/regulatory-feed` was rewritten on real data. They are no longer referenced and can be removed.
