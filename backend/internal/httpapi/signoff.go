package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"chanakya/internal/domain"
	"chanakya/internal/signoff"
	"chanakya/internal/store"
)

const minJustificationLen = 20

// reviewQueue: GET /api/review-queue?as_of=
func (h *handlers) reviewQueue(w http.ResponseWriter, r *http.Request) {
	asOf, ok := parseAsOf(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid as_of")
		return
	}
	items, err := h.store.ReviewQueue(r.Context(), asOf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load review queue")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":       domain.RFC3339UTC(asOf),
		"count":       len(items),
		"obligations": items,
	})
}

type correctionInput struct {
	DeonticType *string         `json:"deontic_type"`
	Condition   *string         `json:"condition"`
	Deadline    *string         `json:"deadline"`
	Threshold   json.RawMessage `json:"threshold"`
}

type signoffInput struct {
	ObligationID  string           `json:"obligation_id"`
	Action        string           `json:"action"` // approve | reject
	SignedBy      string           `json:"signed_by"`
	Justification string           `json:"justification"`
	Corrections   *correctionInput `json:"corrections"`
}

// postSignoff: POST /api/signoff — the human sign-off. On approve it (optionally
// applies corrections, then) Ed25519-signs the canonical obligation and records
// the signature + mandatory justification; on reject it records the decision.
// This is the ONLY path that can move an obligation to approved.
func (h *handlers) postSignoff(w http.ResponseWriter, r *http.Request) {
	var in signoffInput
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	in.ObligationID = strings.TrimSpace(in.ObligationID)
	in.SignedBy = strings.TrimSpace(in.SignedBy)
	in.Justification = strings.TrimSpace(in.Justification)

	if in.ObligationID == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	if in.Action != "approve" && in.Action != "reject" {
		writeError(w, http.StatusBadRequest, "action must be approve or reject")
		return
	}
	if in.SignedBy == "" {
		writeError(w, http.StatusBadRequest, "signed_by is required")
		return
	}
	// Friction is a feature: a substantive typed justification is mandatory.
	if len(in.Justification) < minJustificationLen {
		writeError(w, http.StatusBadRequest, "justification is required (min 20 characters)")
		return
	}

	ctx := r.Context()
	if _, err := h.store.GetObligationDomain(ctx, in.ObligationID); err != nil {
		if notFound(err) {
			writeError(w, http.StatusNotFound, "obligation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load obligation")
		return
	}

	now := domain.RFC3339UTC(time.Now())
	rec := store.SignoffRecord{
		ID:            "so:" + in.ObligationID,
		ObligationID:  in.ObligationID,
		Action:        in.Action,
		SignedBy:      in.SignedBy,
		Justification: in.Justification,
	}

	if in.Action == "reject" {
		ob, _ := h.store.GetObligationDomain(ctx, in.ObligationID)
		hash, err := signoff.HashHex(ob)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to hash obligation")
			return
		}
		rec.ObligationHash = hash
		// A sign-off becomes a world-time fact when it is made (now), not
		// retroactively at the clause's issue date — so as-of a date before the
		// sign-off, the obligation reconstructs as unsigned.
		if err := h.store.UpsertSignoff(ctx, rec, now, now); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to record sign-off")
			return
		}
		if err := h.store.SetObligationStatus(ctx, in.ObligationID, domain.StatusRejected); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update status")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"signoff": rec, "verified": false})
		return
	}

	// approve — apply any corrections first, so the signature covers the final content.
	if in.Corrections != nil {
		corr := store.ObligationCorrection{
			DeonticType: in.Corrections.DeonticType,
			Condition:   in.Corrections.Condition,
			Deadline:    in.Corrections.Deadline,
			Threshold:   in.Corrections.Threshold,
		}
		if !corr.Empty() {
			if err := h.store.ApplyObligationCorrection(ctx, in.ObligationID, corr); err != nil {
				writeError(w, http.StatusBadRequest, "invalid correction: "+err.Error())
				return
			}
		}
	}

	ob, err := h.store.GetObligationDomain(ctx, in.ObligationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reload obligation")
		return
	}
	hash, sig, err := h.signer.Sign(ob)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign obligation")
		return
	}
	rec.ObligationHash = hash
	rec.Signature = sig
	rec.PublicKey = h.signer.PublicKeyB64()
	if err := h.store.UpsertSignoff(ctx, rec, now, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record sign-off")
		return
	}
	if err := h.store.SetObligationStatus(ctx, in.ObligationID, domain.StatusApproved); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"signoff": rec, "verified": true})
}

// getSignoff: GET /api/signoff?obligation_id= — returns the sign-off record and
// a live verification of its signature against the CURRENT obligation content.
func (h *handlers) getSignoff(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("obligation_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "obligation_id is required")
		return
	}
	rec, found, err := h.store.GetSignoff(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sign-off")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "no sign-off for this obligation")
		return
	}

	verification := map[string]any{"valid": false, "reason": "not an approval"}
	if rec.Action == "approve" && rec.Signature != "" {
		ob, err := h.store.GetObligationDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load obligation")
			return
		}
		valid, reason, verr := signoff.Verify(rec.PublicKey, ob, rec.Signature)
		if verr != nil {
			reason = verr.Error()
		}
		currentHash, _ := signoff.HashHex(ob)
		verification = map[string]any{
			"valid":        valid,
			"reason":       reason,
			"signed_hash":  rec.ObligationHash,
			"current_hash": currentHash,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"signoff":      rec,
		"verification": verification,
	})
}
