package policytree

import (
	"testing"

	"go-cpabe/backend/internal/domain"
)

// TestValidateRejectsInvalidTrees 覆盖保存前必须拒绝的核心非法访问树形态。
func TestValidateRejectsInvalidTrees(t *testing.T) {
	attrs := map[string]AttributeMeta{
		"role": {Code: "role", Type: domain.PolicyAttributeEnum, Values: []string{"DATA_OWNER"}, Status: domain.PolicyStatusEnabled},
	}
	cases := []struct {
		name string
		tree Node
	}{
		{name: "empty logic", tree: Node{Type: NodeAND}},
		{name: "bad operator", tree: Node{Type: NodeLeaf, Attribute: "role", Operator: ">", Value: "DATA_OWNER"}},
		{name: "disabled attr", tree: Node{Type: NodeLeaf, Attribute: "department", Operator: OperatorEQ, Value: "研发部"}},
		{name: "bad enum", tree: Node{Type: NodeLeaf, Attribute: "role", Operator: OperatorEQ, Value: "TENANT_ADMIN"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(tc.tree, attrs); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

// TestValidateAcceptsValidTree 确认合法 AND/OR/LEAF 组合可通过校验。
func TestValidateAcceptsValidTree(t *testing.T) {
	attrs := map[string]AttributeMeta{
		"role":       {Code: "role", Type: domain.PolicyAttributeEnum, Values: []string{"DATA_OWNER", "TENANT_ADMIN"}, Status: domain.PolicyStatusEnabled},
		"department": {Code: "department", Type: domain.PolicyAttributeString, Status: domain.PolicyStatusEnabled},
	}
	tree := Node{Type: NodeOR, Children: []Node{
		{Type: NodeLeaf, Attribute: "role", Operator: OperatorEQ, Value: "TENANT_ADMIN"},
		{Type: NodeAND, Children: []Node{
			{Type: NodeLeaf, Attribute: "role", Operator: OperatorEQ, Value: "DATA_OWNER"},
			{Type: NodeLeaf, Attribute: "department", Operator: OperatorNEQ, Value: "财务部"},
		}},
	}}
	if err := Validate(tree, attrs); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
