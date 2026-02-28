package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// Validate checks a struct value against its `schema` struct tags.
// It returns nil if all constraints pass, or a [ValidationErrors] value
// listing every violation found.
func Validate(v any) error {
	rv := reflect.ValueOf(v)

	// Dereference pointer.
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ValidationErrors{{Field: "", Message: "value is nil", Value: nil}}
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("goschema: Validate expects a struct or pointer to struct, got %T", v)
	}

	obj, err := parseObjectSchema(rv.Type())
	if err != nil {
		return err
	}

	errs := validateValue(rv, obj, "")
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// MustValidate is like [Validate] but panics on any validation failure.
// Intended for init-time assertions and tests where a validation error is a
// programming mistake rather than a runtime condition.
func MustValidate(v any) {
	if err := Validate(v); err != nil {
		panic("goschema: MustValidate failed: " + err.Error())
	}
}

// ToJSONSchema returns the JSON Schema (draft-07 compatible) representation
// of type T as a map. The caller never needs to import "reflect".
//
//	js, err := schema.ToJSONSchema[User]()
func ToJSONSchema[T any]() (map[string]any, error) {
	var zero T
	t := reflect.TypeOf(zero)

	// Support both T and *T.
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("goschema: ToJSONSchema requires a struct type parameter")
	}

	obj, err := parseObjectSchema(t)
	if err != nil {
		return nil, err
	}

	return objectSchemaToJSON(obj), nil
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

// Parse unmarshals JSON data into a value of type T and validates it against
// the struct's `schema` tags. It is the idiomatic entry-point combining
// json.Unmarshal, default-filling, and Validate in a single call.
//
//	user, err := schema.Parse[User](data)
func Parse[T any](data []byte) (T, error) {
	var v T

	// Resolve schema to check AdditionalProperties
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	obj, err := parseObjectSchema(t)
	if err != nil {
		return v, err
	}

	// Unmarshal
	dec := json.NewDecoder(bytes.NewReader(data))
	if obj.AdditionalProperties != nil && !*obj.AdditionalProperties {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("goschema: parse error: %w", err)
	}

	// Apply defaults to zero-value fields before validation.
	rv := reflect.ValueOf(&v).Elem()
	applyDefaults(rv, obj)

	if err := Validate(v); err != nil {
		return v, err
	}
	return v, nil
}

// MustParse is like [Parse] but panics on any error (unmarshal or validation).
// Useful for hardcoded/test data that is known to be valid.
//
//	user := schema.MustParse[User]([]byte(`{"name":"Alice","age":30}`))
func MustParse[T any](data []byte) T {
	v, err := Parse[T](data)
	if err != nil {
		panic("goschema: MustParse failed: " + err.Error())
	}
	return v
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
