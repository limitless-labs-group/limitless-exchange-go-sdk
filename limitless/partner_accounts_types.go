package limitless

// CreatePartnerAccountInput is the request payload for partner account creation.
type CreatePartnerAccountInput struct {
	DisplayName        string `json:"displayName,omitempty"`
	CreateServerWallet *bool  `json:"createServerWallet,omitempty"`
}

// CreatePartnerAccountEOAHeaders contains wallet-ownership verification headers for EOA mode.
type CreatePartnerAccountEOAHeaders struct {
	Account        string
	SigningMessage string
	Signature      string
}

// PartnerAccountResponse is returned after creating a partner-owned profile.
type PartnerAccountResponse struct {
	ProfileID int    `json:"profileId"`
	Account   string `json:"account"`
}
