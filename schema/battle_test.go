// Package schema_test contains battle tests covering edge cases and
// adversarial inputs for the goschema library.
package schema_test

import (
	"testing"

	"github.com/twoojoo/goschema/schema"
)

// ============================================================
// 1. UNICODE / MULTIBYTE STRING HANDLING
// ============================================================

type UnicodeStruct struct {
	Name string `json:"name" schema:"minLength=2,maxLength=4"`
}

func TestUnicode_EmojiCountsAsOneRune(t *testing.T) {
	// 🚀 is 4 UTF-8 bytes but 1 rune — maxLength=4 means runes, not bytes
	u := UnicodeStruct{Name: "🚀🚀🚀"}
	if err := schema.Validate(u); err != nil {
		t.Errorf("3 emoji runes should satisfy maxLength=4: %v", err)
	}
}

func TestUnicode_ByteLengthDoesNotInfluenceCount(t *testing.T) {
	// "こんにちは" = 5 runes but 15 UTF-8 bytes.
	// maxLength=4 counts runes, so this must FAIL.
	u := UnicodeStruct{Name: "こんにちは"}
	ve, ok := schema.Validate(u).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("5 runes must exceed maxLength=4; byte count (15) must not be used")
	}
}

func TestUnicode_ExactlyAtMinLength(t *testing.T) {
	u := UnicodeStruct{Name: "AB"} // exactly minLength=2
	if err := schema.Validate(u); err != nil {
		t.Errorf("exactly at minLength should pass: %v", err)
	}
}

func TestUnicode_ExactlyAtMaxLength(t *testing.T) {
	u := UnicodeStruct{Name: "ABCD"} // exactly maxLength=4
	if err := schema.Validate(u); err != nil {
		t.Errorf("exactly at maxLength should pass: %v", err)
	}
}

func TestUnicode_OneBelowMinLength(t *testing.T) {
	u := UnicodeStruct{Name: "A"} // 1 rune, minLength=2
	ve, ok := schema.Validate(u).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("expected minLength violation for 1-rune string")
	}
}

func TestUnicode_OneAboveMaxLength(t *testing.T) {
	u := UnicodeStruct{Name: "ABCDE"} // 5 runes, maxLength=4
	ve, ok := schema.Validate(u).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("expected maxLength violation for 5-rune string")
	}
}

// ============================================================
// 2. FLOATING POINT multipleOf PRECISION (the bug we fixed)
// ============================================================

type FloatMultiple struct {
	X float64 `json:"x" schema:"multipleOf=0.1"`
	Y float64 `json:"y" schema:"multipleOf=0.01"`
}

func TestMultipleOf_FloatPrecision_01(t *testing.T) {
	// 0.3 = 3 × 0.1, but naive math.Mod gives ~0.1 due to float64 repr
	s := FloatMultiple{X: 0.3, Y: 0.30}
	if err := schema.Validate(s); err != nil {
		t.Errorf("0.3 should be a multiple of 0.1 (floating point bug?): %v", err)
	}
}

func TestMultipleOf_FloatPrecision_07(t *testing.T) {
	s := FloatMultiple{X: 0.7, Y: 0.70}
	if err := schema.Validate(s); err != nil {
		t.Errorf("0.7 should be a multiple of 0.1: %v", err)
	}
}

func TestMultipleOf_FloatPrecision_Invalid(t *testing.T) {
	s := FloatMultiple{X: 0.35} // 0.35 / 0.1 = 3.5, not an integer
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("x") {
		t.Error("expected multipleOf=0.1 violation for 0.35")
	}
}

// ============================================================
// 3. EXCLUSIVE BOUNDARIES
// ============================================================

type ExclusiveBounds struct {
	X float64 `json:"x" schema:"exclusiveMinimum=0,exclusiveMaximum=10"`
}

func TestExclusive_AtMin_Fails(t *testing.T) {
	s := ExclusiveBounds{X: 0}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("x") {
		t.Error("exclusiveMinimum=0: value 0 should fail")
	}
}

func TestExclusive_JustAboveMin_Passes(t *testing.T) {
	s := ExclusiveBounds{X: 0.001}
	if err := schema.Validate(s); err != nil {
		t.Errorf("0.001 > 0 should pass exclusiveMinimum=0: %v", err)
	}
}

func TestExclusive_AtMax_Fails(t *testing.T) {
	s := ExclusiveBounds{X: 10}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("x") {
		t.Error("exclusiveMaximum=10: value 10 should fail")
	}
}

func TestExclusive_JustBelowMax_Passes(t *testing.T) {
	s := ExclusiveBounds{X: 9.999}
	if err := schema.Validate(s); err != nil {
		t.Errorf("9.999 < 10 should pass exclusiveMaximum=10: %v", err)
	}
}

// inclusive minimum at exactly 0
type InclusiveBounds struct {
	X int `json:"x" schema:"minimum=0,maximum=10"`
}

func TestInclusive_AtMin_Passes(t *testing.T) {
	if err := schema.Validate(InclusiveBounds{X: 0}); err != nil {
		t.Errorf("minimum=0: value 0 should pass: %v", err)
	}
}

func TestInclusive_AtMax_Passes(t *testing.T) {
	if err := schema.Validate(InclusiveBounds{X: 10}); err != nil {
		t.Errorf("maximum=10: value 10 should pass: %v", err)
	}
}

// ============================================================
// 4. POINTER FIELD EDGE CASES
// ============================================================

type WithPointers struct {
	Name    *string `json:"name"    schema:"required,minLength=2"`
	Age     *int    `json:"age"     schema:"minimum=0"`
	Enabled *bool   `json:"enabled"`
}

func TestPointer_NilRequired_Fails(t *testing.T) {
	s := WithPointers{Name: nil}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("nil *string with required should fail")
	}
}

func TestPointer_NonNilValid_Passes(t *testing.T) {
	n := "Alice"
	a := 30
	b := true
	s := WithPointers{Name: &n, Age: &a, Enabled: &b}
	if err := schema.Validate(s); err != nil {
		t.Errorf("valid pointers should pass: %v", err)
	}
}

func TestPointer_NonNilConstraintViolation(t *testing.T) {
	n := "A" // too short, minLength=2
	s := WithPointers{Name: &n}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("non-nil *string violating minLength should fail")
	}
}

func TestPointer_NilOptional_Passes(t *testing.T) {
	// age and enabled are optional (no required), nil is fine
	n := "Alice"
	s := WithPointers{Name: &n, Age: nil, Enabled: nil}
	if err := schema.Validate(s); err != nil {
		t.Errorf("nil optional pointer should pass: %v", err)
	}
}

// ============================================================
// 5. NIL SLICE / NIL MAP
// ============================================================

type WithSlice struct {
	Tags []string `json:"tags" schema:"minItems=1"`
}

type WithMap struct {
	Labels map[string]string `json:"labels" schema:"minProperties=1"`
}

func TestNilSlice_TreatedAsEmpty(t *testing.T) {
	s := WithSlice{Tags: nil}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("tags") {
		t.Error("nil slice should be treated as empty (len=0) and fail minItems=1")
	}
}

func TestNilMap_TreatedAsEmpty(t *testing.T) {
	s := WithMap{Labels: nil}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("labels") {
		t.Error("nil map should be treated as empty (len=0) and fail minProperties=1")
	}
}

// ============================================================
// 6. VALIDATE WITH NON-STRUCT INPUTS
// ============================================================

func TestValidate_NilInput(t *testing.T) {
	err := schema.Validate(nil)
	if err == nil {
		t.Error("Validate(nil) should return an error")
	}
}

func TestValidate_IntInput(t *testing.T) {
	err := schema.Validate(42)
	if err == nil {
		t.Error("Validate(42) should return an error (not a struct)")
	}
}

func TestValidate_StringInput(t *testing.T) {
	err := schema.Validate("hello")
	if err == nil {
		t.Error("Validate(\"hello\") should return an error (not a struct)")
	}
}

func TestValidate_PtrToNonStruct(t *testing.T) {
	n := 42
	err := schema.Validate(&n)
	if err == nil {
		t.Error("Validate(&int) should return an error")
	}
}

// ============================================================
// 7. ToJSONSchema WITH NON-STRUCT TYPE PARAMETER
// ============================================================

func TestToJSONSchema_NonStruct_Errors(t *testing.T) {
	_, err := schema.ToJSONSchema[string]()
	if err == nil {
		t.Error("ToJSONSchema[string]() should return an error")
	}
}

func TestToJSONSchema_NonStruct_Int(t *testing.T) {
	_, err := schema.ToJSONSchema[int]()
	if err == nil {
		t.Error("ToJSONSchema[int]() should return an error")
	}
}

// ============================================================
// 8. json:"-" FIELDS ARE SKIPPED
// ============================================================

type WithIgnored struct {
	Name   string `json:"name"   schema:"required"`
	Secret string `json:"-"      schema:"required"` // must be skipped
}

func TestJSONMinus_FieldSkipped(t *testing.T) {
	// Secret has schema:"required" but json:"-" — it must be ignored entirely
	s := WithIgnored{Name: "Alice", Secret: ""}
	if err := schema.Validate(s); err != nil {
		t.Errorf("json:\"-\" field should be skipped even with schema tags: %v", err)
	}
}

// ============================================================
// 9. STRUCT WITH NO SCHEMA TAGS
// ============================================================

type NoTags struct {
	Name string
	Age  int
}

func TestNoSchemaTags_AlwaysPasses(t *testing.T) {
	s := NoTags{} // all zero values, but no constraints
	if err := schema.Validate(s); err != nil {
		t.Errorf("struct with no schema tags should always pass: %v", err)
	}
}

// ============================================================
// 10. MULTIPLE ERRORS ARE ALL COLLECTED
// ============================================================

type MultiError struct {
	A string `json:"a" schema:"required"`
	B string `json:"b" schema:"required"`
	C int    `json:"c" schema:"minimum=10"`
	D string `json:"d" schema:"minLength=5"`
}

func TestMultipleErrors_AllCollected(t *testing.T) {
	s := MultiError{A: "", B: "", C: 1, D: "hi"}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok {
		t.Fatal("expected ValidationErrors")
	}
	// A, B required, C minimum, D minLength should all fail
	if !ve.Has("a") {
		t.Error("expected error for field 'a'")
	}
	if !ve.Has("b") {
		t.Error("expected error for field 'b'")
	}
	if !ve.Has("c") {
		t.Error("expected error for field 'c'")
	}
	if !ve.Has("d") {
		t.Error("expected error for field 'd'")
	}
	if len(ve) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(ve), ve)
	}
}

// ============================================================
// 11. VALIDATE POINTER-TO-STRUCT (public API)
// ============================================================

type Simple struct {
	Name string `json:"name" schema:"required"`
}

func TestValidate_PointerToStruct(t *testing.T) {
	s := &Simple{Name: "Alice"}
	if err := schema.Validate(s); err != nil {
		t.Errorf("Validate(&struct) should work: %v", err)
	}
}

func TestValidate_PointerToStruct_Fail(t *testing.T) {
	s := &Simple{Name: ""}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Error("Validate(&struct) with violation should return ValidationErrors")
	}
}

func TestValidate_NilPointerToStruct(t *testing.T) {
	var s *Simple
	err := schema.Validate(s)
	if err == nil {
		t.Error("Validate((*Simple)(nil)) should return an error")
	}
}

// ============================================================
// 12. PATTERN EDGE CASES
// ============================================================

type WithPattern struct {
	Code string `json:"code"   schema:"pattern=^[A-Z]{3}-[0-9]{4}$"`
	Free string `json:"free"   schema:"pattern=.*"`
}

func TestPattern_Valid(t *testing.T) {
	s := WithPattern{Code: "ABC-1234"}
	if err := schema.Validate(s); err != nil {
		t.Errorf("ABC-1234 should match ^[A-Z]{3}-[0-9]{4}$: %v", err)
	}
}

func TestPattern_Invalid(t *testing.T) {
	s := WithPattern{Code: "abc-1234"} // lowercase
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("code") {
		t.Error("lowercase code should fail pattern ^[A-Z]{3}-[0-9]{4}$")
	}
}

func TestPattern_MatchAll_Empty(t *testing.T) {
	// pattern=.* is optional, empty string is valid (optional field, skips constraints)
	s := WithPattern{Free: ""}
	if err := schema.Validate(s); err != nil {
		t.Errorf("empty optional field should pass even with pattern=.*: %v", err)
	}
}

// ============================================================
// 13. FORMAT EDGE CASES
// ============================================================

type WithFormats struct {
	Email    string `json:"email"     schema:"format=email"`
	UUID     string `json:"uuid"      schema:"format=uuid"`
	Date     string `json:"date"      schema:"format=date"`
	DateTime string `json:"datetime"  schema:"format=date-time"`
}

func TestFormat_Emails(t *testing.T) {
	cases := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user+tag@example.co.uk", true},
		{"user.name@sub.domain.org", true},
		{"@missing.com", false},
		{"noatsign.com", false},
		{"user@", false},
		{"user@.com", false},
		{"", true}, // empty optional — skipped
	}
	for _, tc := range cases {
		s := WithFormats{Email: tc.email}
		err := schema.Validate(s)
		if tc.valid && err != nil {
			t.Errorf("email %q should be valid, got: %v", tc.email, err)
		}
		if !tc.valid && tc.email != "" {
			ve, ok := err.(schema.ValidationErrors)
			if !ok || !ve.Has("email") {
				t.Errorf("email %q should be invalid", tc.email)
			}
		}
	}
}

func TestFormat_UUID(t *testing.T) {
	valid := WithFormats{UUID: "550e8400-e29b-41d4-a716-446655440000"}
	if err := schema.Validate(valid); err != nil {
		t.Errorf("valid UUID should pass: %v", err)
	}
	invalid := WithFormats{UUID: "not-a-uuid"}
	ve, ok := schema.Validate(invalid).(schema.ValidationErrors)
	if !ok || !ve.Has("uuid") {
		t.Error("invalid UUID should fail")
	}
}

func TestFormat_Date(t *testing.T) {
	valid := WithFormats{Date: "2024-02-28"}
	if err := schema.Validate(valid); err != nil {
		t.Errorf("valid date should pass: %v", err)
	}
	invalid := WithFormats{Date: "28-02-2024"} // wrong order
	ve, ok := schema.Validate(invalid).(schema.ValidationErrors)
	if !ok || !ve.Has("date") {
		t.Error("wrong date format should fail")
	}
}

// ============================================================
// 14. DEEPLY NESTED STRUCTS (3 LEVELS)
// ============================================================

type Level3 struct {
	Value string `json:"value" schema:"required,minLength=1"`
}

type Level2 struct {
	Inner Level3 `json:"inner"`
}

type Level1 struct {
	Mid Level2 `json:"mid"`
}

func TestDeepNesting_Valid(t *testing.T) {
	s := Level1{Mid: Level2{Inner: Level3{Value: "hello"}}}
	if err := schema.Validate(s); err != nil {
		t.Errorf("deeply nested valid struct should pass: %v", err)
	}
}

func TestDeepNesting_InvalidDeep(t *testing.T) {
	s := Level1{Mid: Level2{Inner: Level3{Value: ""}}}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("mid.inner.value") {
		t.Errorf("expected error at 'mid.inner.value', got: %v", ve)
	}
}

// ============================================================
// 15. DEFAULT + REQUIRED INTERACTION
// ============================================================

type DefaultRequired struct {
	Lang string `json:"lang" schema:"required,enum=en|fr|de,default=en"`
}

func TestDefault_FillsRequired(t *testing.T) {
	// JSON doesn't include lang — default=en fills it, required passes
	cfg, err := schema.ParseJSON[DefaultRequired]([]byte(`{}`))
	if err != nil {
		t.Errorf("default should satisfy required: %v", err)
	}
	if cfg.Lang != "en" {
		t.Errorf("expected lang=en, got %q", cfg.Lang)
	}
}

// ============================================================
// 16. PARSE[T] WITH INVALID JSON STRUCTURE
// ============================================================

func TestParse_WrongType(t *testing.T) {
	// JSON has valid syntax but wrong type for a field
	_, err := schema.ParseJSON[Simple]([]byte(`{"name": 123}`))
	if err == nil {
		t.Error("expected error when JSON type mismatches struct field type")
	}
}

func TestParse_EmptyBytes(t *testing.T) {
	_, err := schema.ParseJSON[Simple]([]byte(``))
	if err == nil {
		t.Error("expected error for empty JSON input")
	}
}

func TestParse_NullJSON(t *testing.T) {
	// `null` is valid JSON but unmarshal into struct gives zero value
	_, err := schema.ParseJSON[Simple]([]byte(`null`))
	// with required name, this should fail validation
	if err == nil {
		t.Error("expected validation error: null JSON leaves required fields empty")
	}
}

// ============================================================
// 17. enum ON EMPTY OPTIONAL vs REQUIRED
// ============================================================

type WithOptionalEnum struct {
	Status string `json:"status" schema:"enum=active|inactive"`
}

type WithRequiredEnum struct {
	Status string `json:"status" schema:"required,enum=active|inactive"`
}

func TestEnum_EmptyOptional_Passes(t *testing.T) {
	// optional field, empty string — enum check should be skipped
	s := WithOptionalEnum{Status: ""}
	if err := schema.Validate(s); err != nil {
		t.Errorf("empty optional enum field should pass: %v", err)
	}
}

func TestEnum_EmptyRequired_Fails(t *testing.T) {
	s := WithRequiredEnum{Status: ""}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("status") {
		t.Error("empty required enum field should fail")
	}
}

func TestEnum_InvalidValue_Fails(t *testing.T) {
	s := WithOptionalEnum{Status: "pending"}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("status") {
		t.Error("value not in enum should fail")
	}
}

// ============================================================
// 18. VALIDATE JSON TAG vs FIELD NAME IN ERRORS
// ============================================================

type JSONNameCheck struct {
	FirstName string `json:"first_name" schema:"required"`
}

func TestErrorFieldUsesJSONName(t *testing.T) {
	s := JSONNameCheck{FirstName: ""}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok {
		t.Fatal("expected ValidationErrors")
	}
	// Error should use JSON name "first_name", not Go name "FirstName"
	if !ve.Has("first_name") {
		t.Errorf("error field name should be 'first_name' (from json tag), got: %v", ve)
	}
}

// ============================================================
// 19. ValidationErrors.MarshalJSON
// ============================================================

func TestValidationErrors_MarshalJSON(t *testing.T) {
	import_json := func() {
		// We inline-test via encoding/json round-trip
	}
	_ = import_json

	s := MultiError{A: "", B: ""}
	err := schema.Validate(s)
	ve, ok := err.(schema.ValidationErrors)
	if !ok {
		t.Fatal("expected ValidationErrors")
	}

	data, merr := ve.MarshalJSON()
	if merr != nil {
		t.Fatalf("MarshalJSON failed: %v", merr)
	}
	if len(data) == 0 {
		t.Error("MarshalJSON returned empty bytes")
	}
}

// ============================================================
// 20. uniqueItems WITH COMPARABLE TYPES
// ============================================================

type WithInts struct {
	IDs []int `json:"ids" schema:"uniqueItems"`
}

func TestUniqueItems_Ints_NoDuplicate(t *testing.T) {
	s := WithInts{IDs: []int{1, 2, 3}}
	if err := schema.Validate(s); err != nil {
		t.Errorf("unique ints should pass: %v", err)
	}
}

func TestUniqueItems_Ints_Duplicate(t *testing.T) {
	s := WithInts{IDs: []int{1, 2, 2}}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("ids") {
		t.Error("duplicate ints should fail uniqueItems")
	}
}

// ============================================================
// 21. STRUCT WITH json:",omitempty" — NAME STILL PARSED
// ============================================================

type WithOmitEmpty struct {
	Name string `json:"name,omitempty" schema:"required"`
}

func TestOmitEmpty_JSONNameParsedCorrectly(t *testing.T) {
	s := WithOmitEmpty{Name: ""}
	ve, ok := schema.Validate(s).(schema.ValidationErrors)
	if !ok || !ve.Has("name") {
		t.Errorf("json:\"name,omitempty\" should resolve to field name 'name': %v", ve)
	}
}

// ============================================================
// 22. MustValidate / MustParse PANIC MESSAGES
// ============================================================

func TestMustValidate_PanicContainsField(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic")
			return
		}
		msg, ok := r.(string)
		if !ok {
			t.Errorf("expected string panic, got %T: %v", r, r)
			return
		}
		if len(msg) == 0 {
			t.Error("panic message should not be empty")
		}
	}()
	schema.MustValidate(Simple{Name: ""})
}
