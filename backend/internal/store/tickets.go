package store

import (
	"context"
	"fmt"
	"time"

	"chanakya/internal/domain"
)

// TicketView is the API shape for a draft remediation ticket.
type TicketView struct {
	ID           string `json:"id"`
	ObligationID string `json:"obligation_id"`
	ClauseRef    string `json:"clause_ref"`
	Title        string `json:"title"`
	Detail       string `json:"detail"`
	Owner        string `json:"owner"`
	Deadline     string `json:"deadline"`
	Citation     string `json:"citation"`
	State        string `json:"state"`
	ValidFrom    string `json:"valid_from"`
}

// UpsertTicket inserts or updates a draft ticket by id.
func (s *Store) UpsertTicket(ctx context.Context, t domain.Ticket) error {
	if !t.State.Valid() {
		return fmt.Errorf("ticket %q: invalid state %q", t.ID, t.State)
	}
	const q = `
		INSERT INTO ticket (id, obligation_id, clause_ref, title, detail, owner,
		                    deadline, citation, state, valid_from, valid_to, tx_from, tx_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			obligation_id=excluded.obligation_id, clause_ref=excluded.clause_ref,
			title=excluded.title, detail=excluded.detail, owner=excluded.owner,
			deadline=excluded.deadline, citation=excluded.citation, state=excluded.state,
			valid_from=excluded.valid_from, valid_to=excluded.valid_to,
			tx_from=excluded.tx_from, tx_to=excluded.tx_to`
	if _, err := s.db.ExecContext(ctx, q,
		t.ID, t.ObligationID, t.ClauseRef, t.Title, nullStr(t.Detail), t.Owner,
		nullStr(t.Deadline), t.Citation, string(t.State),
		t.ValidFrom, nullStr(t.ValidTo), t.TxFrom, nullStr(t.TxTo),
	); err != nil {
		return fmt.Errorf("upsert ticket %q: %w", t.ID, err)
	}
	return nil
}

// GenerateDraftTickets drafts (never files) one ticket per current gap. It is
// idempotent: ticket ids are deterministic per obligation. Returns the number
// of draft tickets in force after generation.
func (s *Store) GenerateDraftTickets(ctx context.Context, txNow time.Time) (int, error) {
	em, err := s.EvidenceMap(ctx, txNow)
	if err != nil {
		return 0, fmt.Errorf("evidence map for tickets: %w", err)
	}
	tx := domain.RFC3339UTC(txNow)
	var drafted int
	for _, oe := range em.Obligations {
		if oe.Satisfied {
			continue
		}
		t := domain.Ticket{
			ID:           "tkt:" + oe.ID,
			ObligationID: oe.ID,
			ClauseRef:    oe.ClauseRef,
			Title:        fmt.Sprintf("Evidence gap: %s obligation on clause %s", oe.Deontic, oe.ClauseRef),
			Detail:       oe.GapReason + ". Establish a control and map a read-only evidence source.",
			Owner:        "Compliance Officer",
			Deadline:     oe.Deadline,
			Citation:     oe.SourceSentence,
			State:        domain.TicketDraft, // NEVER filed automatically
			Temporal:     domain.Temporal{ValidFrom: oe.ValidFrom, TxFrom: tx},
		}
		if err := s.UpsertTicket(ctx, t); err != nil {
			return 0, err
		}
		drafted++
	}
	return drafted, nil
}

// ListTickets returns draft tickets in force as-of a date.
func (s *Store) ListTickets(ctx context.Context, asOf time.Time) ([]TicketView, error) {
	at := domain.RFC3339UTC(asOf)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, obligation_id, clause_ref, title, COALESCE(detail, ''), owner,
		       COALESCE(deadline, ''), citation, state, valid_from
		FROM ticket
		WHERE valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND tx_to IS NULL
		ORDER BY clause_ref`, at, at)
	if err != nil {
		return nil, fmt.Errorf("list tickets: %w", err)
	}
	defer rows.Close()

	out := []TicketView{}
	for rows.Next() {
		var t TicketView
		if err := rows.Scan(&t.ID, &t.ObligationID, &t.ClauseRef, &t.Title, &t.Detail,
			&t.Owner, &t.Deadline, &t.Citation, &t.State, &t.ValidFrom); err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tickets: %w", err)
	}
	return out, nil
}
