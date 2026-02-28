package schema_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/twoojoo/goschema/schema"
)

// ---- test structs ----

type Address struct {
	Street string `json:"street" schema:"minLength=3,maxLength=100,required"`
	City   string `json:"city"   schema:"minLength=2,maxLength=50,required"`
}

type User struct {
	Name    string   `json:"name"   schema:"minLength=2,maxLength=50,required"`
	Email   string   `json:"email"  schema:"format=email,required"`
	Age     int      `json:"age"    schema:"minimum=0,maximum=120"`
	Score   float64  `json:"score"  schema:"minimum=0,maximum=100,multipleOf=0.5"`
	Tags    []string `json:"tags"   schema:"minItems=1,maxItems=10,uniqueItems"`
	Role    string   `json:"role"   schema:"enum=admin|editor|viewer"`
	Bio     string   `json:"bio"    schema:"maxLength=500"`
	Address Address  `json:"address"`
}

// ---- helper ----

func mustValidationErrors(t *testing.T, err error) schema.ValidationErrors {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation errors, got nil")
	}
	ve, ok := err.(schema.ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	return ve
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertHasField(t *testing.T, ve schema.ValidationErrors, field string) {
	t.Helper()
	if !ve.Has(field) {
		t.Errorf("expected validation error for field %q, got: %v", field, ve)
	}
}

// ---- string constraints ----

func TestString_Required(t *testing.T) {
	u := User{Name: "", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "name")
}

func TestString_MinLength(t *testing.T) {
	u := User{Name: "A", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "name")
}

func TestString_MaxLength(t *testing.T) {
	u := User{
		Name:    strings.Repeat("x", 51),
		Email:   "a@b.com",
		Age:     25,
		Tags:    []string{"x"},
		Address: Address{Street: "Main St", City: "Rome"},
	}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "name")
}

func TestString_FormatEmail_Valid(t *testing.T) {
	u := User{Name: "Alice", Email: "alice@example.com", Age: 25, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	assertNoError(t, schema.Validate(u))
}

func TestString_FormatEmail_Invalid(t *testing.T) {
	u := User{Name: "Alice", Email: "not-an-email", Age: 25, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "email")
}

func TestString_Enum_Valid(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Role: "admin", Address: Address{Street: "Main St", City: "Rome"}}
	assertNoError(t, schema.Validate(u))
}

func TestString_Enum_Invalid(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Role: "superuser", Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "role")
}

// ---- number constraints ----

func TestNumber_Minimum(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: -1, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "age")
}

func TestNumber_Maximum(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 200, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "age")
}

func TestNumber_MultipleOf_Valid(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Score: 87.5, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	assertNoError(t, schema.Validate(u))
}

func TestNumber_MultipleOf_Invalid(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Score: 87.3, Tags: []string{"x"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "score")
}

// ---- array constraints ----

func TestArray_MinItems(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "tags")
}

func TestArray_MaxItems(t *testing.T) {
	tags := make([]string, 11)
	for i := range tags {
		tags[i] = "t"
	}
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: tags, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "tags")
}

func TestArray_UniqueItems(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{"go", "go"}, Address: Address{Street: "Main St", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "tags")
}

// ---- nested struct ----

func TestNested_Required(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Address: Address{Street: "", City: "Rome"}}
	ve := mustValidationErrors(t, schema.Validate(u))
	assertHasField(t, ve, "address.street")
}

func TestNested_Valid(t *testing.T) {
	u := User{Name: "Alice", Email: "a@b.com", Age: 25, Tags: []string{"x"}, Address: Address{Street: "Via Roma 1", City: "Rome"}}
	assertNoError(t, schema.Validate(u))
}

// ---- Parse[T] ----

func TestParse_Valid(t *testing.T) {
	data := []byte(`{"name":"Alice","email":"alice@example.com","age":30,"tags":["go"],"address":{"street":"Via Roma 1","city":"Rome"}}`)
	u, err := schema.Parse[User](data)
	assertNoError(t, err)
	if u.Name != "Alice" {
		t.Errorf("expected Name=Alice, got %q", u.Name)
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := schema.Parse[User]([]byte(`{not json}`))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParse_ValidationFails(t *testing.T) {
	data := []byte(`{"name":"A","email":"bad","age":30,"tags":["go"],"address":{"street":"Via Roma 1","city":"Rome"}}`)
	_, err := schema.Parse[User](data)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// ---- MustParse[T] ----

func TestMustParse_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustParse to panic on invalid input")
		}
	}()
	schema.MustParse[User]([]byte(`{"name":"X"}`)) // too short + missing required fields
}

// ---- MustValidate ----

func TestMustValidate_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected MustValidate to panic")
		}
	}()
	schema.MustValidate(User{Name: ""}) // required field missing
}

// ---- ToJSONSchema[T] ----

func TestToJSONSchema(t *testing.T) {
	js, err := schema.ToJSONSchema[User]()
	assertNoError(t, err)

	if js["type"] != "object" {
		t.Errorf("expected type=object, got %v", js["type"])
	}
	props, ok := js["properties"].(map[string]any)
	if !ok {
		t.Fatal("expected properties map")
	}
	if _, ok := props["name"]; !ok {
		t.Error("expected 'name' in properties")
	}
	if _, ok := props["email"]; !ok {
		t.Error("expected 'email' in properties")
	}
}

func TestToJSONSchemaIndent(t *testing.T) {
	b, err := schema.ToJSONSchemaIndent[User]("", "  ")
	assertNoError(t, err)

	// Check if it's valid JSON and contains expected content.
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("failed to unmarshal indented JSON: %v", err)
	}

	if m["type"] != "object" {
		t.Errorf("expected type=object, got %v", m["type"])
	}

	// Crude check for indentation: it should contain a newline and spaces.
	if !bytes.Contains(b, []byte("\n  ")) {
		t.Error("expected indented JSON to contain newline and spaces")
	}
}

func TestMustToJSONSchemaIndent(t *testing.T) {
	b := schema.MustToJSONSchemaIndent[User]("", "\t")
	if !bytes.Contains(b, []byte("\t")) {
		t.Error("expected indented JSON to contain tab character")
	}
}

// ---- full valid struct ----

func TestValidate_AllValid(t *testing.T) {
	u := User{
		Name:    "Alice",
		Email:   "alice@example.com",
		Age:     30,
		Score:   95.5,
		Tags:    []string{"go", "schema"},
		Role:    "editor",
		Bio:     "Hello world",
		Address: Address{Street: "Via Roma 1", City: "Rome"},
	}
	assertNoError(t, schema.Validate(u))
}

// ---- const constraint ----

type StatusDoc struct {
	Status  string  `json:"status"  schema:"const=active"`
	Version float64 `json:"version" schema:"const=2"`
	Debug   bool    `json:"debug"   schema:"const=true"`
}

func TestConst_String_Valid(t *testing.T) {
	s := StatusDoc{Status: "active", Version: 2, Debug: true}
	assertNoError(t, schema.Validate(s))
}

func TestConst_String_Invalid(t *testing.T) {
	s := StatusDoc{Status: "inactive", Version: 2, Debug: true}
	ve := mustValidationErrors(t, schema.Validate(s))
	assertHasField(t, ve, "status")
}

func TestConst_Number_Invalid(t *testing.T) {
	s := StatusDoc{Status: "active", Version: 1, Debug: true}
	ve := mustValidationErrors(t, schema.Validate(s))
	assertHasField(t, ve, "version")
}

func TestConst_Bool_Invalid(t *testing.T) {
	s := StatusDoc{Status: "active", Version: 2, Debug: false}
	ve := mustValidationErrors(t, schema.Validate(s))
	assertHasField(t, ve, "debug")
}

// ---- default values ----

type Config struct {
	Lang    string `json:"lang"    schema:"enum=en|fr|de,default=en"`
	Timeout int    `json:"timeout" schema:"minimum=1,default=30"`
	Verbose bool   `json:"verbose" schema:"default=true"`
}

func TestDefault_AppliedOnEmptyJSON(t *testing.T) {
	cfg, err := schema.Parse[Config]([]byte(`{}`))
	assertNoError(t, err)
	if cfg.Lang != "en" {
		t.Errorf("expected lang=en, got %q", cfg.Lang)
	}
	if cfg.Timeout != 30 {
		t.Errorf("expected timeout=30, got %d", cfg.Timeout)
	}
	if !cfg.Verbose {
		t.Error("expected verbose=true")
	}
}

func TestDefault_NotOverriddenWhenSet(t *testing.T) {
	cfg, err := schema.Parse[Config]([]byte(`{"lang":"fr","timeout":60}`))
	assertNoError(t, err)
	if cfg.Lang != "fr" {
		t.Errorf("expected lang=fr, got %q", cfg.Lang)
	}
	if cfg.Timeout != 60 {
		t.Errorf("expected timeout=60, got %d", cfg.Timeout)
	}
}

// ---- map constraints ----

type Service struct {
	Labels map[string]string `json:"labels" schema:"minProperties=1,maxProperties=5"`
}

func TestMap_MinProperties(t *testing.T) {
	s := Service{Labels: map[string]string{}}
	ve := mustValidationErrors(t, schema.Validate(s))
	assertHasField(t, ve, "labels")
}

func TestMap_MaxProperties(t *testing.T) {
	labels := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5", "f": "6"}
	s := Service{Labels: labels}
	ve := mustValidationErrors(t, schema.Validate(s))
	assertHasField(t, ve, "labels")
}

func TestMap_Valid(t *testing.T) {
	s := Service{Labels: map[string]string{"env": "prod", "team": "platform"}}
	assertNoError(t, schema.Validate(s))
}

// ---- title / description in ToJSONSchema ----

type AnnotatedStruct struct {
	_    any    `schema:"title=My Object,description=A well-documented struct"`
	Name string `json:"name" schema:"required"`
}

func TestToJSONSchema_TitleDescription(t *testing.T) {
	js, err := schema.ToJSONSchema[AnnotatedStruct]()
	assertNoError(t, err)
	if js["title"] != "My Object" {
		t.Errorf("expected title='My Object', got %v", js["title"])
	}
	if js["description"] != "A well-documented struct" {
		t.Errorf("expected description='A well-documented struct', got %v", js["description"])
	}
}
