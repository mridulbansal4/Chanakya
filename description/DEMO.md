# CHANAKYA — 90-second demo script

**Setup (once):** `.\dev.ps1` (or `go run ./backend/cmd/api` + `npm run dev`).
The backend **self-seeds** the SEBI Investment Advisers Master Circular on first
run — no manual seed step. Open http://localhost:3000.

The story: *regulatory text → operational action*, with a human gate and a
cryptographic, bi-temporal audit trail. Nothing is enforced until a human signs.

| # | Time | Screen | Say / do |
|---|------|--------|----------|
| 1 | 0:00–0:12 | **Overview** (`/`) | "CHANAKYA compiled a SEBI circular into a live obligation graph — 10 obligations in force, gaps and pending sign-offs on the posture strip. The graph is the hero." Point at the teal **Backend online** pill: real Go backend, SQLite system-of-record. |
| 2 | 0:12–0:24 | **Register** (`/register`) | "Every obligation is typed and **cited**." Click clause **3.1** → the detail panel highlights the *exact source sentence*. Change the **as-of date** to `2024-01-01` → the register empties: "as-of *before* the circular issued, nothing was in force." Set it back to today. |
| 3 | 0:24–0:40 | **Blast Radius** (`/amendments`) | "What does amending a clause actually touch?" Pick **4.1 Disclosure of fees**, hit **Compute blast radius**. Watch the impact *propagate* clause → obligation → control → evidence. Call out the **semantic** hit: editing the fee clause lights up the **registration** control via the fee-threshold obligation — a cosine-diff catches it. |
| 4 | 0:40–0:52 | **Evidence & Gaps** (`/evidence`) | "We map obligations to *read-only* firm evidence and flag gaps." Clause **5.2** (client-notification) has no control → a **gap** → an auto-**drafted** remediation ticket (deadline P7D, cited). "We draft, we never file into a customer system." |
| 5 | 0:52–1:08 | **Review Queue** (`/review`) | "Nothing is trusted automatically." Click **Review & sign** on an obligation → the deliberate **multi-step modal**: review the source sentence, decide, type a **mandatory justification**, then **Sign with Ed25519**. The signature + hash appear, **VERIFIED ✓**. |
| 6 | 1:08–1:22 | **Policy** (`/policy`) | "A *signed* obligation compiles to deterministic **Rego**." Select **3.1** → show the generated policy. Evaluate the firm state → **Compliant** with an **OPA trace**. Flip enforcement to **hard** + an un-attested input → **Non-compliant, BLOCKED**. "Enforcement is staged audit → soft → hard, and only the deterministic engine enforces." |
| 7 | 1:22–1:30 | **Audit** (`/audit`) + **Feed** (`/feed`) | "Everything is bi-temporal." On **Audit**, set as-of `2024-06-01` → the obligation exists but **sign-off and policy are gone** (weren't signed yet). On **Feed**, show the versioned, **schema-validated** machine-readable feed with provenance + the Ed25519 sign-off — what a regulator's SupTech system consumes. |

**One-line close:** "From regulatory text to a signed, enforceable, auditable
policy — the LLM only ever proposed *data*; a human signed it; a deterministic
engine enforces it; and every claim is cited and reconstructable as-of any date."

## The safety model (the spine of the demo)

1. The LLM produces **DATA only**, validated against a strict JSON schema — never
   code, never enforcement.
2. Enforcement is done **only** by the deterministic OPA/Rego engine, and **only
   after** a human Ed25519-signs the obligation.
3. Evidence connectors are **read-only**.
4. Enforcement is **staged** audit → soft → hard — never hard-blocks before a
   sign-off exists.
5. Every obligation carries a **causal citation** (source clause id + exact
   sentence) or it is rejected before entering the graph.
