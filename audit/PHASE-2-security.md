# Phase 2 — Security (OWASP 2021 + extras)

## Status of prior-audit criticals
- **C-1 (Rego injection → compliance tamper): REMEDIATED & VERIFIED.** `policy/compile.go` now emits every obligation value via `regoString` (JSON-encoded literal) or `regoComment` (sanitized), builds messages from `sprintf` args, and `validatePrepares` rejects any un-parseable module. Re-ran the exact end-to-end exploit against a fresh instance: the non-compliant firm now correctly reports `compliant=false`. Regression: `policy/injection_test.go`.
- **H-2 (injection → persistent enforcement DoS): REMEDIATED.** Same fix + parse-guard; `Compile` never returns an un-evaluable module.

## OWASP table

| Category | Applies | State | Finding |
|---|---|---|---|
| A01 Broken Access Control | Yes | **Absent** | S-1 (no authn/authz on any write, incl. sign-off + policy-stage→hard) |
| A02 Cryptographic Failures | Yes | Partial | Sound Ed25519 (`crypto/ed25519`, `crypto/rand`), signs content not status; but server-held key + no caller auth. |
| A03 Injection | Yes | **Handled** | SQL fully parameterized; Rego injection fixed (C-1). No XSS sinks (grep: no `dangerouslySetInnerHTML`/`innerHTML`/`eval`). |
| A04 Insecure Design | Yes | Partial | Sign-off trust model undermined by S-1. |
| A05 Misconfiguration | Yes | OK | CORS from env, `AllowCredentials:false`, body caps + `DisallowUnknownFields`, 30s timeout, generic error envelopes (no stack leak). |
| A06 Vulnerable Components | Yes | Low | 9 residual npm advisories, no non-breaking fix (Phase 10). |
| A07 Auth Failures | Yes | **Absent** | Subsumed by S-1. |
| A08 Data Integrity | Yes | Good | Content-signing breaks on any material-field tamper (verified `signoff.go:106-126` + tests). |
| A09 Logging | Yes | OK-ish | chi request logging; no secrets/PII logged. No audit log of *who* staged enforcement (ties to S-1). |
| A10 SSRF | Partial | OK | Only outbound is Anthropic (fixed URL). Embedded OPA build does **not** expose `http.send` (verified last pass) → Rego injection never escalated to SSRF. |

### S-1 — No authentication or authorization on any endpoint
- **File:** `backend/internal/httpapi/httpapi.go:74-123` (no auth middleware); `signoff.go:55` (`postSignoff`, `signed_by` free text); `policy.go:134` (`setPolicyStage`→`hard`)
- **Severity:** HIGH · **Confidence:** CONFIRMED · **Effort:** M
- **Attack scenario:** any client reaching the port can (a) forge an approving Ed25519 sign-off under any name — `signoff.go:52` documents this as *"the ONLY path that can move an obligation to approved"* — (b) promote a policy to blocking `hard` enforcement, (c) apply arbitrary field corrections. The regulator feed then republishes the forged sign-off as provenance (`store/feed.go:118`).
- **Exploitability:** Trivial (no auth). Was the delivery vector for the now-fixed C-1.
- **Fix:** authenticate `/api` writes; derive `signed_by` from the principal; authorize `policy/stage`→`hard`.

### S-2 — Rate limiter / RealIP trusts client-settable headers
- **File:** `backend/internal/httpapi/httpapi.go:77` (`middleware.RealIP`) + `:99` (`httprate.LimitByIP`)
- **Severity:** MEDIUM · **Confidence:** LIKELY (unverified: not reproduced; depends on deployment topology) · **Effort:** S
- `middleware.RealIP` overwrites `RemoteAddr` from `X-Forwarded-For`/`X-Real-IP`. With no proxy stripping those, an attacker rotates the header to get a fresh 240/min bucket per spoofed IP (rate-limit bypass) and to poison request logs. Only use `RealIP` behind a trusted proxy that sets those headers; otherwise key the limiter on the real socket.

### S-3 — LLM prompt-injection surface (real-extractor path)
- **File:** `backend/internal/llm/anthropic.go:60-66` (system prompt) → `compiler.go` validation
- **Severity:** MEDIUM · **Confidence:** LIKELY (offline extractor is default; Anthropic path not executed — no key) · **Effort:** M
- Clause text is attacker-influenceable in principle (a malicious "circular"). Model output is DATA-validated against the strict schema and the mandatory verbatim-citation check (`compiler.go:161`), which strongly constrains injection. Residual: `threshold.metric` is schema-typed only as `string`; with C-1 fixed it can no longer alter policy code, but a hallucinated metric can still make a policy silently never-apply (correctness, fails open). Recommend constraining `threshold.metric` to an enum.

**Input validation:** JSON bodies use `io.LimitReader(1<<20)` + `DisallowUnknownFields` (`signoff.go:57`, `policy.go:232`, `amendments.go:75`); query filters validated against domain enums (`obligations.go:35-40`). Good coverage. No file uploads. Regexes (`offline.go:56-65`, `vec.go:24`) are anchored/simple — no ReDoS.
