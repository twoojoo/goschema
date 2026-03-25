// Example: full_featured demonstrates every JSON Schema constraint supported by goschema.
// Run: go run examples/full_featured/main.go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/twoojoo/goschema/schema"
)

// ── Nested types ──────────────────────────────────────────────────────────────

// Address is a nested struct — validation errors use dot-notation paths.
type Address struct {
	Street string `json:"street" schema:"required,minLength=1,maxLength=200"`
	City   string `json:"city"   schema:"required,minLength=1"`
	ZIP    string `json:"zip"    schema:"pattern=^[0-9]{5}$"`
}

// Score demonstrates numeric constraints.
type Score struct {
	Value   float64 `json:"value"    schema:"minimum=0,maximum=100,multipleOf=0.5"`
	Rounded int     `json:"rounded"  schema:"minimum=0,maximum=100"`
}

// ── Full-featured root struct ─────────────────────────────────────────────────

// Profile is a full-featured struct covering every constraint goschema supports.
type Profile struct {
	// ── Struct-level metadata & object rules ─────────────────────────────────
	_ any `schema:"title=UserProfile,description=Complete user profile,additionalProperties=false,dependentRequired:billing_id=billing_email|billing_address"`

	// ── String constraints ────────────────────────────────────────────────────
	ID       string `json:"id"        schema:"required,format=uuid"`
	Username string `json:"username"  schema:"required,minLength=3,maxLength=30,pattern=^[a-zA-Z0-9_]+$"`
	Email    string `json:"email"     schema:"required,format=email"`
	Website  string `json:"website"   schema:"format=uri"`
	Birth    string `json:"birth"     schema:"format=date"`
	LastSeen string `json:"last_seen" schema:"format=date-time"`
	IP       string `json:"ip"        schema:"format=ipv4"`

	// ── Enum + const + default ────────────────────────────────────────────────
	Role   string `json:"role"   schema:"required,enum=admin|editor|viewer,default=viewer"`
	Status string `json:"status" schema:"const=active"`
	Lang   string `json:"lang"   schema:"enum=en|fr|de|it|es,default=en"`

	// ── Numeric constraints ───────────────────────────────────────────────────
	Age      int     `json:"age"       schema:"minimum=0,maximum=150"`
	Salary   float64 `json:"salary"    schema:"minimum=0,exclusiveMinimum=0,multipleOf=0.01"`
	Priority int     `json:"priority"  schema:"minimum=1,maximum=10,default=5"`

	// ── Boolean ───────────────────────────────────────────────────────────────
	Verified  bool `json:"verified"   schema:"default=false"`
	NewsLetter bool `json:"newsletter" schema:"default=false"`

	// ── Nullable pointer (nil is valid) ───────────────────────────────────────
	Bio     *string `json:"bio"      schema:"nullable,maxLength=500"`
	Website2 *string `json:"website2" schema:"nullable,format=uri"`

	// ── Array / slice constraints ─────────────────────────────────────────────
	Tags        []string `json:"tags"         schema:"minItems=1,maxItems=20,uniqueItems"`
	Permissions []string `json:"permissions"  schema:"uniqueItems,items:minLength=1"`
	Scores      []int    `json:"scores"       schema:"minItems=0,maxItems=100"`

	// ── Map constraints ───────────────────────────────────────────────────────
	Metadata map[string]string `json:"metadata"   schema:"minProperties=0,maxProperties=50"`

	// ── Nested struct ─────────────────────────────────────────────────────────
	Address Address `json:"address"`
	Score   Score   `json:"score"`

	// ── Composition: anyOf / oneOf / allOf / not ─────────────────────────────
	// ContactMethod: must be either a phone-like or email-like string
	ContactMethod string `json:"contact_method" schema:"anyOf=minLength=10;pattern=^[0-9+\\-() ]+$"`

	// Nickname: must match exactly ONE — either short (3-8 chars) or starts with 'usr_'
	// Use mutually exclusive patterns so truly only one matches at a time.
	Nickname string `json:"nickname" schema:"oneOf=maxLength=8;minLength=9"`

	// Code: must satisfy all of these sub-schemas
	Code string `json:"code" schema:"allOf=minLength=4;pattern=^[A-Z]"`

	// Notes: must NOT be an empty/very-short string (demonstrate 'not')
	Notes string `json:"notes" schema:"not=maxLength=0"`

	// ── Billing dependency (dependentRequired via struct tag) ─────────────────
	BillingID      string `json:"billing_id"`
	BillingEmail   string `json:"billing_email"   schema:"format=email"`
	BillingAddress string `json:"billing_address"`
}

func main() {
	// ── 1. Generate and print JSON Schema ─────────────────────────────────────
	fmt.Println("════════════════════════════════════════")
	fmt.Println(" JSON Schema output")
	fmt.Println("════════════════════════════════════════")

	schemaBytes, err := schema.ToJSONSchemaIndent[Profile]("", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(schemaBytes))

	// ── 2. Parse valid JSON and validate ──────────────────────────────────────
	fmt.Println("════════════════════════════════════════")
	fmt.Println(" Parse + validate (valid input)")
	fmt.Println("════════════════════════════════════════")

	validJSON := []byte(`{
		"id":             "550e8400-e29b-41d4-a716-446655440000",
		"username":       "alice_90",
		"email":          "alice@example.com",
		"birth":          "1990-06-15",
		"last_seen":      "2024-03-01T10:00:00Z",
		"ip":             "192.168.1.1",
		"role":           "editor",
		"status":         "active",
		"age":            34,
		"salary":         72000.50,
		"tags":           ["go", "api", "validation"],
		"permissions":    ["read", "write"],
		"scores":         [80, 95, 72],
		"metadata":       {"team": "backend"},
		"address":        {"street": "123 Main St", "city": "Rome", "zip": "00100"},
		"score":          {"value": 87.5, "rounded": 88},
		"contact_method": "555-123-4567",
		"nickname":       "alice123",
		"code":           "ABCD",
		"notes":          "all good"
	}`)

	p, err := schema.ParseJSON[Profile](validJSON)
	if err != nil {
		fmt.Printf("Unexpected error: %v\n", err)
	} else {
		fmt.Printf("Parsed OK: username=%s role=%s priority=%d lang=%s\n",
			p.Username, p.Role, p.Priority, p.Lang)
	}

	// ── 3. Validate an invalid instance ───────────────────────────────────────
	fmt.Println("\n════════════════════════════════════════")
	fmt.Println(" Validate (invalid input)")
	fmt.Println("════════════════════════════════════════")

	bad := Profile{
		ID:       "not-a-uuid",                 // format=uuid fails
		Username: "x",                          // minLength=3 fails
		Email:    "not-an-email",               // format=email fails
		Role:     "superuser",                  // enum fails
		Age:      -5,                           // minimum=0 fails
		Tags:     []string{},                   // minItems=1 fails
		Status:   "active",
		Address:  Address{Street: "", City: ""}, // required fields missing
	}

	verr := schema.Validate(bad)
	if ve, ok := verr.(schema.ValidationErrors); ok {
		errJSON, _ := json.MarshalIndent(ve, "", "  ")
		fmt.Println("Validation errors:")
		fmt.Println(string(errJSON))
	}
}
