package schema

// StringConstraints holds JSON Schema constraints applicable to string values.
type StringConstraints struct {
	MinLength *int
	MaxLength *int
	Pattern   *string  // regexp pattern
	Format    *string  // "email", "uri", "date-time", "date", "time", "uuid"
	Enum      []string // allowed values
	Const     *string  // exact value the field must equal
	Required  bool
}

// NumberConstraints holds JSON Schema constraints applicable to numeric values
// (both integer and floating-point).
type NumberConstraints struct {
	Minimum      *float64
	Maximum      *float64
	ExclusiveMin *float64
	ExclusiveMax *float64
	MultipleOf   *float64
	Const        *float64 // exact value the field must equal
	Required     bool
}

// ArrayConstraints holds JSON Schema constraints applicable to slice/array values.
type ArrayConstraints struct {
	MinItems    *int
	MaxItems    *int
	UniqueItems bool
	Required    bool
	Items       *FieldSchema // schema for each element in the array
}

// BoolConstraints holds JSON Schema constraints applicable to boolean values.
type BoolConstraints struct {
	Const    *bool // exact value the field must equal
	Required bool
}

// MapConstraints holds JSON Schema constraints applicable to map[string]X fields.
type MapConstraints struct {
	MinProperties *int // minimum number of keys
	MaxProperties *int // maximum number of keys
	Required      bool
}

// FieldSchema represents the resolved schema for a single struct field.
type FieldSchema struct {
	// Type is the JSON Schema primitive type: "string", "number", "integer",
	// "array", "object", "boolean".
	Type string

	// JSONName is the field name as it appears in JSON (from the `json` tag).
	JSONName string

	// Default is the raw default value string (applied during Parse when the
	// field is zero-valued after unmarshal).
	Default *string

	// Exactly one of the constraint sets below will be non-nil, matching Type.
	String *StringConstraints
	Number *NumberConstraints
	Array  *ArrayConstraints
	Bool   *BoolConstraints
	Map    *MapConstraints

	// Nested holds the ObjectSchema for embedded struct fields (Type == "object").
	Nested *ObjectSchema

	Required bool

	// Advanced keywords
	Nullable bool

	AnyOf []FieldSchema
	OneOf []FieldSchema
	AllOf []FieldSchema
	Not   *FieldSchema
}

// ObjectSchema is the fully resolved schema for a struct type.
// Keys are JSON field names.
type ObjectSchema struct {
	Title       string
	Description string
	Fields      map[string]FieldSchema

	// Advanced keywords
	AdditionalProperties *bool               // nil means true (default)
	DependentRequired    map[string][]string // property dependencies
}
