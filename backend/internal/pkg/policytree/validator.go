package policytree

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"

	"go-cpabe/backend/internal/domain"
)

var attrCodePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{1,63}$`)

// Validate 校验访问树结构、属性引用和值类型；它是保存策略前的后端权威边界。
func Validate(root Node, attrs map[string]AttributeMeta) error {
	errs := validateNode(root, attrs, "root")
	if len(errs) > 0 {
		return errs
	}
	return nil
}

// validateNode 递归校验节点；访问树 JSON 是树结构，因此循环和孤立节点主要在前端 flow 转换阶段处理。
func validateNode(node Node, attrs map[string]AttributeMeta, path string) ValidationErrors {
	switch node.Type {
	case NodeAND, NodeOR:
		return validateLogicNode(node, attrs, path)
	case NodeLeaf:
		return validateLeafNode(node, attrs, path)
	default:
		return ValidationErrors{{Path: path, Message: "节点类型必须是 AND、OR 或 LEAF"}}
	}
}

// validateLogicNode 校验 AND/OR 子节点数量并递归校验子树。
func validateLogicNode(node Node, attrs map[string]AttributeMeta, path string) ValidationErrors {
	errs := ValidationErrors{}
	if len(node.Children) < 2 {
		errs = append(errs, ValidationError{Path: path, Message: "AND/OR 逻辑节点至少需要两个子节点"})
	}
	if node.Attribute != "" || node.Operator != "" || node.Value != nil {
		errs = append(errs, ValidationError{Path: path, Message: "逻辑节点不能包含属性条件字段"})
	}
	for i, child := range node.Children {
		errs = append(errs, validateNode(child, attrs, fmt.Sprintf("%s.children[%d]", path, i))...)
	}
	return errs
}

// validateLeafNode 校验属性叶子节点的属性、操作符和值类型。
func validateLeafNode(node Node, attrs map[string]AttributeMeta, path string) ValidationErrors {
	errs := ValidationErrors{}
	if len(node.Children) > 0 {
		errs = append(errs, ValidationError{Path: path, Message: "属性叶子节点不能包含子节点"})
	}
	if !attrCodePattern.MatchString(node.Attribute) {
		errs = append(errs, ValidationError{Path: path, Message: "属性编码格式非法"})
	}
	if node.Operator != OperatorEQ && node.Operator != OperatorNEQ {
		errs = append(errs, ValidationError{Path: path, Message: "操作符只能是 = 或 !="})
	}
	meta, ok := attrs[node.Attribute]
	if !ok {
		errs = append(errs, ValidationError{Path: path, Message: "属性未开放或不存在"})
		return errs
	}
	if meta.Status != domain.PolicyStatusEnabled {
		errs = append(errs, ValidationError{Path: path, Message: "属性已禁用"})
	}
	if err := validateValue(node.Value, meta); err != nil {
		errs = append(errs, ValidationError{Path: path, Message: err.Error()})
	}
	return errs
}

// validateValue 按属性类型校验叶子节点值，防止客户端绕过前端控件提交非法值。
func validateValue(value any, meta AttributeMeta) error {
	if value == nil {
		return fmt.Errorf("属性值不能为空")
	}
	switch meta.Type {
	case domain.PolicyAttributeEnum:
		text, ok := value.(string)
		if !ok || text == "" {
			return fmt.Errorf("枚举属性值必须是非空字符串")
		}
		for _, allowed := range meta.Values {
			if text == allowed {
				return nil
			}
		}
		return fmt.Errorf("枚举属性值不在平台开放范围内")
	case domain.PolicyAttributeNumber:
		if _, ok := numericValue(value); !ok {
			return fmt.Errorf("数字属性值必须是数字")
		}
	case domain.PolicyAttributeString:
		text, ok := value.(string)
		if !ok || text == "" {
			return fmt.Errorf("文本属性值不能为空")
		}
	default:
		return fmt.Errorf("属性类型非法")
	}
	return nil
}

// numericValue 兼容 JSON 解码后的 float64、json.Number 和前端可能提交的数字字符串。
func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		n, err := strconv.ParseFloat(v, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
