package schema_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/twoojoo/goschema/schema"
)

// ── time.Time support ────────────────────────────────────────────────────────

type EventWithTime struct {
	Name      string    `json:"name"       schema:"required"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func TestTimeTime_EmitsDateTimeFormat(t *testing.T) {
	js, err := schema.ToJSONSchema[EventWithTime]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	props := js["properties"].(map[string]any)

	createdAt := props["created_at"].(map[string]any)
	if createdAt["type"] != "string" {
		t.Errorf("expected type 'string' for time.Time, got %v", createdAt["type"])
	}
	if createdAt["format"] != "date-time" {
		t.Errorf("expected format 'date-time' for time.Time, got %v", createdAt["format"])
	}
}

func TestTimeTime_ParseJSON(t *testing.T) {
	data := []byte(`{"name":"Deploy","created_at":"2024-03-01T10:00:00Z","updated_at":"2024-03-01T11:00:00Z"}`)
	ev, err := schema.ParseJSON[EventWithTime](data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Name != "Deploy" {
		t.Errorf("expected name=Deploy, got %s", ev.Name)
	}
	if ev.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// ── Custom format (RegisterFormat) ───────────────────────────────────────────

type WithPhone struct {
	Phone string `json:"phone" schema:"format=phone"`
}

func TestRegisterFormat_ValidValue(t *testing.T) {
	schema.RegisterFormat("phone", func(v string) bool {
		return len(v) >= 7 && len(v) <= 15
	})

	if err := schema.Validate(WithPhone{Phone: "555123456"}); err != nil {
		t.Errorf("valid phone should pass: %v", err)
	}
}

func TestRegisterFormat_InvalidValue(t *testing.T) {
	schema.RegisterFormat("phone", func(v string) bool {
		return len(v) >= 7 && len(v) <= 15
	})

	err := schema.Validate(WithPhone{Phone: "123"})
	ve, ok := err.(schema.ValidationErrors)
	if !ok || !ve.Has("phone") {
		t.Error("short phone should fail custom format validation")
	}
}

func TestRegisterFormat_EmittedInSchema(t *testing.T) {
	schema.RegisterFormat("phone", func(v string) bool { return true })

	js, err := schema.ToJSONSchema[WithPhone]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	props := js["properties"].(map[string]any)
	phoneSchema := props["phone"].(map[string]any)
	if phoneSchema["format"] != "phone" {
		t.Errorf("expected format 'phone' in schema, got %v", phoneSchema["format"])
	}
}

// ── x- extension fields ───────────────────────────────────────────────────────

type ProductWithExtensions struct {
	_ any `schema:"title=Product,x-internal=true,x-logo=https://example.com/logo.png"`

	Name  string  `json:"name"  schema:"required,x-example=Widget"`
	Price float64 `json:"price" schema:"minimum=0,x-currency=USD"`
}

func TestExtensions_FieldLevel(t *testing.T) {
	js, err := schema.ToJSONSchema[ProductWithExtensions]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	props := js["properties"].(map[string]any)
	nameSchema := props["name"].(map[string]any)
	if nameSchema["x-example"] != "Widget" {
		t.Errorf("expected x-example=Widget, got %v", nameSchema["x-example"])
	}
	priceSchema := props["price"].(map[string]any)
	if priceSchema["x-currency"] != "USD" {
		t.Errorf("expected x-currency=USD, got %v", priceSchema["x-currency"])
	}
}

func TestExtensions_StructLevel(t *testing.T) {
	js, err := schema.ToJSONSchema[ProductWithExtensions]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if js["x-internal"] != "true" {
		t.Errorf("expected x-internal=true at root, got %v", js["x-internal"])
	}
	if js["x-logo"] != "https://example.com/logo.png" {
		t.Errorf("expected x-logo at root, got %v", js["x-logo"])
	}
}

func TestExtensions_ValidJSON(t *testing.T) {
	js, err := schema.ToJSONSchema[ProductWithExtensions]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Extensions must produce valid JSON
	_, err = json.Marshal(js)
	if err != nil {
		t.Fatalf("schema with extensions must be valid JSON: %v", err)
	}
}

// ── Schema cache correctness ───────────────────────────────────────────────────

type CacheStructA struct {
	Tags []string `json:"tags" schema:"items:minLength=5"`
}

type CacheStructB struct {
	Tags []string `json:"tags"` // no items constraint
}

func TestSchemaCache_DoesNotLeakConstraintsAcrossTypes(t *testing.T) {
	// First build A (items:minLength=5)
	if err := schema.Validate(CacheStructA{Tags: []string{"hello"}}); err != nil {
		t.Fatalf("CacheStructA valid: %v", err)
	}

	// B should NOT inherit A's items:minLength=5
	if err := schema.Validate(CacheStructB{Tags: []string{"x"}}); err != nil {
		t.Errorf("CacheStructB should not be affected by A's items constraint: %v", err)
	}
}
