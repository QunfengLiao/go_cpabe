package crypto

// Capability 描述前端动态展示算法所需的非秘密元数据。
type Capability struct {
	Code               string `json:"code"`
	DisplayName        string `json:"display_name"`
	Category           string `json:"category"`
	Version            string `json:"version"`
	AuthorizationType  string `json:"authorization_type"`
	ProtectedKeyFormat string `json:"protected_key_format"`
	ClientRuntime      string `json:"client_runtime"`
}

// ProductCapabilities 返回本期真实可用算法；不得加入占位或模拟 CP-ABE。
func ProductCapabilities() []Capability {
	return []Capability{{
		Code: AlgorithmRSAOAEP256, DisplayName: "RSA + AES", Category: "PUBLIC_KEY",
		Version: AlgorithmVersion1, AuthorizationType: "RSA_RECIPIENT",
		ProtectedKeyFormat: "RSA-OAEP-SHA256-RAW", ClientRuntime: "LOCAL_GO_WORKER",
	}}
}
