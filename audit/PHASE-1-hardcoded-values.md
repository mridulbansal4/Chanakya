# Phase 1 - Hardcoded Values

**Summary table**

| Category | Count | Highest severity |
|---|---|---|
| Secret | 0 | - |
| Environment | 2 (absolute paths in scripts) | LOW |
| Business constant | 4 | MEDIUM |
| Duplication | 2 | LOW |
| Design token | 1 | LOW |

**No secrets in code or git history.** `chanakya_signing.key` and `chanakya.db` are gitignored and confirmed never committed (`git log --all` empty). LLM key is env-only.

### HC-1 - Firm record-retention period hardcoded
- **File:** `backend/internal/store/policy.go:218` - `metrics["retention_period"] = 5`
- **Category:** Business Constant · **Severity:** MEDIUM · **Confidence:** CONFIRMED · **Effort:** XS
- The firm's actual retention period is baked into the suggested policy-evaluation input, making the retention *requirement* policy pass by default. Should live in the entity `meta_json` alongside `clients`/`annual_fees_inr`.

### HC-2 - Metric-key remap couples layers by literal
- **File:** `backend/internal/store/policy.go:210-214` (`annual_fees_inr` → `annual_fees`)
- **Category:** Business Constant / Duplication · **Severity:** MEDIUM · **Confidence:** CONFIRMED · **Effort:** S
- The fee policy gates on `input.metrics["annual_fees"]` (`llm/offline.go:174`) while the entity stores `annual_fees_inr`; the bridge is a string remap. Drift on either side makes the policy silently "not applicable → compliant" (fails open). Define shared metric-key constants.

### HC-3 - Review threshold duplicated across the stack
- **File:** `frontend/apps/web/lib/format.ts:73` (`REVIEW_THRESHOLD_PCT = 75`) vs `backend/internal/compiler/compiler.go:34` (`DefaultReviewThreshold = 0.75`)
- **Category:** Duplication · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** S
- Same business rule expressed as two independent literals on opposite sides of the API. If backend threshold changes, the UI copy silently lies. Document as a shared contract or surface via API.

### HC-4 - Justification minimum duplicated across the boundary
- **File:** `frontend/apps/web/components/signoff-modal.tsx:19` (`MIN_JUSTIFICATION = 20`) vs `backend/internal/httpapi/signoff.go:15` (`minJustificationLen = 20`)
- **Category:** Duplication · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** XS
- Independent constants for the same server-enforced rule; the server is authoritative, the client copy can drift.

### HC-5 - Hardcoded mock officer identity
- **File:** `frontend/apps/web/components/app-shell.tsx:21-25` (`OFFICER = { name:"Priya Menon", role:"Compliance Officer", firm:"Acme Investment Advisers" }`)
- **Category:** Business Constant · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** S
- The "logged-in user" is a hardcoded literal; ties to the absence of auth (Phase 2 H-1). Fine for a demo; must become the authenticated principal before real use.

### HC-6 - Fabricated "Propagation Time" metric
- **File:** `frontend/apps/web/app/page.tsx:72` (`value: "1.2s"`, tone `verified`)
- **Category:** Business Constant / Magic string · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** XS
- A fixed string presented in the KPI banner as a measured performance figure. Either compute it or label it illustrative.

### HC-7 - Absolute Windows paths in scripts
- **File:** `scripts/generate_documents.py:20` (`OUT = r"C:\Projects\SEBI\CHANAKYA\Documents"`), `scripts/capture_screenshots.py:5` (`OUT = r"C:\Projects\SEBI\CHANAKYA\docs\screenshots"`)
- **Category:** Environment · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** XS
- Breaks for any other checkout path. Derive from `__file__` / a repo-root arg.

### HC-8 - Design values outside the token layer
- **File:** `frontend/apps/web/app/page.tsx` (`#08090E`, `#11131C`, `#1A1D2C`, `bg-blue-600`, `text-amber-400`, …), `components/app-shell.tsx` (`#090A0F`, `bg-blue-600/30`)
- **Category:** Design Token · **Severity:** LOW · **Confidence:** CONFIRMED · **Effort:** S
- The Overview and shell use raw hex + raw Tailwind palette classes, while the rest of the app uses semantic tokens (`bg-surface`, `text-foreground`, `border-line`, `text-ok/warn/risk`). Inconsistent theming source of truth (see Phase 6).

**Acceptable / not findings:** rate limit `240/min`, body cap `1<<20`, `defaultBlastThreshold=0.30`, embedding `Dim=256`, offline extractor confidence bases (`0.70/+0.15/+0.10/0.95`), fixture PAN/registration numbers (synthetic demo data, `ia_master_circular.json:13`), MITC refs/dates in `amendment-sim.ts` (scripted demo). These are named constants in their rightful homes or clearly-marked demo fixtures.

**Proposed target structure:** a backend `internal/constants` (or reuse `domain`) for metric keys + review threshold, surfaced to the client via a small `/api/config` payload so the two `75`/`20` pairs and the metric keys have one source; script paths derived from `__file__`.
