package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/twoojoo/goschema/schema"
)

type SimpleUser struct {
	Name string `json:"name" schema:"required"`
}

func TestToJSONSchema_List(t *testing.T) {
	js, err := schema.ToJSONSchema[[]SimpleUser]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if js["type"] != "array" {
		t.Errorf("expected type 'array', got %v", js["type"])
	}

	items, ok := js["items"].(map[string]any)
	if !ok {
		t.Fatal("expected 'items' to be a map")
	}

	if items["type"] != "object" {
		t.Errorf("expected item type 'object', got %v", items["type"])
	}

	props := items["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Error("expected 'name' property in items")
	}
}

func TestToJSONSchema_Map(t *testing.T) {
	js, err := schema.ToJSONSchema[map[string]SimpleUser]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if js["type"] != "object" {
		t.Errorf("expected type 'object', got %v", js["type"])
	}

	addProps, ok := js["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatal("expected 'additionalProperties' to be a map (schema)")
	}

	if addProps["type"] != "object" {
		t.Errorf("expected additionalProperties type 'object', got %v", addProps["type"])
	}
}

func TestToJSONSchema_Primitive(t *testing.T) {
	js, err := schema.ToJSONSchema[string]()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if js["type"] != "string" {
		t.Errorf("expected type 'string', got %v", js["type"])
	}
}

func TestToJSONSchema_Indent_NonStruct(t *testing.T) {
	b, err := schema.ToJSONSchemaIndent[[]string]("", "  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var js map[string]any
	if err := json.Unmarshal(b, &js); err != nil {
		t.Fatalf("failed to unmarshal indented schema: %v", err)
	}

	if js["type"] != "array" {
		t.Errorf("expected type 'array', got %v", js["type"])
	}
}

func TestParseJSON_List(t *testing.T) {
	data := []byte(`[{"name":"Alice"},{"name":"Bob"}]`)
	users, err := schema.ParseJSON[[]SimpleUser](data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
	if users[0].Name != "Alice" || users[1].Name != "Bob" {
		t.Errorf("incorrect data: %+v", users)
	}

	// Test validation failure in list
	badData := []byte(`[{"name":"Alice"},{"name":""}]`) // name required
	_, err = schema.ParseJSON[[]SimpleUser](badData)
	if err == nil {
		t.Fatal("expected error for empty name in list")
	}
}

func TestValidate_Map(t *testing.T) {
	m := map[string]SimpleUser{
		"user1": {Name: "Alice"},
		"user2": {Name: "Bob"},
	}
	if err := schema.Validate(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test validation failure in map
	m["user3"] = SimpleUser{Name: ""} // name required
	if err := schema.Validate(m); err == nil {
		t.Fatal("expected error for empty name in map")
	}
}
