package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// FeedVersion is the schema version of the machine-readable regulator feed.
const FeedVersion = "1.0"

// FeedSignoff is the sign-off provenance embedded in the feed.
type FeedSignoff struct {
	Action         string `json:"action"`
	SignedBy       string `json:"signed_by"`
	ObligationHash string `json:"obligation_hash"`
	Signature      string `json:"signature,omitempty"`
	PublicKey      string `json:"public_key,omitempty"`
}

// FeedProvenance is the causal provenance for one obligation.
type FeedProvenance struct {
	SourceClauseRef     string       `json:"source_clause_ref"`
	SourceSentence      string       `json:"source_sentence"`
	ExtractorConfidence float64      `json:"extractor_confidence"`
	Signoff             *FeedSignoff `json:"signoff,omitempty"`
}

// FeedObligation is one obligation in the feed, with full provenance.
type FeedObligation struct {
	ID          string          `json:"id"`
	ClauseRef   string          `json:"clause_ref"`
	Bearer      string          `json:"bearer"`
	DeonticType string          `json:"deontic_type"`
	Condition   string          `json:"condition,omitempty"`
	Threshold   json.RawMessage `json:"threshold"`
	Deadline    string          `json:"deadline,omitempty"`
	Status      string          `json:"status"`
	ValidFrom   string          `json:"valid_from"`
	Provenance  FeedProvenance  `json:"provenance"`
}

// RegulatorFeed is the versioned SupTech feed payload.
type RegulatorFeed struct {
	FeedVersion   string `json:"feed_version"`
	Source        string `json:"source"`
	Regulator     string `json:"regulator"`
	GeneratedAsOf string `json:"generated_as_of"`
	Circular      struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		IssuedOn string `json:"issued_on"`
	} `json:"circular"`
	Obligations []FeedObligation `json:"obligations"`
}

// RegulatorFeed builds the machine-readable feed of obligations in force as-of
// a date, each with causal provenance (source sentence + extractor confidence)
// and, where signed, the Ed25519 sign-off. Read-only; provenance = "SupTech".
func (s *Store) RegulatorFeed(ctx context.Context, circularID string, asOf time.Time) (RegulatorFeed, error) {
	at := domain.RFC3339UTC(asOf)
	feed := RegulatorFeed{
		FeedVersion: FeedVersion, Source: "CHANAKYA SupTech feed", Regulator: "SEBI",
		GeneratedAsOf: at, Obligations: []FeedObligation{},
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, issued_on FROM circular WHERE id = ? AND tx_to IS NULL`, circularID).
		Scan(&feed.Circular.ID, &feed.Circular.Title, &feed.Circular.IssuedOn)
	if err != nil {
		if err == sql.ErrNoRows {
			return RegulatorFeed{}, ErrNotFound
		}
		return RegulatorFeed{}, fmt.Errorf("feed circular: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT o.id, c.clause_ref, o.bearer, o.deontic_type, COALESCE(o.condition,''),
		       o.threshold_json, COALESCE(o.deadline,''), o.status, o.valid_from,
		       o.source_clause_ref, o.source_sentence, o.confidence,
		       so.action, so.signed_by, so.obligation_hash,
		       COALESCE(so.signature,''), COALESCE(so.public_key,'')
		FROM obligation o
		JOIN clause c ON c.id = o.clause_id
		LEFT JOIN signoff so ON so.obligation_id = o.id AND so.tx_to IS NULL
		     AND so.valid_from <= ? AND (so.valid_to IS NULL OR so.valid_to > ?)
		WHERE c.circular_id = ?
		  AND o.valid_from <= ? AND (o.valid_to IS NULL OR o.valid_to > ?) AND o.tx_to IS NULL
		ORDER BY c.ordinal, o.deontic_type`,
		at, at, circularID, at, at)
	if err != nil {
		return RegulatorFeed{}, fmt.Errorf("feed obligations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			fo                                   FeedObligation
			threshold                            string
			soAction, soBy, soHash, soSig, soPub sql.NullString
		)
		if err := rows.Scan(
			&fo.ID, &fo.ClauseRef, &fo.Bearer, &fo.DeonticType, &fo.Condition,
			&threshold, &fo.Deadline, &fo.Status, &fo.ValidFrom,
			&fo.Provenance.SourceClauseRef, &fo.Provenance.SourceSentence, &fo.Provenance.ExtractorConfidence,
			&soAction, &soBy, &soHash, &soSig, &soPub,
		); err != nil {
			return RegulatorFeed{}, fmt.Errorf("scan feed obligation: %w", err)
		}
		if threshold == "" {
			threshold = "{}"
		}
		fo.Threshold = json.RawMessage(threshold)
		if soAction.Valid && soAction.String == "approve" {
			fo.Provenance.Signoff = &FeedSignoff{
				Action: soAction.String, SignedBy: soBy.String, ObligationHash: soHash.String,
				Signature: soSig.String, PublicKey: soPub.String,
			}
		}
		feed.Obligations = append(feed.Obligations, fo)
	}
	if err := rows.Err(); err != nil {
		return RegulatorFeed{}, fmt.Errorf("iterate feed obligations: %w", err)
	}
	return feed, nil
}
