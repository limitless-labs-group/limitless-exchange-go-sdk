package limitless

// DeriveApiTokenInput is the request payload for self-service token creation.
type DeriveApiTokenInput struct {
	Label  string   `json:"label,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

// ApiTokenProfile identifies the profile associated with an API token response.
type ApiTokenProfile struct {
	ID      int    `json:"id"`
	Account string `json:"account"`
}

// DeriveApiTokenResponse is the one-time response from token creation.
type DeriveApiTokenResponse struct {
	ApiKey    string          `json:"apiKey"`
	Secret    string          `json:"secret"`
	TokenID   string          `json:"tokenId"`
	CreatedAt string          `json:"createdAt"`
	Scopes    []string        `json:"scopes"`
	Profile   ApiTokenProfile `json:"profile"`
}

// ApiToken represents an active token in list responses.
type ApiToken struct {
	TokenID    string   `json:"tokenId"`
	Label      *string  `json:"label"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"createdAt"`
	LastUsedAt *string  `json:"lastUsedAt"`
}

// PartnerCapabilities is the self-service capability config for a partner.
type PartnerCapabilities struct {
	PartnerProfileID       int      `json:"partnerProfileId"`
	TokenManagementEnabled bool     `json:"tokenManagementEnabled"`
	AllowedScopes          []string `json:"allowedScopes"`
}

const (
	ScopeTrading          = "trading"
	ScopeAccountCreation  = "account_creation"
	ScopeDelegatedSigning = "delegated_signing"
)
