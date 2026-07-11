package model

// HealthResponse 是健康检查接口的完整响应，包含应用和依赖状态。
type HealthResponse struct {
	Status    string           `json:"status"`
	CheckedAt string           `json:"checkedAt"`
	App       AppHealth        `json:"app"`
	MySQL     DependencyHealth `json:"mysql"`
	Redis     DependencyHealth `json:"redis"`
}

// AppHealth 描述当前应用进程自身状态。
type AppHealth struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

// DependencyHealth 描述 MySQL、Redis 等外部依赖的状态和脱敏错误信息。
type DependencyHealth struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Metrics map[string]any `json:"metrics,omitempty"`
}
