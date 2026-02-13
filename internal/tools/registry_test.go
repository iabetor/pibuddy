package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	tool := NewDateTimeTool()
	reg.Register(tool)

	if reg.Count() != 1 {
		t.Errorf("expected count 1, got %d", reg.Count())
	}

	got, ok := reg.Get("get_datetime")
	if !ok {
		t.Fatal("expected to find tool 'get_datetime'")
	}
	if got.Name() != "get_datetime" {
		t.Errorf("expected name 'get_datetime', got %q", got.Name())
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected not to find 'nonexistent'")
	}
}

func TestRegistry_Definitions(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewDateTimeTool())
	reg.Register(NewCalculatorTool())

	defs := reg.Definitions()
	if len(defs) != 2 {
		t.Errorf("expected 2 definitions, got %d", len(defs))
	}

	for _, d := range defs {
		if d.Type != "function" {
			t.Errorf("expected type 'function', got %q", d.Type)
		}
		if d.Function.Name == "" {
			t.Error("definition name should not be empty")
		}
		if d.Function.Description == "" {
			t.Error("definition description should not be empty")
		}
	}
}

func TestRegistry_Execute(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewDateTimeTool())

	result, err := reg.Execute(context.Background(), "get_datetime", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestRegistry_ExecuteUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Execute(context.Background(), "unknown_tool", json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}
