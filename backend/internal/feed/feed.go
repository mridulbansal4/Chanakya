// Package feed holds the JSON schema for the machine-readable regulator
// (SupTech) feed and a validator. The feed is emitted read-only and validated
// against this schema before it leaves the building, so a downstream regulator
// can rely on its shape.
package feed

import (
	"bytes"
	_ "embed"
	"fmt"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var SchemaJSON []byte

// Validator validates feed payloads against the embedded schema.
type Validator struct {
	schema *jsonschema.Schema
}

// NewValidator compiles the embedded feed schema.
func NewValidator() (*Validator, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(SchemaJSON))
	if err != nil {
		return nil, fmt.Errorf("parse feed schema: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("chanakya:regulator-feed", doc); err != nil {
		return nil, fmt.Errorf("add feed schema: %w", err)
	}
	sch, err := c.Compile("chanakya:regulator-feed")
	if err != nil {
		return nil, fmt.Errorf("compile feed schema: %w", err)
	}
	return &Validator{schema: sch}, nil
}

// Validate checks a marshaled feed payload against the schema.
func (v *Validator) Validate(payload []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("feed is not valid JSON: %w", err)
	}
	if err := v.schema.Validate(inst); err != nil {
		return fmt.Errorf("feed failed schema validation: %w", err)
	}
	return nil
}
