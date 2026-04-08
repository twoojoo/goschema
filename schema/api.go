package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
)

// customFormats is the registry for user-defined format validators.
// Keys are format names (without any prefix); values are func(string) bool.
var customFormats sync.Map

// RegisterFormat registers a custom format validator that will be applied
// whenever a field carries schema:"format=name".
// The validator receives the field value as a string and returns true if valid.
// Custom formats are also emitted as-is in the JSON Schema output, which is
// fully compatible with JSON Schema draft-07 and OpenAPI/Swagger specs.
//
//	schema.RegisterFormat("phone", func(v string) bool {
//	    return regexp.MustCompile(`^\+?[0-9\s\-]{7,15}$`).MatchString(v)
//	})
func RegisterFormat(name string, validate func(value string) bool) {
	customFormats.Store(name, validate)
}

// Validate checks v against the JSON Schema constraints defined by its
// `schema` struct tags. It accepts structs, pointers, slices, and maps.
// Errors from all fields are collected and returned as [ValidationErrors].
// Returns nil if all constraints pass.
//
//	err := schema.Validate(user)
//	if ve, ok := err.(schema.ValidationErrors); ok {
//	    for _, e := range ve {
//	        fmt.Printf("[%s] %s\n", e.Field, e.Message)
//	    }
//	}
func Validate(v any) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return ValidationErrors{{Field: "", Message: "value is nil", Value: nil}}
	}

	// Dereference pointer.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ValidationErrors{{Field: "", Message: "value is nil", Value: nil}}
		}
		rv = rv.Elem()
	}

	fs, err := reflectTypeToSchema(rv.Type())
	if err != nil {
		return err
	}

	errs := validateField(rv, fs, "")
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// MustValidate is like [Validate] but panics on any validation failure.
// Useful for init-time assertions and hardcoded configs where a violation
// is always a programming mistake, not a runtime condition.
//
//	var defaultCfg = Config{Env: "prod", MaxRetries: 3}
//	func init() { schema.MustValidate(defaultCfg) }
func MustValidate(v any) {
	if err := Validate(v); err != nil {
		panic("goschema: MustValidate failed: " + err.Error())
	}
}

// ToJSONSchema returns a JSON Schema (draft-07 compatible) map[string]any for
// type T. It works for any Go type: structs, slices, maps, and primitives.
// The caller never needs to import "reflect".
//
//	// Single struct
//	js, err := schema.ToJSONSchema[User]()
//
//	// OpenAPI list response
//	js, err := schema.ToJSONSchema[[]User]()
//
//	// OpenAPI dictionary response
//	js, err := schema.ToJSONSchema[map[string]User]()
func ToJSONSchema[T any]() (map[string]any, error) {
	var zero T
	t := reflect.TypeOf(zero)

	// Support both T and *T.
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil {
		return nil, fmt.Errorf("goschema: ToJSONSchema requires a non-nil type")
	}

	fs, err := reflectTypeToSchema(t)
	if err != nil {
		return nil, err
	}

	return fieldSchemaToJSON(fs), nil
}

// ToJSONSchemaIndent is like ToJSONSchema but returns the schema as indented
// JSON bytes.
func ToJSONSchemaIndent[T any](prefix, indent string) ([]byte, error) {
	m, err := ToJSONSchema[T]()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(m, prefix, indent)
}

// MustToJSONSchemaIndent is like ToJSONSchemaIndent but panics on error.
func MustToJSONSchemaIndent[T any](prefix, indent string) []byte {
	b, err := ToJSONSchemaIndent[T](prefix, indent)
	if err != nil {
		panic("goschema: MustToJSONSchemaIndent failed: " + err.Error())
	}
	return b
}

// ParseJSON unmarshals JSON data into a value of type T, fills any zero-value
// fields from `schema:"default=..."` tags, then validates all constraints.
// It is the idiomatic single-call entry-point replacing json.Unmarshal + Validate.
//
// All errors — JSON syntax, type mismatch, unknown fields, and constraint
// violations — are returned as [ValidationErrors] for uniform handling.
//
//	user, err := schema.ParseJSON[User](data)
//	if ve, ok := err.(schema.ValidationErrors); ok {
//	    // handle structured errors
//	}
func ParseJSON[T any](data []byte) (T, error) {
	var v T

	// Resolve schema for unmarshal options (e.g. DisallowUnknownFields)
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	fs, err := reflectTypeToSchema(t)
	if err != nil {
		return v, err
	}

	// Unmarshal
	dec := json.NewDecoder(bytes.NewReader(data))
	// Strict mode only applies if we have an object schema with AdditionalProperties=false.
	if fs.Nested != nil && fs.Nested.AdditionalProperties != nil && !*fs.Nested.AdditionalProperties {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&v); err != nil {
		return v, wrapUnmarshalError(err)
	}

	// Apply defaults before validation.
	rv := reflect.ValueOf(&v).Elem()
	applyFieldDefaults(rv, fs)

	if err := Validate(v); err != nil {
		return v, err
	}
	return v, nil
}

// Parse is an alias for [ParseJSON].
//
// Deprecated: use [ParseJSON] instead.
func Parse[T any](data []byte) (T, error) {
	return ParseJSON[T](data)
}

// ValidateJSON unmarshals JSON data into type T, applies defaults, validates
// all constraints, and then discards the value. Returns nil on success,
// or a [ValidationErrors] on any failure.
//
//	if err := schema.ValidateJSON[User](data); err != nil { ... }
func ValidateJSON[T any](data []byte) error {
	_, err := ParseJSON[T](data)
	return err
}

// MustParseJSON is like [ParseJSON] but panics on any error.
// Useful for hardcoded test payloads that must be valid.
//
//	cfg := schema.MustParseJSON[Config]([]byte(`{"env":"prod"}`))
func MustParseJSON[T any](data []byte) T {
	v, err := ParseJSON[T](data)
	if err != nil {
		panic("goschema: MustParseJSON failed: " + err.Error())
	}
	return v
}

// MustParse is an alias for [MustParseJSON].
//
// Deprecated: use [MustParseJSON] instead.
func MustParse[T any](data []byte) T {
	return MustParseJSON[T](data)
}

// MustValidateJSON is like [ValidateJSON] but panics on error.
//
//	schema.MustValidateJSON[User](data)
func MustValidateJSON[T any](data []byte) {
	if err := ValidateJSON[T](data); err != nil {
		panic("goschema: MustValidateJSON failed: " + err.Error())
	}
}

// wrapUnmarshalError converts standard library JSON errors into our ValidationErrors type.
func wrapUnmarshalError(err error) error {
	if err == nil {
		return nil
	}

	// 1. Type Mismatch (e.g. string sent for int field)
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return ValidationErrors{{
			Field:   typeErr.Field,
			Message: fmt.Sprintf("expected type %s", typeErr.Type.String()),
			Value:   typeErr.Value,
		}}
	}

	// 2. Syntax Error (malformed JSON)
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return ValidationErrors{{
			Field:   "",
			Message: fmt.Sprintf("invalid JSON syntax at offset %d: %s", syntaxErr.Offset, syntaxErr.Error()),
		}}
	}

	// 3. Unknown Field (from DisallowUnknownFields)
	// Example message: "json: unknown field \"xxx\""
	if strings.HasPrefix(err.Error(), "json: unknown field") {
		msg := strings.TrimPrefix(err.Error(), "json: ")
		field := ""
		fmt.Sscanf(msg, "unknown field %q", &field)
		return ValidationErrors{{
			Field:   field,
			Message: msg,
		}}
	}

	// 4. Unexpected EOF (truncated JSON)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return ValidationErrors{{
			Field:   "",
			Message: "invalid JSON: unexpected end of input",
		}}
	}

	return err
}

// ---- JSON Schema emitter ----

func objectSchemaToJSON(obj *ObjectSchema) map[string]any {
	required := []string{}
	properties := map[string]any{}

	for name, fs := range obj.Fields {
		if fs.Required {
			required = append(required, name)
		}
		properties[name] = fieldSchemaToJSON(fs)
	}

	result := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if obj.Title != "" {
		result["title"] = obj.Title
	}
	if obj.Description != "" {
		result["description"] = obj.Description
	}
	if len(required) > 0 {
		result["required"] = required
	}
	if obj.AdditionalProperties != nil {
		result["additionalProperties"] = *obj.AdditionalProperties
	}
	if len(obj.DependentRequired) > 0 {
		result["dependentRequired"] = obj.DependentRequired
	}
	for k, v := range obj.Extensions {
		result[k] = v
	}
	return result
}

func fieldSchemaToJSON(fs FieldSchema) map[string]any {
	var m map[string]any

	switch fs.Type {
	case "string":
		m = stringSchemaToJSON(fs.String)
	case "integer":
		m = numberSchemaToJSON(fs.Number)
		m["type"] = "integer"
	case "number":
		m = numberSchemaToJSON(fs.Number)
		m["type"] = "number"
	case "boolean":
		m = map[string]any{"type": "boolean"}
	case "array":
		m = arraySchemaToJSON(fs.Array)
	case "object":
		if fs.Map != nil {
			m = mapSchemaToJSON(fs.Map)
		} else if fs.Nested != nil {
			m = objectSchemaToJSON(fs.Nested)
		} else {
			m = map[string]any{"type": "object"}
		}
	default:
		m = map[string]any{}
	}

	// Advanced Keywords
	if fs.Nullable {
		m["nullable"] = true
	}
	if fs.Not != nil {
		m["not"] = fieldSchemaToJSON(*fs.Not)
	}
	if len(fs.AnyOf) > 0 {
		m["anyOf"] = compositionToJSON(fs.AnyOf)
	}
	if len(fs.OneOf) > 0 {
		m["oneOf"] = compositionToJSON(fs.OneOf)
	}
	if len(fs.AllOf) > 0 {
		m["allOf"] = compositionToJSON(fs.AllOf)
	}
	for k, v := range fs.Extensions {
		m[k] = v
	}

	return m
}

func compositionToJSON(schemas []FieldSchema) []map[string]any {
	res := make([]map[string]any, len(schemas))
	for i, s := range schemas {
		res[i] = fieldSchemaToJSON(s)
	}
	return res
}

func stringSchemaToJSON(c *StringConstraints) map[string]any {
	m := map[string]any{"type": "string"}
	if c == nil {
		return m
	}
	if c.MinLength != nil {
		m["minLength"] = *c.MinLength
	}
	if c.MaxLength != nil {
		m["maxLength"] = *c.MaxLength
	}
	if c.Pattern != nil {
		m["pattern"] = *c.Pattern
	}
	if c.Format != nil {
		m["format"] = *c.Format
	}
	if len(c.Enum) > 0 {
		m["enum"] = c.Enum
	}
	if c.Const != nil {
		m["const"] = *c.Const
	}
	return m
}

func numberSchemaToJSON(c *NumberConstraints) map[string]any {
	m := map[string]any{}
	if c == nil {
		return m
	}
	if c.Minimum != nil {
		m["minimum"] = *c.Minimum
	}
	if c.Maximum != nil {
		m["maximum"] = *c.Maximum
	}
	if c.ExclusiveMin != nil {
		m["exclusiveMinimum"] = *c.ExclusiveMin
	}
	if c.ExclusiveMax != nil {
		m["exclusiveMaximum"] = *c.ExclusiveMax
	}
	if c.MultipleOf != nil {
		m["multipleOf"] = *c.MultipleOf
	}
	if c.Const != nil {
		m["const"] = *c.Const
	}
	return m
}

func arraySchemaToJSON(c *ArrayConstraints) map[string]any {
	m := map[string]any{"type": "array"}
	if c == nil {
		return m
	}
	if c.MinItems != nil {
		m["minItems"] = *c.MinItems
	}
	if c.MaxItems != nil {
		m["maxItems"] = *c.MaxItems
	}
	if c.UniqueItems {
		m["uniqueItems"] = true
	}
	if c.Items != nil {
		m["items"] = fieldSchemaToJSON(*c.Items)
	}
	return m
}

func mapSchemaToJSON(c *MapConstraints) map[string]any {
	m := map[string]any{"type": "object"}
	if c == nil {
		return m
	}
	if c.MinProperties != nil {
		m["minProperties"] = *c.MinProperties
	}
	if c.MaxProperties != nil {
		m["maxProperties"] = *c.MaxProperties
	}
	if c.Values != nil {
		m["additionalProperties"] = fieldSchemaToJSON(*c.Values)
	}
	return m
}

// Ensure ValidationErrors satisfies the json.Marshaler interface so callers
// can serialise errors directly if needed.
var _ json.Marshaler = (ValidationErrors)(nil)

// MarshalJSON serialises ValidationErrors as a JSON array.
func (ve ValidationErrors) MarshalJSON() ([]byte, error) {
	type entry struct {
		Field   string `json:"field"`
		Message string `json:"message"`
		Value   any    `json:"value,omitempty"`
	}
	entries := make([]entry, len(ve))
	for i, e := range ve {
		entries[i] = entry{Field: e.Field, Message: e.Message, Value: e.Value}
	}
	return json.Marshal(entries)
}
