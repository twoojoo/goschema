# AGENTS.md — goschema

Instructions for AI agents working with or integrating this codebase.

## What this repo is

`goschema` is a Go library (`github.com/twoojoo/goschema`) that provides:
1. **Struct-tag-based validation** — annotate Go structs with `schema:""` tags, call `schema.Validate(v)`.
2. **JSON parsing + validation** — `schema.ParseJSON[T](data)` = unmarshal + fill defaults + validate, in one call.
3. **JSON Schema emission** — `schema.ToJSONSchema[T]()` produces a draft-07 `map[string]any`. Works on any Go type including slices and maps.

## Import path

```go
import "github.com/twoojoo/goschema/schema"
```

## Source layout

```
schema/
  api.go        ← All public-facing functions (Validate, ParseJSON, ToJSONSchema, …)
  tags.go       ← Tag parsing, reflectTypeToSchema, buildFieldSchema
  validate.go   ← Runtime validation engine (validateField, validateObject, …)
  schema.go     ← Data types: FieldSchema, ObjectSchema, *Constraints
  errors.go     ← ValidationError, ValidationErrors, Has(), Error()
  *_test.go     ← Test files alongside their source
examples/       ← Runnable use-case examples
llms.txt        ← Machine-readable API summary (llms.txt spec)
README.md       ← Full human-readable documentation
```

## How to add constraints

Constraints live as struct tags **only**. There is no builder/fluent API. The tag format is:

```go
Field Type `json:"field" schema:"constraint1=value,constraint2=value2"`
```

Struct-level metadata uses a blank sentinel field:
```go
type T struct {
    _ any `schema:"title=T,additionalProperties=false"`
    // ... fields
}
```

## Test commands

```bash
go test ./schema/...            # run all tests
go test -v ./schema/...         # verbose
go test -run TestFoo ./schema/  # run a specific test
go test -cover ./schema/...     # with coverage
```

## Key functions to know

| Function | Purpose |
|---|---|
| `Validate(v any) error` | Validate any in-memory value against schema tags |
| `ParseJSON[T]([]byte) (T, error)` | Unmarshal + defaults + validate |
| `ValidateJSON[T]([]byte) error` | Like ParseJSON but discards the result |
| `ToJSONSchema[T]() (map[string]any, error)` | Generate JSON Schema for any Go type |
| `ToJSONSchemaIndent[T]("", "  ") ([]byte, error)` | Pretty-printed JSON Schema bytes |

## Error handling pattern

All errors returned are either plain `error` or `schema.ValidationErrors`. Always type-assert:

```go
err := schema.Validate(v)
if ve, ok := err.(schema.ValidationErrors); ok {
    for _, e := range ve {
        fmt.Printf("[%s] %s\n", e.Field, e.Message)
    }
}
```

`ValidationErrors` implements `json.Marshaler` so you can serialize it directly.

## Common patterns

### OpenAPI/Swagger schema generation

```go
// Generates schema for the Items array in a list response
js, _ := schema.ToJSONSchema[[]User]()

// Generates schema for a map/dictionary response
js, _ := schema.ToJSONSchema[map[string]User]()
```

### Strict JSON parsing (reject unknown fields)

```go
type Req struct {
    _ any `schema:"additionalProperties=false"`
    // ...
}
parsed, err := schema.ParseJSON[Req](data) // unknown fields → ValidationErrors
```

### Init-time assertion

```go
var defaultCfg = Config{Env: "prod", MaxRetries: 3}
func init() { schema.MustValidate(defaultCfg) }
```

## Design rules (for agents adding features)

- **No external dependencies.** Core must remain stdlib-only.
- **All exported symbols must have Godoc comments** starting with the symbol name.
- **Every new feature needs tests** in `*_test.go` alongside the source file.
- **ValidationErrors is the single error type** — wrap any new errors into it.
- New constraint tags must be documented in both `README.md` and `llms.txt`.
