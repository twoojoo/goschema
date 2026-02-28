package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/giovanni/goschema/schema"
)

// Address is a nested struct to demonstrate recursive validation.
type Address struct {
	Street string `json:"street" schema:"minLength=3,maxLength=100,required"`
	City   string `json:"city"   schema:"minLength=2,maxLength=50,required"`
}

// User demonstrates string, number, array, enum, format, and nested struct constraints.
type User struct {
	Name    string   `json:"name"    schema:"minLength=2,maxLength=50,required"`
	Email   string   `json:"email"   schema:"format=email,required"`
	Age     int      `json:"age"     schema:"minimum=0,maximum=120"`
	Score   float64  `json:"score"   schema:"minimum=0,maximum=100,multipleOf=0.5"`
	Tags    []string `json:"tags"    schema:"minItems=1,maxItems=10,uniqueItems"`
	Role    string   `json:"role"    schema:"enum=admin|editor|viewer"`
	Address Address  `json:"address"`
}

func main() {
	fmt.Println("=== goschema examples ===")
	fmt.Println()

	// 1. ToJSONSchema[T] — derive JSON Schema from the struct type.
	fmt.Println("--- JSON Schema for User ---")
	js, err := schema.ToJSONSchema[User]()
	if err != nil {
		log.Fatal(err)
	}
	out, _ := json.MarshalIndent(js, "", "  ")
	fmt.Println(string(out))
	fmt.Println()

	// 2. Parse[T] — unmarshal + validate valid JSON.
	fmt.Println("--- Parse valid JSON ---")
	validJSON := []byte(`{
		"name": "Alice",
		"email": "alice@example.com",
		"age": 30,
		"score": 87.5,
		"tags": ["go", "schema"],
		"role": "editor",
		"address": {"street": "Via Roma 1", "city": "Rome"}
	}`)

	user, err := schema.Parse[User](validJSON)
	if err != nil {
		log.Fatal("unexpected error:", err)
	}
	fmt.Printf("Parsed user: %+v\n\n", user)

	// 3. Parse[T] — unmarshal + validate invalid JSON.
	fmt.Println("--- Parse invalid JSON (multiple violations) ---")
	invalidJSON := []byte(`{
		"name": "A",
		"email": "not-an-email",
		"age": 200,
		"score": 87.3,
		"tags": ["go", "go"],
		"role": "superuser",
		"address": {"street": "X", "city": "R"}
	}`)

	_, err = schema.Parse[User](invalidJSON)
	if err != nil {
		ve, ok := err.(schema.ValidationErrors)
		if ok {
			fmt.Printf("Validation failed with %d error(s):\n", len(ve))
			for _, e := range ve {
				fmt.Printf("  • [%s] %s\n", e.Field, e.Message)
			}
		} else {
			fmt.Println("Parse error:", err)
		}
	}
	fmt.Println()

	// 4. MustParse[T] — demo panic recovery.
	fmt.Println("--- MustParse (panic on invalid input) ---")
	func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered panic:", r)
			}
		}()
		schema.MustParse[User]([]byte(`{"name":"X"}`))
	}()
	fmt.Println()

	// 5. Validate — use with an already-constructed struct.
	fmt.Println("--- Validate constructed struct ---")
	u := User{Name: "Bob", Email: "bob@example.com", Age: 42, Score: 50.0, Tags: []string{"test"}, Role: "admin", Address: Address{Street: "Main St", City: "NY"}}
	if err := schema.Validate(u); err != nil {
		fmt.Println("Validation error:", err)
	} else {
		fmt.Println("Struct is valid ✓")
	}
}

type MYStruct struct {
	_ any `schema:"title=MyStruct,description=MyStruct"`
}
