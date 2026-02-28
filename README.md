# goschema

A zero-dependency Go library for defining **JSON Schema constraints directly on Go structs** using struct tags — with utilities to validate, parse, and generate JSON Schema from your types.

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

user, err := schema.Parse[User](jsonBytes)   // unmarshal + defaults + validate
schema.MustValidate(user)                    // panic on violation
js, _  := schema.ToJSONSchema[User]()        // derive JSON Schema map
```

---

## Installation

```bash
go get github.com/twoojoo/goschema
```

Requires **Go 1.21+** (generics). **Zero external dependencies.**

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
    p, err := schema.Parse[Product]([]byte(`{"name":"Widget","price":9.99}`))
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

### `Parse[T any](data []byte) (T, error)`

Combines three steps in one call:
1. `json.Unmarshal` — deserialise JSON
2. **Apply defaults** — fill zero-value fields with `default=` tag values
3. `Validate` — check all constraints

```go
user, err := schema.Parse[User](jsonBytes)
```

### `MustParse[T any](data []byte) T`

Like `Parse[T]` but panics on unmarshal or validation failure.

```go
cfg := schema.MustParse[Config]([]byte(`{"env":"prod"}`))
```

### `ToJSONSchema[T any]() (map[string]any, error)`

Returns a **JSON Schema (draft-07)** compatible `map[string]any` for type `T`. The caller never imports `reflect`.

```go
js, err := schema.ToJSONSchema[User]()
out, _ := json.MarshalIndent(js, "", "  ")
fmt.Println(string(out))
```

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

### Struct-level metadata (`title` and `description`)

Use a blank identifier `_` field as a sentinel — it is invisible to the validator:

```go
type Product struct {
    _    any    `schema:"title=Product,description=A purchasable item"`
    Name string `json:"name" schema:"required"`
}

js, _ := schema.ToJSONSchema[Product]()
// js["title"]       == "Product"
// js["description"] == "A purchasable item"
```

---

## `ValidationErrors`

`ValidationErrors` implements `error` and can be cast directly for per-field inspection:

```go
err := schema.Validate(user)
if ve, ok := err.(schema.ValidationErrors); ok {
    fmt.Println(ve.Error()) // human-readable string
    ve.Has("email")         // true if "email" has at least one error

    // JSON-serialisable
    data, _ := json.Marshal(ve)
    // [{"field":"email","message":"must be a valid email","value":"bad"}]
}
```

Each `ValidationError`:

```go
type ValidationError struct {
    Field   string // dot-separated JSON path, e.g. "address.street"
    Message string // human-readable description
    Value   any    // the actual value that failed
}
```

---

## Pointer Fields

Pointer fields (`*T`) are **optional by default**. Add `required` to make them mandatory.

```go
type Doc struct {
    Name    string  `json:"name"    schema:"required"`         // must be non-empty string
    Summary *string `json:"summary"`                           // nil is fine
    Author  *string `json:"author"  schema:"required"`         // nil fails
    Score   *int    `json:"score"   schema:"minimum=0,maximum=100"` // constraints applied if non-nil
}
```

---

## Defaults

`default=` is applied by `Parse[T]` **only** when the field is the zero value after unmarshal. It is never applied by `Validate` alone.

```go
type Config struct {
    Lang    string `json:"lang"    schema:"enum=en|fr|de,default=en"`
    Timeout int    `json:"timeout" schema:"minimum=1,default=30"`
    Debug   bool   `json:"debug"   schema:"default=false"`
}

cfg, _ := schema.Parse[Config]([]byte(`{}`))
// cfg.Lang    == "en"
// cfg.Timeout == 30
```

> **Note:** For non-pointer numeric and boolean fields, a `default` will be applied even if the JSON explicitly sends the zero value (`0`, `false`, `""`). Use `*int`, `*bool`, `*string` pointers if you need to distinguish "absent" from "zero".

---

## JSON Schema Feature Support

### ✅ Supported

| Feature | Tag / Mechanism |
|---|---|
| `type` | Inferred from Go type |
| `title` | `_ any \`schema:"title=..."\`` sentinel field |
| `description` | `_ any \`schema:"description=..."\`` sentinel field |
| `required` | `schema:"required"` on field |
| `default` | `schema:"default=VALUE"` (applied in `Parse[T]`) |
| `const` | `schema:"const=VALUE"` — string, number, boolean |
| `enum` | `schema:"enum=A\|B\|C"` |
| `minLength` / `maxLength` | String — counts Unicode runes |
| `pattern` | String — standard Go regexp |
| `format` | String — `email`, `uri`, `date`, `time`, `date-time`, `uuid`, `ipv4`, `ipv6` |
| `minimum` / `maximum` | Numeric — inclusive |
| `exclusiveMinimum` / `exclusiveMaximum` | Numeric — exclusive |
| `multipleOf` | Numeric — float-precision-safe |
| `minItems` / `maxItems` | Array / slice |
| `uniqueItems` | Array / slice of comparable types |
| `minProperties` / `maxProperties` | `map[string]T` fields |
| Nested object schemas | Recursive struct validation |
| Dot-path error reporting | `"address.street"` |
| `ToJSONSchema[T]()` | Full JSON Schema map output |

### ❌ Not Supported

| Feature | Notes |
|---|---|
| `allOf` / `anyOf` / `oneOf` | Composition keywords — not expressible in flat struct tags without losing Go type structure |
| `not` | Negated schemas — same limitation as above |
| `if` / `then` / `else` | Conditional schemas (draft-07) |
| `$ref` / `$id` / `$schema` | Schema references and identifiers |
| `dependencies` / `dependentRequired` | Field-level dependency rules |
| `additionalProperties` | Meaningless for typed Go structs |
| `patternProperties` | Keys matching a regex pattern |
| `items` schema | Per-element constraints on `[]SomeStruct` (array-level constraints work) |
| `contains` | At least one item matches a schema |
| `readOnly` / `writeOnly` | Access semantics |
| `nullable` emitted in ToJSONSchema | `*T` → `["T","null"]` type union not yet emitted |
| `format`: `hostname`, `idn-email`, `uri-reference`, `regex`, etc. | Less common formats |
| Circular / recursive types | Will cause an infinite recursion — **document your types as DAGs** |

---

## Behaviour Notes

- **`minLength`/`maxLength` count Unicode runes**, not bytes. `"🚀🚀"` has length 2.
- **Optional string fields** (`required` absent) skip `pattern`, `format`, `enum`, and `const` checks when the value is `""`. Set `required` to enforce presence first.
- **`json:"-"` fields** are completely skipped, even if they carry `schema` tags.
- **Unexported fields** are always skipped.
- **`json:",omitempty"`** — the JSON name is parsed correctly (`name,omitempty` → key `name`).
- **All validation errors are collected** — `Validate` never stops at the first failure.
- **`multipleOf` uses ratio-based float comparison** (`n/factor` near integer) to avoid `math.Mod` precision issues with values like `0.3` vs `0.1`.
