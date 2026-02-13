package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
)

// CalculatorTool 数学表达式计算器。
type CalculatorTool struct{}

func NewCalculatorTool() *CalculatorTool {
	return &CalculatorTool{}
}

func (t *CalculatorTool) Name() string { return "calculate" }

func (t *CalculatorTool) Description() string {
	return "计算数学表达式。支持加减乘除、括号。当用户需要数学计算时使用。"
}

func (t *CalculatorTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"expression": {
				"type": "string",
				"description": "要计算的数学表达式，例如 (1+2)*3"
			}
		},
		"required": ["expression"]
	}`)
}

type calcArgs struct {
	Expression string `json:"expression"`
}

func (t *CalculatorTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var a calcArgs
	if err := json.Unmarshal(args, &a); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	expr := strings.TrimSpace(a.Expression)
	if expr == "" {
		return "", fmt.Errorf("表达式不能为空")
	}

	result, err := evalExpr(expr)
	if err != nil {
		return "", fmt.Errorf("计算失败: %w", err)
	}

	if result == math.Trunc(result) {
		return fmt.Sprintf("%s = %d", expr, int64(result)), nil
	}
	return fmt.Sprintf("%s = %g", expr, result), nil
}

// evalExpr 使用 Go AST 解析并求值简单算术表达式。
func evalExpr(expr string) (float64, error) {
	// 替换中文符号
	expr = strings.ReplaceAll(expr, "×", "*")
	expr = strings.ReplaceAll(expr, "÷", "/")
	expr = strings.ReplaceAll(expr, "（", "(")
	expr = strings.ReplaceAll(expr, "）", ")")

	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("表达式语法错误: %s", expr)
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("不支持的类型: %s", n.Value)

	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("除数不能为零")
			}
			return left / right, nil
		case token.REM:
			if right == 0 {
				return 0, fmt.Errorf("除数不能为零")
			}
			return math.Mod(left, right), nil
		default:
			return 0, fmt.Errorf("不支持的运算符: %s", n.Op)
		}

	case *ast.ParenExpr:
		return evalNode(n.X)

	case *ast.UnaryExpr:
		val, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		return val, nil

	default:
		return 0, fmt.Errorf("不支持的表达式类型")
	}
}
