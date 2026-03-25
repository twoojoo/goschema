// Example: strict_json demonstrates strict JSON parsing that rejects unknown fields.
// Run: go run examples/strict_json/main.go
package main

import (
	"fmt"

	"github.com/twoojoo/goschema/schema"
)

// APIRequest uses additionalProperties=false to reject any unknown JSON key.
type APIRequest struct {
	_ any `schema:"additionalProperties=false"`

	Action  string `json:"action"  schema:"required,enum=create|update|delete"`
	Payload string `json:"payload" schema:"required"`
}

func main() {
	// ── Valid input ───────────────────────────────────────────────────────────
	validJSON := []byte(`{"action":"create","payload":"hello"}`)
	req, err := schema.ParseJSON[APIRequest](validJSON)
	if err != nil {
		panic(err)
	}
	fmt.Println("Valid:", req.Action, req.Payload)

	// ── Input with an unknown field ───────────────────────────────────────────
	withExtra := []byte(`{"action":"create","payload":"hello","extra":"boom"}`)
	_, err = schema.ParseJSON[APIRequest](withExtra)
	if ve, ok := err.(schema.ValidationErrors); ok {
		fmt.Println("Rejected extra field:", ve[0].Message)
	}

	// ── Input with invalid enum value ─────────────────────────────────────────
	badEnum := []byte(`{"action":"destroy","payload":"hello"}`)
	_, err = schema.ParseJSON[APIRequest](badEnum)
	if ve, ok := err.(schema.ValidationErrors); ok {
		fmt.Println("Rejected enum:", ve[0].Message)
	}

	// ── Validate-JSON-only (discard result) ────────────────────────────────────
	if err := schema.ValidateJSON[APIRequest](validJSON); err != nil {
		panic(err)
	}
	fmt.Println("ValidateJSON: ok")
}
