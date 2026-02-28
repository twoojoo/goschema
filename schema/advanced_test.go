package schema_test

import (
	"testing"

	"github.com/twoojoo/goschema/schema"
)

// ---- items ----

type List struct {
	Items []string `json:"items" schema:"items:minLength=5"`
}

func TestItems_Validation(t *testing.T) {
	s := List{Items: []string{"hello", "world", "hi"}}
	err := schema.Validate(s)
	ve, ok := err.(schema.ValidationErrors)
	if !ok || !ve.Has("items[2]") {
		t.Errorf("expected error foritems[2] ('hi' too short), got: %v", err)
	}
}

// ---- nullable ----

type NullableDoc struct {
	Name *string `json:"name" schema:"required,nullable"`
}

func TestNullable_Validation(t *testing.T) {
	s := NullableDoc{Name: nil}
	if err := schema.Validate(s); err != nil {
		t.Errorf("nil should be valid when nullable=true even if required: %v", err)
	}
}

// ---- composition (anyOf, oneOf, allOf, not) ----

type CompDoc struct {
	X string `json:"x" schema:"anyOf=minLength=5;pattern=^[0-9]+$"`
	Y string `json:"y" schema:"oneOf=minLength=5;pattern=^[0-9]+$"`
	Z string `json:"z" schema:"not=minLength=5"`
}

func TestAnyOf_Validation(t *testing.T) {
	// minLength=5 passes
	assertNoError(t, schema.Validate(CompDoc{X: "hello"}))
	// pattern=^[0-9]+ passes
	assertNoError(t, schema.Validate(CompDoc{X: "123"}))
	// neither fails
	ve := mustValidationErrors(t, schema.Validate(CompDoc{X: "hi"}))
	assertHasField(t, ve, "x")
}

func TestOneOf_Validation(t *testing.T) {
	// only one passes: OK
	assertNoError(t, schema.Validate(CompDoc{Y: "hello"}))
	assertNoError(t, schema.Validate(CompDoc{Y: "123"}))
	// both pass (e.g. "123456"): FAIL
	ve := mustValidationErrors(t, schema.Validate(CompDoc{Y: "12345"}))
	assertHasField(t, ve, "y")
}

func TestNot_Validation(t *testing.T) {
	// minLength=5 fails the 'not'
	ve := mustValidationErrors(t, schema.Validate(CompDoc{Z: "hello"}))
	assertHasField(t, ve, "z")
	// minLength=2 passes the 'not'
	assertNoError(t, schema.Validate(CompDoc{Z: "hi"}))
}

// ---- additionalProperties ----

type StrictStruct struct {
	_    any    `schema:"additionalProperties=false"`
	Name string `json:"name"`
}

func TestAdditionalProperties_Parse(t *testing.T) {
	data := []byte(`{"name":"Alice","extra":"field"}`)
	_, err := schema.Parse[StrictStruct](data)
	if err == nil {
		t.Fatal("expected error for extra field when additionalProperties=false")
	}
}

// ---- dependentRequired ----

type DepDoc struct {
	_         any    `schema:"dependentRequired:billing_id=credit_card|billing_addr"`
	BillingID string `json:"billing_id"`
	CC        string `json:"credit_card"`
	Addr      string `json:"billing_addr"`
}

func TestDependentRequired_Validation(t *testing.T) {
	// source absent: OK
	assertNoError(t, schema.Validate(DepDoc{}))
	// source present, dependents present: OK
	assertNoError(t, schema.Validate(DepDoc{BillingID: "123", CC: "visa", Addr: "123 St"}))
	// source present, dependents missing: FAIL
	err := schema.Validate(DepDoc{BillingID: "123"})
	if err == nil {
		t.Fatal("expected dependentRequired violation")
	}
}

// ---- ToJSONSchema emission ----

func TestToJSONSchema_Advanced(t *testing.T) {
	js, err := schema.ToJSONSchema[CompDoc]()
	assertNoError(t, err)

	x := js["properties"].(map[string]any)["x"].(map[string]any)
	if _, ok := x["anyOf"]; !ok {
		t.Error("expected anyOf in JSON Schema output for field x")
	}

	z := js["properties"].(map[string]any)["z"].(map[string]any)
	if _, ok := z["not"]; !ok {
		t.Error("expected not in JSON Schema output for field z")
	}
}

func TestToJSONSchema_Object(t *testing.T) {
	js, err := schema.ToJSONSchema[StrictStruct]()
	assertNoError(t, err)
	if js["additionalProperties"] != false {
		t.Errorf("expected additionalProperties:false, got %v", js["additionalProperties"])
	}

	js2, err := schema.ToJSONSchema[DepDoc]()
	assertNoError(t, err)
	if _, ok := js2["dependentRequired"]; !ok {
		t.Error("expected dependentRequired in JSON Schema output")
	}
}
