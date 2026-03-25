// Example: openapi_schema demonstrates generating JSON Schema for OpenAPI/Swagger specs.
// Run: go run examples/openapi_schema/main.go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/twoojoo/goschema/schema"
)

type User struct {
	_ any `schema:"title=User,description=A registered platform user"`

	ID    int    `json:"id"    schema:"minimum=1,required"`
	Name  string `json:"name"  schema:"minLength=1,maxLength=100,required"`
	Email string `json:"email" schema:"format=email,required"`
	Role  string `json:"role"  schema:"enum=admin|editor|viewer,default=viewer"`
}

func main() {
	// Schema for a single object — use directly as a component in OpenAPI.
	userSchema, err := schema.ToJSONSchemaIndent[User]("", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println("=== Single User Schema ===")
	fmt.Println(string(userSchema))

	// Schema for a list endpoint response (`items` is User).
	listSchema, _ := schema.ToJSONSchema[[]User]()
	listJSON, _ := json.MarshalIndent(listSchema, "", "  ")
	fmt.Println("\n=== List Response Schema ===")
	fmt.Println(string(listJSON))

	// Schema for a dictionary endpoint response (map: id → User).
	mapSchema, _ := schema.ToJSONSchema[map[string]User]()
	mapJSON, _ := json.MarshalIndent(mapSchema, "", "  ")
	fmt.Println("\n=== Map Response Schema ===")
	fmt.Println(string(mapJSON))
}
