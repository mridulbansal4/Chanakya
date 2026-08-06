package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DocumentBlob is a content-addressed source document held in the system of
// record. Keeping the bytes here - rather than a filesystem path - is what lets
// an audit pack hand over the EXACT document that produced an obligation, years
// later, from the same single file.
type DocumentBlob struct {
	SHA256     string
	Bytes      []byte
	Filename   string
	Size       int
	PageCount  int
	UploadedAt string
}

// PutDocumentBlob stores a document by its content address. Re-uploading identical
// bytes is a no-op: the sha256 already identifies the content, so there is
// nothing to update and no second copy to make.
func (s *Store) PutDocumentBlob(ctx context.Context, b DocumentBlob) error {
	if b.SHA256 == "" {
		return errors.New("put document blob: empty sha256")
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO document_blob (sha256, bytes, filename, size, page_count)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(sha256) DO NOTHING`,
		b.SHA256, b.Bytes, b.Filename, b.Size, b.PageCount,
	); err != nil {
		return fmt.Errorf("put document blob %q: %w", b.SHA256, err)
	}
	return nil
}

// GetDocumentBlob returns a stored document by content address.
func (s *Store) GetDocumentBlob(ctx context.Context, sha string) (DocumentBlob, error) {
	var b DocumentBlob
	err := s.db.QueryRowContext(ctx, `
		SELECT sha256, bytes, filename, size, page_count, uploaded_at
		FROM document_blob WHERE sha256 = ?`, sha).
		Scan(&b.SHA256, &b.Bytes, &b.Filename, &b.Size, &b.PageCount, &b.UploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DocumentBlob{}, fmt.Errorf("document blob %q: %w", sha, ErrNotFound)
	}
	if err != nil {
		return DocumentBlob{}, fmt.Errorf("get document blob %q: %w", sha, err)
	}
	return b, nil
}

// HasDocumentBlob reports whether a document with this content address is stored.
func (s *Store) HasDocumentBlob(ctx context.Context, sha string) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx,
		`SELECT 1 FROM document_blob WHERE sha256 = ?`, sha).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check document blob %q: %w", sha, err)
	}
	return true, nil
}
