package schema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// parseObjectSchema builds an ObjectSchema by inspecting the reflect.Type of a
// struct. It is called recursively for nested struct fields.
func parseObjectSchema(t reflect.Type) (*ObjectSchema, error) {
	// Dereference pointer types.
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("goschema: expected struct, got %s", t.Kind())
	}

	obj := &ObjectSchema{Fields: make(map[string]FieldSchema)}

	for i := range t.NumField() {
		f := t.Field(i)

		// The blank identifier field `_ any` is a sentinel for struct-level metadata
		// (title, description). It is not a real field and must not be validated.
		if f.Name == "_" {
			opts := parseTagOptions(f.Tag.Get("schema"))
			if v, ok := opts["title"]; ok {
				obj.Title = v
			}
			if v, ok := opts["description"]; ok {
				obj.Description = v
			}
			if v, ok := opts["additionalProperties"]; ok {
				b := v == "true"
				obj.AdditionalProperties = &b
			}
			// dependentRequired:fieldA=fieldB|fieldC
			for k, v := range opts {
				if strings.HasPrefix(k, "dependentRequired:") {
					if obj.DependentRequired == nil {
						obj.DependentRequired = make(map[string][]string)
					}
					sourceField := strings.TrimPrefix(k, "dependentRequired:")
					requiredFields := strings.Split(v, "|")
					obj.DependentRequired[sourceField] = requiredFields
				}
			}
			continue
		}

		// Skip unexported fields.
		if !f.IsExported() {
			continue
		}

		// Determine the JSON name.
		jsonName := jsonFieldName(f)
		if jsonName == "-" {
			continue
		}

		// Build the FieldSchema.
		fs, err := buildFieldSchema(f, jsonName)
		if err != nil {
			return nil, fmt.Errorf("goschema: field %q: %w", f.Name, err)
		}

		obj.Fields[jsonName] = fs
	}

	return obj, nil
}

// jsonFieldName returns the JSON key for a struct field, honouring the `json`
// tag. Falls back to the field name if no tag is present.
func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "" {
		return f.Name
	}
	return parts[0]
}

// buildFieldSchema maps a reflect.StructField to a FieldSchema by combining
// the Go type information with the `schema` struct tag.
func buildFieldSchema(f reflect.StructField, jsonName string) (FieldSchema, error) {
	ft := f.Type

	// Dereference pointer — a nil pointer means "not required" by default.
	isPtr := ft.Kind() == reflect.Ptr
	if isPtr {
		ft = ft.Elem()
	}

	rawTag := f.Tag.Get("schema")
	opts := parseTagOptions(rawTag)

	fs := FieldSchema{JSONName: jsonName}
	// `required` can be set explicitly in the tag; pointers are optional by
	// default unless required is set.
	fs.Required = opts["required"] == "true" || (!isPtr && opts["required"] != "false" && rawTagHasKey(rawTag, "required"))

	// Parse default value (raw string — applied during Parse[T]).
	if v, ok := opts["default"]; ok {
		fs.Default = &v
	}

	switch ft.Kind() {
	case reflect.String:
		fs.Type = "string"
		sc, err := buildStringConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.String = sc

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		fs.Type = "integer"
		nc, err := buildNumberConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.Number = nc

	case reflect.Float32, reflect.Float64:
		fs.Type = "number"
		nc, err := buildNumberConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.Number = nc

	case reflect.Bool:
		fs.Type = "boolean"
		bc, err := buildBoolConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.Bool = bc

	case reflect.Slice, reflect.Array:
		fs.Type = "array"
		ac, err := buildArrayConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.Array = ac

	case reflect.Map:
		fs.Type = "object"
		mc, err := buildMapConstraints(opts, fs.Required)
		if err != nil {
			return fs, err
		}
		fs.Map = mc

	case reflect.Struct:
		fs.Type = "object"
		nested, err := parseObjectSchema(ft)
		if err != nil {
			return fs, err
		}
		fs.Nested = nested

	default:
		fs.Type = "any"
	}

	// Advanced keywords
	fs.Nullable = opts["nullable"] == "true"

	// Composition (simple one-rule-per-schema for now)
	if v, ok := opts["not"]; ok {
		sub, err := buildSubSchema(v)
		if err != nil {
			return fs, err
		}
		fs.Not = sub
	}

	// For multiple sub-schemas (anyOf/oneOf/allOf), we look for semi-colon separated lists
	// e.g. anyOf="minLength=5;pattern=^[0-9]+$"
	parseComposition := func(key string) ([]FieldSchema, error) {
		if v, ok := opts[key]; ok {
			schemas := strings.Split(v, ";")
			res := make([]FieldSchema, 0, len(schemas))
			for _, s := range schemas {
				sub, err := buildSubSchema(s)
				if err != nil {
					return nil, err
				}
				res = append(res, *sub)
			}
			return res, nil
		}
		return nil, nil
	}

	var err error
	if fs.AnyOf, err = parseComposition("anyOf"); err != nil {
		return fs, err
	}
	if fs.OneOf, err = parseComposition("oneOf"); err != nil {
		return fs, err
	}
	if fs.AllOf, err = parseComposition("allOf"); err != nil {
		return fs, err
	}

	return fs, nil
}

// buildSubSchema builds a FieldSchema from a subset of a tag string.
func buildSubSchema(raw string) (*FieldSchema, error) {
	opts := parseTagOptions(raw)
	// We don't have reflect.StructField here, so we assume a generic "any" type
	// and apply whatever constraints are in the options.
	fs := &FieldSchema{Type: "any"}
	var err error

	// Try building all constraint types; the validator will use whichever is non-nil.
	if fs.String, err = buildStringConstraints(opts, false); err != nil {
		return nil, err
	}
	if fs.Number, err = buildNumberConstraints(opts, false); err != nil {
		return nil, err
	}
	if fs.Bool, err = buildBoolConstraints(opts, false); err != nil {
		return nil, err
	}
	// We don't recurse into array/object here for simplicity in tags.
	return fs, nil
}

// parseTagOptions parses a `schema` tag value into a key→value map.
//
// Tag grammar:
//
//	schema:"minLength=2,maxLength=50,pattern=^[a-z]+$,required"
//
// Boolean flags (like `required` and `uniqueItems`) are represented as
// key→"true" when present.
func parseTagOptions(tag string) map[string]string {
	opts := make(map[string]string)
	if tag == "" {
		return opts
	}
	// We can't just split on "," because pattern values may contain commas.
	// Strategy: scan for "key=value" pairs; boolean flags have no "=".
	// We split on commas that are NOT inside a value (values are everything
	// after "=" until the next unescaped comma that is followed by a key).
	parts := splitTagParts(tag)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			// Boolean flag.
			opts[part] = "true"
		} else {
			key := strings.TrimSpace(part[:idx])
			val := strings.TrimSpace(part[idx+1:])
			opts[key] = val
		}
	}
	return opts
}

// splitTagParts splits the raw tag string on commas, but only at positions
// that are followed by a known keyword — allowing commas inside patterns.
func splitTagParts(tag string) []string {
	// Known multi-word keys — everything else is a key name without commas.
	// We use a simple greedy split and rely on the fact that pattern values
	// that contain commas are unusual; for robustness we handle it by
	// treating the first segment that contains "=" specially.
	var parts []string
	var buf strings.Builder
	i := 0
	for i < len(tag) {
		ch := tag[i]
		if ch == ',' {
			parts = append(parts, buf.String())
			buf.Reset()
		} else {
			buf.WriteByte(ch)
		}
		i++
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

// rawTagHasKey returns true if the raw tag string contains the given key as a
// standalone token (with or without a value).
func rawTagHasKey(tag, key string) bool {
	for _, part := range splitTagParts(tag) {
		part = strings.TrimSpace(part)
		if part == key || strings.HasPrefix(part, key+"=") {
			return true
		}
	}
	return false
}

// ---- per-type constraint builders ----

func buildStringConstraints(opts map[string]string, required bool) (*StringConstraints, error) {
	sc := &StringConstraints{Required: required}

	if v, ok := opts["minLength"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("minLength must be an integer: %w", err)
		}
		sc.MinLength = &n
	}
	if v, ok := opts["maxLength"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("maxLength must be an integer: %w", err)
		}
		sc.MaxLength = &n
	}
	if v, ok := opts["pattern"]; ok {
		sc.Pattern = &v
	}
	if v, ok := opts["format"]; ok {
		sc.Format = &v
	}
	if v, ok := opts["enum"]; ok {
		sc.Enum = strings.Split(v, "|")
	}
	if v, ok := opts["const"]; ok {
		sc.Const = &v
	}

	return sc, nil
}

func buildNumberConstraints(opts map[string]string, required bool) (*NumberConstraints, error) {
	nc := &NumberConstraints{Required: required}

	parseF := func(key string) (*float64, error) {
		v, ok := opts[key]
		if !ok {
			return nil, nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("%s must be a number: %w", key, err)
		}
		return &f, nil
	}

	var err error
	if nc.Minimum, err = parseF("minimum"); err != nil {
		return nil, err
	}
	if nc.Maximum, err = parseF("maximum"); err != nil {
		return nil, err
	}
	if nc.ExclusiveMin, err = parseF("exclusiveMinimum"); err != nil {
		return nil, err
	}
	if nc.ExclusiveMax, err = parseF("exclusiveMaximum"); err != nil {
		return nil, err
	}
	if nc.MultipleOf, err = parseF("multipleOf"); err != nil {
		return nil, err
	}
	if nc.Const, err = parseF("const"); err != nil {
		return nil, err
	}

	return nc, nil
}

func buildBoolConstraints(opts map[string]string, required bool) (*BoolConstraints, error) {
	bc := &BoolConstraints{Required: required}
	if v, ok := opts["const"]; ok {
		b := v == "true"
		bc.Const = &b
	}
	return bc, nil
}

func buildArrayConstraints(opts map[string]string, required bool) (*ArrayConstraints, error) {
	ac := &ArrayConstraints{Required: required}

	if v, ok := opts["minItems"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("minItems must be an integer: %w", err)
		}
		ac.MinItems = &n
	}
	if v, ok := opts["maxItems"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("maxItems must be an integer: %w", err)
		}
		ac.MaxItems = &n
	}
	if opts["uniqueItems"] == "true" {
		ac.UniqueItems = true
	}

	// items:minLength=5
	itemsRaw := ""
	for k, v := range opts {
		if strings.HasPrefix(k, "items:") {
			rule := strings.TrimPrefix(k, "items:")
			if itemsRaw != "" {
				itemsRaw += ","
			}
			itemsRaw += rule + "=" + v
		}
	}
	if itemsRaw != "" {
		sub, err := buildSubSchema(itemsRaw)
		if err != nil {
			return nil, err
		}
		ac.Items = sub
	}

	return ac, nil
}

func buildMapConstraints(opts map[string]string, required bool) (*MapConstraints, error) {
	mc := &MapConstraints{Required: required}

	if v, ok := opts["minProperties"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("minProperties must be an integer: %w", err)
		}
		mc.MinProperties = &n
	}
	if v, ok := opts["maxProperties"]; ok {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("maxProperties must be an integer: %w", err)
		}
		mc.MaxProperties = &n
	}

	return mc, nil
}
