package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestCalculatorTool_Name(t *testing.T) {
	tool := NewCalculatorTool()
	if tool.Name() != "calculate" {
		t.Errorf("expected name 'calculate', got %q", tool.Name())
	}
}

func TestCalculatorTool_Execute(t *testing.T) {
	tool := NewCalculatorTool()

	tests := []struct {
		name     string
		expr     string
		expected string
		wantErr  bool
	}{
		{"addition", "1+2", "1+2 = 3", false},
		{"subtraction", "10-3", "10-3 = 7", false},
		{"multiplication", "3*4", "3*4 = 12", false},
		{"division", "10/4", "10/4 = 2.5", false},
		{"parentheses", "(1+2)*3", "(1+2)*3 = 9", false},
		{"complex", "(10+5)*2-8/4", "(10+5)*2-8/4 = 28", false},
		{"chinese_multiply", "3×4", "3×4 = 12", false},
		{"chinese_divide", "10÷2", "10÷2 = 5", false},
		{"chinese_parens", "（1+2）×3", "（1+2）×3 = 9", false},
		{"modulo", "10%3", "10%3 = 1", false},
		{"negative", "-5+3", "-5+3 = -2", false},
		{"float", "1.5+2.5", "1.5+2.5 = 4", false},
		{"divide_by_zero", "1/0", "", true},
		{"empty", "", "", true},
		{"invalid", "abc", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, _ := json.Marshal(calcArgs{Expression: tt.expr})
			result, err := tool.Execute(context.Background(), args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for expr %q, got result %q", tt.expr, result)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for expr %q: %v", tt.expr, err)
			}
			if !strings.Contains(result, tt.expected) {
				t.Errorf("expected result to contain %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCalculatorTool_InvalidJSON(t *testing.T) {
	tool := NewCalculatorTool()
	_, err := tool.Execute(context.Background(), json.RawMessage(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
