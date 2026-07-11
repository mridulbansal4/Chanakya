// Package signoff implements the cryptographic human sign-off: a canonical,
// deterministic serialization of an obligation's CONTENT, an Ed25519 signature
// over it, and verification. Signing the content (not the review status) means
// a signature stays valid when status flips to "approved", but breaks the
// instant any material field is tampered with.
//
// NOTE: for the MVP the signing key is a server-held key acting as the
// reviewer's key; in production the signature would be produced client-side
// with the reviewer's own key / an HSM. The verification model is identical.
package signoff

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"chanakya/internal/domain"
)

// canonicalObligation fixes the field set + order that a signature attests to.
// encoding/json marshals struct fields in declaration order, giving a
// deterministic canonical form. Status is deliberately excluded.
type canonicalObligation struct {
	ID              string          `json:"id"`
	ClauseID        string          `json:"clause_id"`
	Bearer          string          `json:"bearer"`
	DeonticType     string          `json:"deontic_type"`
	Condition       string          `json:"condition"`
	Threshold       json.RawMessage `json:"threshold"`
	Deadline        string          `json:"deadline"`
	Penalty         string          `json:"penalty"`
	SourceClauseRef string          `json:"source_clause_ref"`
	SourceSentence  string          `json:"source_sentence"`
	ValidFrom       string          `json:"valid_from"`
}

// Canonical returns the deterministic bytes an obligation signature covers.
func Canonical(o domain.Obligation) ([]byte, error) {
	th := o.ThresholdJSON
	if th == "" {
		th = "{}"
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(th)); err != nil {
		return nil, fmt.Errorf("compact threshold: %w", err)
	}
	c := canonicalObligation{
		ID: o.ID, ClauseID: o.ClauseID, Bearer: o.Bearer,
		DeonticType: string(o.DeonticType), Condition: o.Condition,
		Threshold: json.RawMessage(compact.Bytes()), Deadline: o.Deadline,
		Penalty: o.Penalty, SourceClauseRef: o.SourceClauseRef,
		SourceSentence: o.SourceSentence, ValidFrom: o.ValidFrom,
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical obligation: %w", err)
	}
	return b, nil
}

// HashHex returns the sha256 hex of the canonical obligation (for display /
// tamper comparison).
func HashHex(o domain.Obligation) (string, error) {
	c, err := Canonical(o)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(c)
	return hex.EncodeToString(sum[:]), nil
}

// Signer holds the Ed25519 private key and signs obligations.
type Signer struct {
	priv ed25519.PrivateKey
}

// NewSigner wraps an Ed25519 private key.
func NewSigner(priv ed25519.PrivateKey) *Signer { return &Signer{priv: priv} }

// PublicKeyB64 returns the base64 public key.
func (s *Signer) PublicKeyB64() string {
	pub := s.priv.Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub)
}

// Sign returns the canonical hash (hex) and the base64 Ed25519 signature.
func (s *Signer) Sign(o domain.Obligation) (hashHex, sigB64 string, err error) {
	c, err := Canonical(o)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(c)
	sig := ed25519.Sign(s.priv, c)
	return hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString(sig), nil
}

// Verify checks a stored signature against the CURRENT obligation content. A
// mismatch (tampering, or a signature from a different obligation) returns
// (false, reason). The bool is the cryptographic result.
func Verify(pubB64 string, o domain.Obligation, sigB64 string) (bool, string, error) {
	pub, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		return false, "malformed public key", fmt.Errorf("decode public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return false, "wrong public key size", nil
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return false, "malformed signature", fmt.Errorf("decode signature: %w", err)
	}
	c, err := Canonical(o)
	if err != nil {
		return false, "canonicalization failed", err
	}
	if ed25519.Verify(ed25519.PublicKey(pub), c, sig) {
		return true, "signature valid for current obligation content", nil
	}
	return false, "signature does NOT match current obligation content (tampered or corrected since signing)", nil
}

// LoadOrCreateKey resolves the Ed25519 signing key: from CHANAKYA_SIGNING_KEY_B64
// (a base64 32-byte seed) if set, else from the seed file at path, else it
// generates a new key and persists the seed to path (0600). The seed file must
// be gitignored — it is the demo signing authority.
func LoadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	if env := os.Getenv("CHANAKYA_SIGNING_KEY_B64"); env != "" {
		seed, err := base64.StdEncoding.DecodeString(env)
		if err != nil {
			return nil, fmt.Errorf("decode CHANAKYA_SIGNING_KEY_B64: %w", err)
		}
		if len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("CHANAKYA_SIGNING_KEY_B64 must be a %d-byte seed", ed25519.SeedSize)
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if b, err := os.ReadFile(path); err == nil {
		seed, derr := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(b)))
		if derr != nil || len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("signing key file %q is corrupt", path)
		}
		return ed25519.NewKeyFromSeed(seed), nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read signing key %q: %w", path, err)
	}

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	seedB64 := base64.StdEncoding.EncodeToString(priv.Seed())
	if err := os.WriteFile(path, []byte(seedB64), 0o600); err != nil {
		return nil, fmt.Errorf("persist signing key %q: %w", path, err)
	}
	return priv, nil
}
