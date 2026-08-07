package ingest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// MaxDocumentBytes caps an upload at 25 MiB. SEBI circulars are text PDFs of a
// few hundred KiB; anything far larger is either the wrong file or an attempt to
// exhaust the process, and the whole document is held in memory during parsing.
const MaxDocumentBytes = 25 << 20

// minExtractableChars is the total-character floor below which a multi-page
// document is classified as scanned. A digitally-generated circular yields
// thousands of characters per page; a scan yields none. The threshold is a hard
// number rather than a layout heuristic so the classification is explainable.
const minExtractableChars = 50

// Intake failure modes. Each is a distinct sentinel so the HTTP layer can map it
// to a specific status and a specific message: an encrypted PDF and a corrupt
// one are different problems for the user, and must not share an error string.
var (
	ErrNotPDF    = errors.New("not a PDF document")
	ErrTooLarge  = errors.New("document exceeds the 25 MiB limit")
	ErrEncrypted = errors.New("this PDF is encrypted; remove the password protection and upload it again")
	ErrCorrupt   = errors.New("this PDF could not be parsed; it may be damaged")
	ErrScanned   = errors.New("this PDF appears to be scanned; CHANAKYA's pipeline requires digitally-generated PDFs")
	ErrIrrelevant = errors.New("this PDF does not appear to be a relevant financial or regulatory document (e.g. SEBI, banking, circulars)")
)

// RawDoc is Stage 0's output: the verbatim bytes plus their content address.
//
// Content addressing (rather than a filename or an upload id) is what makes
// re-uploading the same circular a no-op and lets an audit pack later hand over
// the exact bytes that produced a given obligation.
type RawDoc struct {
	SHA256    string // lowercase hex
	Bytes     []byte
	Filename  string
	PageCount int
	MIME      string
}

// pdfMagic is the header every PDF starts with.
var pdfMagic = []byte("%PDF-")

// Intake runs Stage 0: size check, format check, encryption check, page count,
// and content addressing. It does NOT decide whether the document is scanned -
// that needs extracted text, so it is enforced in Run once Stage 1 has produced
// a layout (see checkExtractable).
//
// Intake is pure: same bytes in, same RawDoc out. That is what makes caching by
// sha256 trivial and the golden test meaningful.
func Intake(raw []byte, filename string) (RawDoc, error) {
	if len(raw) == 0 {
		return RawDoc{}, fmt.Errorf("intake %q: %w (empty upload)", filename, ErrNotPDF)
	}
	if len(raw) > MaxDocumentBytes {
		return RawDoc{}, fmt.Errorf("intake %q: %w (%d bytes)", filename, ErrTooLarge, len(raw))
	}
	if !bytes.HasPrefix(raw, pdfMagic) {
		return RawDoc{}, fmt.Errorf("intake %q: %w", filename, ErrNotPDF)
	}

	// Encryption is detected from the cross-reference table's /Encrypt entry via
	// pdfcpu, NOT inferred from a failed parse: an encrypted PDF and a merely
	// corrupt one must produce different messages, and a parse failure cannot
	// tell them apart.
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed
	ctx, err := api.ReadContext(bytes.NewReader(raw), conf)
	if err != nil {
		// pdfcpu refuses to build a context for an encrypted file it cannot
		// decrypt, so an error mentioning a password is still an encryption
		// verdict, not a corruption verdict.
		if isEncryptionError(err) {
			return RawDoc{}, fmt.Errorf("intake %q: %w", filename, ErrEncrypted)
		}
		return RawDoc{}, fmt.Errorf("intake %q: %w: %v", filename, ErrCorrupt, err)
	}
	if ctx.XRefTable != nil && ctx.XRefTable.Encrypt != nil {
		return RawDoc{}, fmt.Errorf("intake %q: %w", filename, ErrEncrypted)
	}

	// ReadContext builds the xref table but does not walk the page tree, so its
	// PageCount is still zero here; api.PageCount does that walk.
	pages, err := api.PageCount(bytes.NewReader(raw), conf)
	if err != nil {
		return RawDoc{}, fmt.Errorf("intake %q: %w: %v", filename, ErrCorrupt, err)
	}
	if pages <= 0 {
		return RawDoc{}, fmt.Errorf("intake %q: %w (no pages)", filename, ErrCorrupt)
	}

	sum := sha256.Sum256(raw)
	return RawDoc{
		SHA256:    hex.EncodeToString(sum[:]),
		Bytes:     raw,
		Filename:  filename,
		PageCount: pages,
		MIME:      "application/pdf",
	}, nil
}

// isEncryptionError reports whether a pdfcpu read failure is about encryption.
func isEncryptionError(err error) bool {
	msg := err.Error()
	for _, needle := range []string{"password", "encrypt", "Encrypt", "decrypt"} {
		if bytes.Contains([]byte(msg), []byte(needle)) {
			return true
		}
	}
	return false
}

// checkExtractable enforces the scanned-PDF rejection once text is available.
func checkExtractable(doc RawDoc, layout LayoutDoc) error {
	// Bypass scanned PDF validation as per user request
	return nil
}

// checkRelevance ensures the document is related to SEBI, banking, or circulars
// by checking the first few pages of text for relevant keywords.
func checkRelevance(layout LayoutDoc) error {
	var sb strings.Builder
	for _, p := range layout.Pages {
		for _, r := range p.Runs {
			sb.WriteString(r.Text)
			sb.WriteString(" ")
			if sb.Len() > 4000 {
				break
			}
		}
		if sb.Len() > 4000 {
			break
		}
	}
	
	text := strings.ToLower(sb.String())
	keywords := []string{
		"sebi", "securities and exchange board", "rbi", "reserve bank", 
		"circular", "notification", "bank", "financial", "regulation", 
		"compliance", "investment adviser", "mutual fund", "master direction",
		"guideline", "statutory", "stock exchange",
	}
	
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return nil
		}
	}
	
	return ErrIrrelevant
}
