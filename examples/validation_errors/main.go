// Example: validation_errors demonstrates inspecting and serializing ValidationErrors.
// Run: go run examples/validation_errors/main.go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/twoojoo/goschema/schema"
)

type CreateUserRequest struct {
	_ any `schema:"additionalProperties=false"`

	Name  string `json:"name"  schema:"required,minLength=2,maxLength=50"`
	Email string `json:"email" schema:"required,format=email"`
	Age   int    `json:"age"   schema:"minimum=0,maximum=120"`
}

func main() {
	// ── 1. Constraint violations ──────────────────────────────────────────────
	bad := CreateUserRequest{Name: "A", Email: "not-an-email", Age: -1}
	err := schema.Validate(bad)

	fmt.Println("=== Constraint Violations ===")
	if ve, ok := err.(schema.ValidationErrors); ok {
		for _, e := range ve {
			fmt.Printf("  field=%-10s  message=%s\n", e.Field, e.Message)
		}

		// Errors can be serialized to JSON as-is.
		out, _ := json.MarshalIndent(ve, "", "  ")
		fmt.Println("\n=== As JSON ===")
		fmt.Println(string(out))

		// Check for a specific field.
		fmt.Printf("\nhas 'email' error: %v\n", ve.Has("email"))
	}

	// ── 2. JSON parsing errors wrapped as ValidationErrors ────────────────────
	fmt.Println("\n=== JSON Parse Errors ===")

	// Type mismatch
	_, err = schema.ParseJSON[CreateUserRequest]([]byte(`{"name":"Alice","email":"a@b.com","age":"bad"}`))
	if ve, ok := err.(schema.ValidationErrors); ok {
		fmt.Printf("  type mismatch: field=%s  message=%s\n", ve[0].Field, ve[0].Message)
	}

	// Unknown field (additionalProperties=false)
	_, err = schema.ParseJSON[CreateUserRequest]([]byte(`{"name":"Alice","email":"a@b.com","age":30,"extra":"x"}`))
	if ve, ok := err.(schema.ValidationErrors); ok {
		fmt.Printf("  unknown field: field=%s  message=%s\n", ve[0].Field, ve[0].Message)
	}

	// Malformed JSON
	_, err = schema.ParseJSON[CreateUserRequest]([]byte(`{name: "Alice"}`))
	if ve, ok := err.(schema.ValidationErrors); ok {
		fmt.Printf("  syntax error:  error=%s\n", ve[0].Message)
	}
}
