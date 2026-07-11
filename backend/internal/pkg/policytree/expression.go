package policytree

import (
	"fmt"
	"strconv"
	"strings"
)

// GenerateExpression 将访问树转换为标准展示表达式，调用前应先通过 Validate。
func GenerateExpression(node Node) string {
	return expressionFor(node, true)
}

// expressionFor 递归生成表达式；嵌套逻辑节点使用括号保持可读性和语义稳定。
func expressionFor(node Node, root bool) string {
	switch node.Type {
	case NodeLeaf:
		return leafExpression(node)
	case NodeAND, NodeOR:
		parts := make([]string, 0, len(node.Children))
		for _, child := range node.Children {
			parts = append(parts, expressionFor(child, false))
		}
		joined := strings.Join(parts, " "+string(node.Type)+" ")
		if root {
			return joined
		}
		return "(" + joined + ")"
	default:
		return ""
	}
}

// leafExpression 输出属性叶子节点表达式，等号操作符沿用 attr:value 的简洁展示。
func leafExpression(node Node) string {
	value := fmt.Sprint(node.Value)
	if s, ok := node.Value.(string); ok {
		value = s
	}
	if node.Operator == OperatorNEQ {
		return node.Attribute + "!=" + quoteIfNeeded(value)
	}
	if node.Operator == OperatorGTE || node.Operator == OperatorLTE || node.Operator == OperatorBelongsTo {
		return node.Attribute + string(node.Operator) + quoteIfNeeded(value)
	}
	return node.Attribute + ":" + quoteIfNeeded(value)
}

// quoteIfNeeded 对包含空白或逻辑关键字的值加引号，避免预览表达式产生歧义。
func quoteIfNeeded(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " ()") || strings.EqualFold(value, "AND") || strings.EqualFold(value, "OR") {
		return strconv.Quote(value)
	}
	return value
}
