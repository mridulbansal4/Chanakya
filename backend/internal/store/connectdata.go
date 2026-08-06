package store

import (
	"context"
	"fmt"

	"chanakya/internal/connect"
)

// ConnectorData exposes the seeded enterprise graph to the mock connectors.
//
// It is a READ-ONLY projection by construction: every method below is a SELECT.
// The connectors read the firm's own seeded data rather than reaching out to a
// network, which is what makes "zero network calls in the default path" a fact
// about the code rather than a claim about configuration.
type ConnectorData struct {
	store *Store
}

// NewConnectorData wires the store into the connector layer.
func NewConnectorData(s *Store) *ConnectorData { return &ConnectorData{store: s} }

// Communications returns email or meeting records.
func (c *ConnectorData) Communications(ctx context.Context, kind string, limit int) ([]connect.Record, error) {
	rows, err := c.store.db.QueryContext(ctx, `
		SELECT id, COALESCE(subject,''), COALESCE(thread_id,''), COALESCE(sent_on,''), participants
		FROM communication
		WHERE kind = ? AND tx_to IS NULL
		ORDER BY sent_on DESC, id
		LIMIT ?`, kind, limit)
	if err != nil {
		return nil, fmt.Errorf("read %s communications: %w", kind, err)
	}
	defer rows.Close()

	var out []connect.Record
	for rows.Next() {
		var id, subject, thread, sent, participants string
		if err := rows.Scan(&id, &subject, &thread, &sent, &participants); err != nil {
			return nil, fmt.Errorf("scan communication: %w", err)
		}
		out = append(out, connect.Record{
			ID: id, Title: subject, Timestamp: sent,
			Fields: map[string]string{"thread_id": thread, "participants": participants},
		})
	}
	return out, rows.Err()
}

// CalendarEvents returns scheduled events.
func (c *ConnectorData) CalendarEvents(ctx context.Context, limit int) ([]connect.Record, error) {
	rows, err := c.store.db.QueryContext(ctx, `
		SELECT id, title, starts_at, attendees, COALESCE(kind,'')
		FROM calendar_event WHERE tx_to IS NULL
		ORDER BY starts_at DESC, id LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read calendar events: %w", err)
	}
	defer rows.Close()

	var out []connect.Record
	for rows.Next() {
		var id, title, startsAt, attendees, kind string
		if err := rows.Scan(&id, &title, &startsAt, &attendees, &kind); err != nil {
			return nil, fmt.Errorf("scan calendar event: %w", err)
		}
		out = append(out, connect.Record{
			ID: id, Title: title, Timestamp: startsAt,
			Fields: map[string]string{"attendees": attendees, "kind": kind},
		})
	}
	return out, rows.Err()
}

// Documents returns firm documents.
func (c *ConnectorData) Documents(ctx context.Context, limit int) ([]connect.Record, error) {
	rows, err := c.store.db.QueryContext(ctx, `
		SELECT id, title, kind, version, COALESCE(owner_dept,''), COALESCE(last_reviewed,'')
		FROM document WHERE tx_to IS NULL ORDER BY title LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read documents: %w", err)
	}
	defer rows.Close()

	var out []connect.Record
	for rows.Next() {
		var id, title, kind, dept, reviewed string
		var version int
		if err := rows.Scan(&id, &title, &kind, &version, &dept, &reviewed); err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		out = append(out, connect.Record{
			ID: id, Title: title, Timestamp: reviewed,
			Fields: map[string]string{
				"kind": kind, "version": fmt.Sprint(version), "owner_dept": dept,
			},
		})
	}
	return out, rows.Err()
}

// Registers returns the firm's registers.
func (c *ConnectorData) Registers(ctx context.Context, limit int) ([]connect.Record, error) {
	rows, err := c.store.db.QueryContext(ctx, `
		SELECT id, kind, row_count, COALESCE(source_system,''), COALESCE(last_updated,'')
		FROM register WHERE tx_to IS NULL ORDER BY kind LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read registers: %w", err)
	}
	defer rows.Close()

	var out []connect.Record
	for rows.Next() {
		var id, kind, source, updated string
		var count int
		if err := rows.Scan(&id, &kind, &count, &source, &updated); err != nil {
			return nil, fmt.Errorf("scan register: %w", err)
		}
		out = append(out, connect.Record{
			ID: id, Title: kind + " register", Timestamp: updated,
			Fields: map[string]string{"rows": fmt.Sprint(count), "source_system": source},
		})
	}
	return out, rows.Err()
}

// Employees returns the firm's people.
func (c *ConnectorData) Employees(ctx context.Context, limit int) ([]connect.Record, error) {
	rows, err := c.store.db.QueryContext(ctx, `
		SELECT id, name, role, COALESCE(email,''), COALESCE(department_id,'')
		FROM employee WHERE tx_to IS NULL ORDER BY name LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("read employees: %w", err)
	}
	defer rows.Close()

	var out []connect.Record
	for rows.Next() {
		var id, name, role, email, dept string
		if err := rows.Scan(&id, &name, &role, &email, &dept); err != nil {
			return nil, fmt.Errorf("scan employee: %w", err)
		}
		out = append(out, connect.Record{
			ID: id, Title: name, Actor: role,
			Fields: map[string]string{"email": email, "department": dept},
		})
	}
	return out, rows.Err()
}
