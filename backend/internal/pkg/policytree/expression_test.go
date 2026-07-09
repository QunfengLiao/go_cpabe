package policytree

import "testing"

// TestGenerateExpression 确认表达式生成保留逻辑关系和必要括号。
func TestGenerateExpression(t *testing.T) {
	tree := Node{Type: NodeOR, Children: []Node{
		{Type: NodeAND, Children: []Node{
			{Type: NodeLeaf, Attribute: "department", Operator: OperatorEQ, Value: "研发部"},
			{Type: NodeLeaf, Attribute: "role", Operator: OperatorEQ, Value: "DATA_OWNER"},
		}},
		{Type: NodeLeaf, Attribute: "role", Operator: OperatorEQ, Value: "TENANT_ADMIN"},
	}}
	got := GenerateExpression(tree)
	want := "(department:研发部 AND role:DATA_OWNER) OR role:TENANT_ADMIN"
	if got != want {
		t.Fatalf("expression mismatch\nwant: %s\n got: %s", want, got)
	}
}
