package models

import "time"

// LLMModelCustomHeader represents one custom HTTP header to attach to provider requests.
// Value may contain `{{VAR}}` placeholders resolved at request time against the active
// OpenClaw instance (e.g. {{INSTANCE_ID}}, {{INSTANCE_NAME}}, {{ACCESS_TOKEN}}).
type LLMModelCustomHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// LLMModel stores an admin-managed AI model configuration.
type LLMModel struct {
	ID                int                    `db:"id,primarykey,autoincrement" json:"id"`
	DisplayName       string                 `db:"display_name" json:"display_name"`
	Description       *string                `db:"description" json:"description,omitempty"`
	ProviderType      string                 `db:"provider_type" json:"provider_type"`
	ProtocolType      string                 `db:"protocol_type" json:"protocol_type,omitempty"`
	BaseURL           string                 `db:"base_url" json:"base_url"`
	ProviderModelName string                 `db:"provider_model_name" json:"provider_model_name"`
	APIKey            *string                `db:"api_key" json:"api_key,omitempty"`
	APIKeySecretRef   *string                `db:"api_key_secret_ref" json:"api_key_secret_ref,omitempty"`
	IsSecure          bool                   `db:"is_secure" json:"is_secure"`
	IsActive          bool                   `db:"is_active" json:"is_active"`
	InputPrice        float64                `db:"input_price" json:"input_price"`
	OutputPrice       float64                `db:"output_price" json:"output_price"`
	Currency          string                 `db:"currency" json:"currency"`
	CustomHeadersJSON *string                `db:"custom_headers_json" json:"-"`
	CustomHeaders     []LLMModelCustomHeader `db:"-" json:"custom_headers,omitempty"`
	CreatedAt         time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time              `db:"updated_at" json:"updated_at"`
}

// TableName returns the table name for the LLM model.
func (m LLMModel) TableName() string {
	return "llm_models"
}
