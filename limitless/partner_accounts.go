package limitless

import (
	"context"
	"fmt"
	"strconv"
)

const partnerAccountDisplayNameMaxLength = 44
const partnerAccountAllowanceHMACOnlyError = "Partner account allowance recovery requires HMAC-scoped API token auth; legacy API keys are not supported."

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

// CheckAllowances checks delegated-trading approval readiness from live chain state
// for a partner-created server wallet profile.
func (s *PartnerAccountService) CheckAllowances(ctx context.Context, profileID int) (*PartnerAccountAllowanceResponse, error) {
	if err := s.requireAllowanceHMACAuth("CheckPartnerAccountAllowances"); err != nil {
		return nil, err
	}
	path, err := partnerAccountAllowancesPath(profileID)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Checking partner account allowances", map[string]any{"profileId": profileID})

	var resp PartnerAccountAllowanceResponse
	if err := s.client.Get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RetryAllowances re-checks live chain state and retries delegated-trading approvals
// that are still missing for a partner-created server wallet profile.
// Submitted targets in the response mean this retry request submitted a sponsored
// transaction or user operation; call CheckAllowances again after a short delay to
// observe confirmed chain state.
func (s *PartnerAccountService) RetryAllowances(ctx context.Context, profileID int) (*PartnerAccountAllowanceResponse, error) {
	if err := s.requireAllowanceHMACAuth("RetryPartnerAccountAllowances"); err != nil {
		return nil, err
	}
	path, err := partnerAccountAllowancesPath(profileID)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Retrying partner account allowances", map[string]any{"profileId": profileID})

	var resp PartnerAccountAllowanceResponse
	if err := s.client.Post(ctx, path+"/retry", struct{}{}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *PartnerAccountService) requireAllowanceHMACAuth(operation string) error {
	if err := s.client.requireAuth(operation); err != nil {
		return err
	}
	if s.client.HMACCredentials() == nil {
		return fmt.Errorf(partnerAccountAllowanceHMACOnlyError)
	}
	return nil
}

func partnerAccountAllowancesPath(profileID int) (string, error) {
	if profileID <= 0 {
		return "", fmt.Errorf("ProfileID must be a positive integer")
	}
	return "/profiles/partner-accounts/" + strconv.Itoa(profileID) + "/allowances", nil
}
