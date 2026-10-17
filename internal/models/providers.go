package models

type ConnectProviderReq struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

type ConnectProviderRes struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

type ProviderItem struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
}

type ListProvidersRes struct {
	Providers []ProviderItem `json:"providers"`
}
