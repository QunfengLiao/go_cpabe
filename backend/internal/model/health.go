package model

type HealthResponse struct {
	Status    string           `json:"status"`
	CheckedAt string           `json:"checkedAt"`
	App       AppHealth        `json:"app"`
	MySQL     DependencyHealth `json:"mysql"`
	Redis     DependencyHealth `json:"redis"`
}

type AppHealth struct {
	Status string `json:"status"`
	Env    string `json:"env"`
}

type DependencyHealth struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
