package policytree

import (
	"encoding/json"
	"fmt"
	"strings"

	"go-cpabe/backend/internal/domain"
)

// NodeType 表示访问树节点类型，当前阶段只支持 AND、OR 和属性叶子节点。
type NodeType string

const (
	// NodeAND 表示所有子节点都必须满足。
	NodeAND NodeType = "AND"
	// NodeOR 表示任一子节点满足即可。
	NodeOR NodeType = "OR"
	// NodeLeaf 表示具体属性条件。
	NodeLeaf NodeType = "LEAF"
)

// Operator 表示属性叶子节点支持的比较操作符。
type Operator string

const (
	// OperatorEQ 表示属性值必须等于指定值。
	OperatorEQ Operator = "="
	// OperatorNEQ 表示属性值必须不等于指定值。
	OperatorNEQ Operator = "!="
	// OperatorGTE 表示数字属性值必须大于或等于指定值。
	OperatorGTE Operator = ">="
	// OperatorLTE 表示数字属性值必须小于或等于指定值。
	OperatorLTE Operator = "<="
	// OperatorBelongsTo 表示树形属性属于指定节点或其下级节点。
	OperatorBelongsTo Operator = "belongs_to"
)

// Node 是后端保存和校验访问树的权威结构，不包含任何前端画布坐标。
type Node struct {
	Type      NodeType `json:"type"`
	Children  []Node   `json:"children,omitempty"`
	Attribute string   `json:"attribute,omitempty"`
	Operator  Operator `json:"operator,omitempty"`
	Value     any      `json:"value,omitempty"`
	ValueID   uint64   `json:"valueId,omitempty"`
	ValueCode string   `json:"valueCode,omitempty"`
	Label     string   `json:"label,omitempty"`
	Path      string   `json:"path,omitempty"`
}

// AttributeMeta 是访问树校验所需的最小属性字典信息，避免 policytree 包依赖数据库仓储。
type AttributeMeta struct {
	Code         string
	Type         domain.PolicyAttributeType
	Values       []string
	ValuesByCode map[string]AttributeValueMeta
	Status       domain.PolicyStatus
}

// AttributeValueMeta 是访问树叶子节点稳定值字段的后端校验依据。
type AttributeValueMeta struct {
	ID   uint64
	Code string
	Path string
}

// ValidationError 描述访问树校验失败原因，Path 采用 root.children[0] 形式方便前端定位节点。
type ValidationError struct {
	Path    string
	Message string
}

// Error 将校验错误格式化为可记录的字符串。
func (e ValidationError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

// ValidationErrors 聚合多个访问树错误，让 Service 层能一次性返回可理解的失败原因。
type ValidationErrors []ValidationError

// Error 返回首个校验错误和错误数量，避免响应中暴露过长内部细节。
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return "访问树非法"
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	return fmt.Sprintf("%s 等 %d 个访问树错误", e[0].Error(), len(e))
}

// Parse 将 JSON 原文解析为访问树节点，调用方仍必须继续执行 Validate。
func Parse(raw []byte) (Node, error) {
	var node Node
	if len(raw) == 0 {
		return node, ValidationErrors{{Path: "root", Message: "访问树不能为空"}}
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&node); err != nil {
		return node, ValidationErrors{{Path: "root", Message: "访问树 JSON 格式非法"}}
	}
	return node, nil
}

// MarshalCanonical 将访问树输出为稳定 JSON，便于入库和前端回显。
func MarshalCanonical(node Node) ([]byte, error) {
	return json.Marshal(node)
}
