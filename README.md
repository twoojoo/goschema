# goschema — Go JSON Schema validation & generation via struct tags

**goschema** is a zero-dependency Go library that turns struct tags into a full JSON Schema (draft-07) validation, parsing, and generation engine.

[![Tests](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/twoojoo/goschema/main/badges/tests.json)](https://github.com/twoojoo/goschema/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/twoojoo/goschema/main/badges/coverage.json)](https://github.com/twoojoo/goschema/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/twoojoo/goschema/schema.svg)](https://pkg.go.dev/github.com/twoojoo/goschema/schema)
[![Latest Release](https://img.shields.io/github/v/release/twoojoo/goschema)](https://github.com/twoojoo/goschema/releases/latest)
[![Go Version](https://img.shields.io/github/go-mod/go-version/twoojoo/goschema)](https://go.dev)
[![Zero Dependencies](https://img.shields.io/badge/dependencies-none-brightgreen)](./go.mod)

Define constraints once on your Go structs — validate incoming JSON, fill defaults, and export OpenAPI-compatible schemas for free.

- ✅ **Struct-tag validation** — `required`, `minLength`, `format=email`, `enum`, `pattern`, … on any Go field
- ✅ **JSON parse + validate** — `ParseJSON[T](data)` replaces `json.Unmarshal` + a hand-written validator
- ✅ **JSON Schema generation** — `ToJSONSchema[T]()` emits draft-07 maps for structs, slices (`[]User`), and maps (`map[string]User`)
- ✅ **OpenAPI / Swagger ready** — use the emitted schema directly as a component or response definition
- ✅ **Zero external dependencies** — pure stdlib, no reflection wrappers, no code generation
- ✅ **Go generics** — type-safe API, no `interface{}` casting on your side
- ✅ **`time.Time` support** — automatically emits `format: date-time`; validates correctly post-unmarshal
- ✅ **Custom formats** — `RegisterFormat("phone", fn)` for domain-specific validators
- ✅ **OpenAPI extensions** — attach `x-` fields to any struct or field via `schema:"x-key=value"`

```go
type User struct {
    _ any `schema:"title=User,description=A registered user"`

    Name  string            `json:"name"   schema:"minLength=2,maxLength=50,required"`
    Email string            `json:"email"  schema:"format=email,required"`
    Age   int               `json:"age"    schema:"minimum=0,maximum=120"`
    Role  string            `json:"role"   schema:"enum=admin|editor|viewer,default=viewer"`
    Tags  []string          `json:"tags"   schema:"minItems=1,uniqueItems"`
    Meta  map[string]string `json:"meta"   schema:"maxProperties=10"`
}

user, err := schema.ParseJSON[User](jsonBytes) // unmarshal + defaults + validate
schema.MustValidate(user)                   // panic on violation
js, _  := schema.ToJSONSchema[User]()       // derive JSON Schema map
```

---

## Installation

```bash
go get github.com/twoojoo/goschema
```

Requires **Go 1.22+** (generics, range-over-integer). **Zero external dependencies.**

---

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/twoojoo/goschema/schema"
)

type Product struct {
    Name  string  `json:"name"  schema:"minLength=1,maxLength=100,required"`
    Price float64 `json:"price" schema:"minimum=0,exclusiveMinimum=0,required"`
    Stock int     `json:"stock" schema:"minimum=0,default=0"`
}

func main() {
    // Parse JSON, apply defaults, then validate — all in one call
    p, err := schema.ParseJSON[Product]([]byte(`{"name":"Widget","price":9.99}`))
    if err != nil {
        // err is a schema.ValidationErrors — inspect it directly
        for _, ve := range err.(schema.ValidationErrors) {
            fmt.Printf("  [%s] %s\n", ve.Field, ve.Message)
        }
        return
    }
    fmt.Println(p.Stock) // 0 — filled from default
}
```

---

## API Reference

### `Validate(v any) error`

Validates a struct (or pointer to struct) against its `schema` tags. Returns `nil` on success, or a [`ValidationErrors`](#validationerrors) slice listing every violation.

```go
err := schema.Validate(user)
if ve, ok := err.(schema.ValidationErrors); ok {
    for _, e := range ve {
        fmt.Printf("[%s] %s (value: %v)\n", e.Field, e.Message, e.Value)
    }
}
```

### `MustValidate(v any)`

Like `Validate` but **panics** on any failure. Use in `init()`, test fixtures, or hardcoded configs where a violation is a programming error.

```go
schema.MustValidate(defaultConfig) // panics if config is invalid
```

### `ParseJSON[T any](data []byte) (T, error)`

The idiomatic entry-point: combines `json.Unmarshal`, default-value filling, and validation in a single call.

```go
user, err := schema.ParseJSON[User](jsonBytes)
```

**`Parse`** is available as a legacy alias.

### `ValidateJSON[T any](data []byte) error`

Like `ParseJSON` but discards the resulting object. Useful if you only need to check validity.

```go
err := schema.ValidateJSON[User](data)
```

### `MustParseJSON[T any](data []byte) T`

Like `ParseJSON` but panics on unmarshal or validation failure.

```go
cfg := schema.MustParseJSON[Config]([]byte(`{"env":"prod"}`))
```

**`MustParse`** is available as a legacy alias.

### `MustValidateJSON[T any](data []byte)`

Like `ValidateJSON` but panics on failure.

```go
schema.MustValidateJSON[User](data)
```

### `ToJSONSchema[T any]() (map[string]any, error)`

Returns a **JSON Schema (draft-07)** compatible `map[string]any` for type `T`. Works with structs, slices, maps, and primitives.

```go
// Struct — used as a component schema in OpenAPI
js, err := schema.ToJSONSchema[User]()

// List response — emits {"type":"array","items":{...User schema...}}
js, err := schema.ToJSONSchema[[]User]()

// Dictionary response — emits {"type":"object","additionalProperties":{...User schema...}}
js, err := schema.ToJSONSchema[map[string]User]()
```

### `ToJSONSchemaIndent[T any](prefix, indent string) ([]byte, error)`

Like `ToJSONSchema` but returns the schema as indented JSON bytes.

```go
b, _ := schema.ToJSONSchemaIndent[User]("", "  ")
fmt.Println(string(b))
```

### `MustToJSONSchemaIndent[T any](prefix, indent string) []byte`

Like `ToJSONSchemaIndent` but panics on error.

```go
b := schema.MustToJSONSchemaIndent[User]("", "  ")
```

---

### `RegisterFormat(name string, validate func(value string) bool)`

Registers a custom format validator. The function is called whenever a string field has `schema:"format=name"`. The format name is also emitted in `ToJSONSchema` output, making it fully OpenAPI-compatible (custom formats are valid in draft-07 and OpenAPI 3.x — validators that don't recognise the name simply skip it).

```go
schema.RegisterFormat("phone", func(v string) bool {
    matched, _ := regexp.MatchString(`^\+?[0-9\s\-]{7,15}$`, v)
    return matched
})

type Contact struct {
    Phone string `json:"phone" schema:"format=phone"`
}
```

---

### `time.Time` support

Fields of type `time.Time` are automatically mapped to `{ "type": "string", "format": "date-time" }` in the generated schema — no tag needed. Validation is delegated to `encoding/json` which guarantees RFC 3339 compliance.

```go
type Event struct {
    Name      string    `json:"name"       schema:"required"`
    CreatedAt time.Time `json:"created_at"`  // → type: string, format: date-time
}
```

---

### Schema Extensions (`x-` fields)

Any tag key prefixed with `x-` is collected and emitted verbatim in the JSON Schema output. This is the standard mechanism for OpenAPI vendor extensions and is fully compatible with Swagger UI, Redoc, and most code-gen tools.

```go
type Product struct {
    // Struct-level extensions go in the sentinel field
    _ any `schema:"title=Product,x-internal=true,x-logo=https://example.com/logo.png"`

    // Field-level extensions
    Name  string  `json:"name"  schema:"required,x-example=Widget"`
    Price float64 `json:"price" schema:"minimum=0,x-currency=USD"`
}
```

Emits at the object root: `"x-internal": "true"`, `"x-logo": "..."`. Emits on each field: `"x-example": "Widget"`, `"x-currency": "USD"`.

---

## Tag Reference

All constraints live in the `schema:""` struct tag. Multiple constraints are separated by commas.

```go
Name string `json:"name" schema:"minLength=2,maxLength=50,required"`
```

### String fields (`string`)

| Tag | Description | Example |
|---|---|---|
| `required` | Value must be non-empty | `schema:"required"` |
| `minLength=N` | Minimum **rune** count (not bytes) | `schema:"minLength=2"` |
| `maxLength=N` | Maximum **rune** count | `schema:"maxLength=100"` |
| `pattern=REGEXP` | Must match the regular expression | `schema:"pattern=^[A-Z]+"` |
| `format=F` | Must match a named format (see below) | `schema:"format=email"` |
| `enum=A\|B\|C` | Must be one of the listed values (delimiter: `\|`) | `schema:"enum=admin\|editor\|viewer"` |
| `const=VALUE` | Must equal this exact value | `schema:"const=active"` |
| `default=VALUE` | Zero-value filled by `Parse[T]` | `schema:"default=en"` |

**Built-in formats** (no external dependencies):

| Format | Validates |
|---|---|
| `email` | RFC 5321 email address |
| `uri` | URI with scheme |
| `date` | `YYYY-MM-DD` |
| `time` | `HH:MM:SS[.sss][Z\|±HH:MM]` |
| `date-time` | ISO 8601 combined date-time |
| `uuid` | UUID v1–v5 (case-insensitive) |
| `ipv4` | Dotted-quad IPv4 address |
| `ipv6` | Hexadecimal IPv6 address |

### Numeric fields (`int`, `int8` … `int64`, `float32`, `float64`)

| Tag | Description |
|---|---|
| `minimum=N` | Value must be `>= N` |
| `maximum=N` | Value must be `<= N` |
| `exclusiveMinimum=N` | Value must be `> N` |
| `exclusiveMaximum=N` | Value must be `< N` |
| `multipleOf=N` | Value must be a multiple of N (float-precision-safe) |
| `const=VALUE` | Must equal this exact value |
| `default=VALUE` | Zero-value filled by `Parse[T]` |

### Boolean fields (`bool`)

| Tag | Description |
|---|---|
| `const=true\|false` | Must equal this exact boolean |
| `default=true\|false` | Zero-value filled by `Parse[T]` |

### Slice / array fields (`[]T`)

| Tag | Description |
|---|---|
| `required` | Slice must be non-empty |
| `minItems=N` | Must have at least N elements |
| `maxItems=N` | Must have at most N elements |
| `uniqueItems` | All elements must be distinct (comparable types) |

### Map fields (`map[string]T`)

| Tag | Description |
|---|---|
| `required` | Map must be non-empty |
| `minProperties=N` | Must have at least N keys |
| `maxProperties=N` | Must have at most N keys |

### Nested structs

Nested structs are **recursively validated** automatically. Error paths use **dot notation**:

```go
type Address struct {
    Street string `json:"street" schema:"required"`
}
type User struct {
    Address Address `json:"address"`
}
// error field: "address.street"
```

| `anyOf=S1;S2` | Value must match at least one sub-schema | `schema:"anyOf=minLength=5;pattern=^[0-9]+$"` |
| `oneOf=S1;S2` | Value must match exactly one sub-schema | `schema:"oneOf=minLength=5;pattern=^[0-9]+$"` |
| `allOf=S1;S2` | Value must match all sub-schemas | `schema:"allOf=minLength=2;pattern=^[A-Z]+$"` |
| `not=S` | Value must NOT match sub-schema | `schema:"not=minLength=5"` |
| `nullable` | `nil` is always valid | `schema:"nullable"` |

### Struct-level metadata & Advanced Object rules

Use a blank identifier `_` field as a sentinel:

```go
type Product struct {
    _    any    `schema:"title=Product,description=A item,additionalProperties=false,dependentRequired:billing_id=credit_card|billing_addr"`
    Name string `json:"name" schema:"required"`
}
```

- **`additionalProperties=false`**: Used by `Parse[T]` to forbid unknown JSON fields.
- **`dependentRequired:A=B|C`**: If field A is present, B and C must also be present.

---

## JSON Schema Feature Support

### ✅ Supported

| Feature | Tag / Mechanism |
|---|---|
| `type` | Inferred from Go type |
| `title` | `_ any \`schema:"title=..."\`` |
| `description` | `_ any \`schema:"description=..."\`` |
| `required` | `schema:"required"` |
| `default` | `schema:"default=VALUE"` |
| `const` | `schema:"const=VALUE"` |
| `enum` | `schema:"enum=A\|B\|C"` |
| `minLength` / `maxLength` | String (runes) |
| `pattern` | String (regexp) |
| `format` | `email`, `uri`, `date`, `time`, `date-time`, `uuid`, `ipv4`, `ipv6` |
| `minimum` / `maximum` | Numeric |
| `exclusiveMinimum` / `exclusiveMaximum` | Numeric |
| `multipleOf` | Numeric (float-safe) |
| `minItems` / `maxItems` | Array / slice |
| `uniqueItems` | Array / slice |
| `items` | Array elements: `schema:"items:minLength=5"` |
| `anyOf` / `oneOf` / `allOf` | Composition: `schema:"anyOf=S1;S2"` — **inline primitive constraints only; cannot reference other structs** |
| `not` | Negation: `schema:"not=S"` — **inline primitive constraints only** |
| `nullable` | `schema:"nullable"` |
| `minProperties` / `maxProperties` | Maps |
| `dependentRequired` | `schema:"dependentRequired:A=B|C"` |
| `additionalProperties` | `schema:"additionalProperties=false"` (Strict Parse) |

### ❌ Not Supported

| Feature | Notes |
|---|---|
| `if` / `then` / `else` | Conditional logic |
| `$ref` / `$id` / `$schema` | Schema references |
| `contains` | Array contains at least one match |
| `patternProperties` | Regex-based key patterns |
| POSITIONAL `items` (tuple-like) | Positional validation for heterogeneous arrays |
| Circular types | Unsupported (infinite recursion) |
| `anyOf` / `oneOf` / `allOf` / `not` referencing structs | Sub-schemas must be inline primitive constraints (e.g. `minLength=5`); you cannot reference another Go struct type |

---

## Behaviour Notes

- **`minLength`/`maxLength` count Unicode runes**, not bytes. `"🚀🚀"` has length 2.
- **Optional string fields** (`required` absent) skip `pattern`, `format`, `enum`, and `const` checks when the value is `""`. Set `required` to enforce presence first.
- **`json:"-"` fields** are completely skipped, even if they carry `schema` tags.
- **Unexported fields** are always skipped.
- **`json:",omitempty"`** — the JSON name is parsed correctly (`name,omitempty` → key `name`).
- **Consistent Errors**: `ParseJSON` and `ValidateJSON` convert standard library JSON errors (like `UnmarshalTypeError` or `SyntaxError`) into `ValidationErrors` so you can handle them uniformly.
- **`multipleOf` uses ratio-based float comparison** (`n/factor` near integer) to avoid `math.Mod` precision issues.

---

## Examples

Runnable examples are in the [`examples/`](./examples/) directory:

| Example | Description |
|---|---|
| [`examples/openapi_schema/`](./examples/openapi_schema/main.go) | Generate schemas for structs, slices (`[]User`), and maps (`map[string]User`) for OpenAPI specs |
| [`examples/validation_errors/`](./examples/validation_errors/main.go) | Inspect and serialise `ValidationErrors`; handle JSON parse errors uniformly |
| [`examples/strict_json/`](./examples/strict_json/main.go) | Strict JSON parsing with `additionalProperties=false` |

```bash
go run examples/openapi_schema/main.go
go run examples/validation_errors/main.go
go run examples/strict_json/main.go
```

---

## Agent & LLM Integration

This repo ships [`llms.txt`](./llms.txt) (a machine-readable summary following the [llms.txt spec](https://llmstxt.org)) and [`AGENTS.md`](./AGENTS.md) for AI-coding-tool context. These files help AI agents understand the library API without reading the full source.
