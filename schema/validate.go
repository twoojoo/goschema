package schema

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
)

// Pre-compiled format regexps — no external dependencies.
var formatPatterns = map[string]*regexp.Regexp{
	"email":     regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
	"uri":       regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+\-.]*://[^\s]*$`),
	"date":      regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`),
	"time":      regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?$`),
	"date-time": regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`),
	"uuid":      regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`),
	"ipv4":      regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`),
	"ipv6":      regexp.MustCompile(`(?i)^[0-9a-f:]+$`),
}

// validateValue is the core recursive validation engine.
// path is the dot-separated JSON field path for error messages.
func validateValue(v reflect.Value, schema *ObjectSchema, path string) ValidationErrors {
	var errs ValidationErrors

	// Dereference pointers.
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			errs = append(errs, checkNilPointerRequired(schema, path)...)
			return errs
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return errs
	}

	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}

		jsonName := jsonFieldName(f)
		if jsonName == "-" {
			continue
		}

		fs, ok := schema.Fields[jsonName]
		if !ok {
			continue
		}

		fv := v.Field(i)
		fp := fieldPath(path, jsonName)
		errs = append(errs, validateField(fv, fs, fp)...)
	}

	return errs
}

// fieldPath builds a dot-separated path.
func fieldPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

// checkNilPointerRequired returns errors for all required fields in a schema
// when the parent pointer is nil.
func checkNilPointerRequired(schema *ObjectSchema, path string) ValidationErrors {
	var errs ValidationErrors
	for name, fs := range schema.Fields {
		if fs.Required {
			errs = append(errs, ValidationError{
				Field:   fieldPath(path, name),
				Message: "field is required",
				Value:   nil,
			})
		}
	}
	return errs
}

// validateField validates a single field value against its FieldSchema.
func validateField(v reflect.Value, fs FieldSchema, path string) ValidationErrors {
	var errs ValidationErrors

	// Handle pointer fields.
	isPtr := v.Kind() == reflect.Ptr
	if isPtr {
		if v.IsNil() {
			if fs.Required {
				errs = append(errs, ValidationError{
					Field:   path,
					Message: "field is required",
					Value:   nil,
				})
			}
			return errs
		}
		v = v.Elem()
	}

	switch fs.Type {
	case "string":
		errs = append(errs, validateString(v, fs.String, path)...)
	case "integer", "number":
		errs = append(errs, validateNumber(v, fs.Number, path)...)
	case "boolean":
		errs = append(errs, validateBool(v, fs.Bool, path)...)
	case "array":
		errs = append(errs, validateArray(v, fs.Array, path)...)
	case "object":
		if fs.Map != nil {
			errs = append(errs, validateMap(v, fs.Map, path)...)
		} else if fs.Nested != nil {
			errs = append(errs, validateValue(v, fs.Nested, path)...)
		}
	}

	return errs
}

func validateString(v reflect.Value, c *StringConstraints, path string) ValidationErrors {
	var errs ValidationErrors
	if c == nil {
		return errs
	}

	s := v.String()

	if c.Required && s == "" {
		errs = append(errs, ValidationError{Field: path, Message: "field is required", Value: s})
		return errs
	}

	// For optional fields, skip presence-dependent constraints when empty.
	if s == "" {
		return errs
	}

	runes := []rune(s)
	runeLen := len(runes)

	if c.MinLength != nil && runeLen < *c.MinLength {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be at least %d characters long (got %d)", *c.MinLength, runeLen),
			Value:   s,
		})
	}
	if c.MaxLength != nil && runeLen > *c.MaxLength {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be at most %d characters long (got %d)", *c.MaxLength, runeLen),
			Value:   s,
		})
	}
	if c.Pattern != nil {
		re, err := regexp.Compile(*c.Pattern)
		if err != nil {
			errs = append(errs, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("invalid pattern %q: %v", *c.Pattern, err),
				Value:   s,
			})
		} else if !re.MatchString(s) {
			errs = append(errs, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("must match pattern %q", *c.Pattern),
				Value:   s,
			})
		}
	}
	if c.Format != nil {
		if re, ok := formatPatterns[*c.Format]; ok {
			if !re.MatchString(s) {
				errs = append(errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("must be a valid %s", *c.Format),
					Value:   s,
				})
			}
		}
	}
	if len(c.Enum) > 0 {
		found := false
		for _, allowed := range c.Enum {
			if s == allowed {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("must be one of %v", c.Enum),
				Value:   s,
			})
		}
	}
	if c.Const != nil && s != *c.Const {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must equal %q", *c.Const),
			Value:   s,
		})
	}

	return errs
}

func validateNumber(v reflect.Value, c *NumberConstraints, path string) ValidationErrors {
	var errs ValidationErrors
	if c == nil {
		return errs
	}

	var n float64
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(v.Int())
	case reflect.Float32, reflect.Float64:
		n = v.Float()
	default:
		return errs
	}

	if c.Minimum != nil && n < *c.Minimum {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be >= %g (got %g)", *c.Minimum, n),
			Value:   n,
		})
	}
	if c.Maximum != nil && n > *c.Maximum {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be <= %g (got %g)", *c.Maximum, n),
			Value:   n,
		})
	}
	if c.ExclusiveMin != nil && n <= *c.ExclusiveMin {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be > %g (got %g)", *c.ExclusiveMin, n),
			Value:   n,
		})
	}
	if c.ExclusiveMax != nil && n >= *c.ExclusiveMax {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must be < %g (got %g)", *c.ExclusiveMax, n),
			Value:   n,
		})
	}
	if c.MultipleOf != nil && *c.MultipleOf != 0 {
		quotient := n / *c.MultipleOf
		if math.Abs(quotient-math.Round(quotient)) > 1e-9 {
			errs = append(errs, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("must be a multiple of %g (got %g)", *c.MultipleOf, n),
				Value:   n,
			})
		}
	}
	if c.Const != nil && n != *c.Const {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must equal %g", *c.Const),
			Value:   n,
		})
	}

	return errs
}

func validateBool(v reflect.Value, c *BoolConstraints, path string) ValidationErrors {
	var errs ValidationErrors
	if c == nil {
		return errs
	}
	if c.Const != nil && v.Bool() != *c.Const {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must equal %v", *c.Const),
			Value:   v.Bool(),
		})
	}
	return errs
}

func validateArray(v reflect.Value, c *ArrayConstraints, path string) ValidationErrors {
	var errs ValidationErrors
	if c == nil {
		return errs
	}

	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return errs
	}

	n := v.Len()

	if c.Required && n == 0 {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: "field is required (empty slice)",
			Value:   n,
		})
		return errs
	}
	if c.MinItems != nil && n < *c.MinItems {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must have at least %d items (got %d)", *c.MinItems, n),
			Value:   n,
		})
	}
	if c.MaxItems != nil && n > *c.MaxItems {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must have at most %d items (got %d)", *c.MaxItems, n),
			Value:   n,
		})
	}
	if c.UniqueItems {
		seen := make(map[any]struct{}, n)
		for i := range n {
			item := v.Index(i).Interface()
			if _, dup := seen[item]; dup {
				errs = append(errs, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("items must be unique (duplicate: %v)", item),
					Value:   item,
				})
				break
			}
			seen[item] = struct{}{}
		}
	}

	return errs
}

// validateMap validates a map[string]X field against MapConstraints.
func validateMap(v reflect.Value, c *MapConstraints, path string) ValidationErrors {
	var errs ValidationErrors
	if c == nil {
		return errs
	}

	if v.Kind() != reflect.Map {
		return errs
	}

	n := v.Len()

	if c.Required && n == 0 {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: "field is required (empty map)",
			Value:   n,
		})
		return errs
	}
	if c.MinProperties != nil && n < *c.MinProperties {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must have at least %d properties (got %d)", *c.MinProperties, n),
			Value:   n,
		})
	}
	if c.MaxProperties != nil && n > *c.MaxProperties {
		errs = append(errs, ValidationError{
			Field:   path,
			Message: fmt.Sprintf("must have at most %d properties (got %d)", *c.MaxProperties, n),
			Value:   n,
		})
	}

	return errs
}

// applyDefaults walks a settable struct value and sets zero-value fields to
// their declared default (from `schema:"default=..."`) before validation runs.
// It must be called with reflect.ValueOf(&v).Elem() so fields are settable.
func applyDefaults(v reflect.Value, obj *ObjectSchema) {
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		jsonName := jsonFieldName(f)
		if jsonName == "-" {
			continue
		}
		fs, ok := obj.Fields[jsonName]
		if !ok {
			continue
		}

		fv := v.Field(i)
		if !fv.CanSet() {
			continue
		}

		// Apply the default only when the field is still the zero value.
		if fs.Default != nil && fv.IsZero() {
			raw := *fs.Default
			switch fv.Kind() {
			case reflect.String:
				fv.SetString(raw)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
					fv.SetInt(n)
				}
			case reflect.Float32, reflect.Float64:
				if f, err := strconv.ParseFloat(raw, 64); err == nil {
					fv.SetFloat(f)
				}
			case reflect.Bool:
				fv.SetBool(raw == "true")
			}
		}

		// Recurse into nested structs regardless of whether this field had a default.
		if fs.Nested != nil {
			applyDefaults(fv, fs.Nested)
		}
	}
}
