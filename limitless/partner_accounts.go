package limitless

import (
	"context"
	"fmt"
)

const partnerAccountDisplayNameMaxLength = 44

// PartnerAccountService manages partner-owned profile creation.
type PartnerAccountService struct {
	client *HttpClient
	logger Logger
}

// PartnerAccountServiceOption configures a PartnerAccountService.
type PartnerAccountServiceOption func(*PartnerAccountService)

// WithPartnerAccountLogger sets the logger for the PartnerAccountService.
func WithPartnerAccountLogger(l Logger) PartnerAccountServiceOption {
	return func(s *PartnerAccountService) {
		s.logger = l
	}
}

// NewPartnerAccountService creates a new partner account service.
func NewPartnerAccountService(client *HttpClient, opts ...PartnerAccountServiceOption) *PartnerAccountService {
	s := &PartnerAccountService{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateAccount creates a partner-owned profile using either server-wallet or EOA verification mode.
func (s *PartnerAccountService) CreateAccount(ctx context.Context, input CreatePartnerAccountInput, eoaHeaders *CreatePartnerAccountEOAHeaders) (*PartnerAccountResponse, error) {
	if err := s.client.requireAuth("CreatePartnerAccount"); err != nil {
		return nil, err
	}

	serverWalletMode := input.CreateServerWallet != nil && *input.CreateServerWallet
	if !serverWalletMode && eoaHeaders == nil {
		return nil, fmt.Errorf("EOA headers are required when CreateServerWallet is not true")
	}
	if input.DisplayName != "" && len(input.DisplayName) > partnerAccountDisplayNameMaxLength {
		return nil, fmt.Errorf("displayName must be at most %d characters", partnerAccountDisplayNameMaxLength)
	}

	headers := map[string]string{}
	if eoaHeaders != nil {
		headers["x-account"] = eoaHeaders.Account
		headers["x-signing-message"] = eoaHeaders.SigningMessage
		headers["x-signature"] = eoaHeaders.Signature
	}

	var resp PartnerAccountResponse
	if err := s.client.PostWithHeaders(ctx, "/profiles/partner-accounts", input, headers, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
