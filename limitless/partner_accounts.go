package limitless

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const partnerAccountDisplayNameMaxLength = 44
const partnerAccountAllowanceHMACOnlyError = "Partner account allowance recovery requires HMAC-scoped API token auth; legacy API keys are not supported."
const partnerAccountListHMACOnlyError = "Partner account listing requires HMAC-scoped API token auth; legacy API keys are not supported."
const partnerAccountsMaxLimit = 25

// PartnerAccountService manages partner-owned profiles and partner account operations.
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

// ListAccounts lists partner-owned accounts, or recovers a specific account by
// address when params.Account is provided.
func (s *PartnerAccountService) ListAccounts(ctx context.Context, params ListPartnerAccountsParams) (*ListPartnerAccountsResponse, error) {
	if err := s.requireHMACAuth("ListPartnerAccounts", partnerAccountListHMACOnlyError); err != nil {
		return nil, err
	}
	path, err := partnerAccountsPath(params)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Listing partner accounts", map[string]any{
		"account": params.Account,
		"limit":   params.Limit,
		"page":    params.Page,
	})

	var resp ListPartnerAccountsResponse
	if err := s.client.Get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CheckAllowances checks delegated-trading approval readiness from live chain state
// for a partner-created server wallet profile.
func (s *PartnerAccountService) CheckAllowances(ctx context.Context, profileID int) (*PartnerAccountAllowanceResponse, error) {
	if err := s.requireHMACAuth("CheckPartnerAccountAllowances", partnerAccountAllowanceHMACOnlyError); err != nil {
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
	if err := s.requireHMACAuth("RetryPartnerAccountAllowances", partnerAccountAllowanceHMACOnlyError); err != nil {
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

// AddWithdrawalAddress adds an active partner withdrawal destination allowlist
// entry using a Privy identity token. API-token auth is not used for this endpoint.
func (s *PartnerAccountService) AddWithdrawalAddress(ctx context.Context, identityToken string, input PartnerWithdrawalAddressInput) (*PartnerWithdrawalAddressResponse, error) {
	if identityToken == "" {
		return nil, fmt.Errorf("identity token is required for AddWithdrawalAddress")
	}
	if input.Address == "" {
		return nil, fmt.Errorf("address is required for AddWithdrawalAddress")
	}

	s.logger.Debug("Adding partner withdrawal address", map[string]any{"address": input.Address})

	var resp PartnerWithdrawalAddressResponse
	if err := s.client.PostWithIdentity(ctx, "/portfolio/withdrawal-addresses", identityToken, input, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteWithdrawalAddress removes a partner withdrawal destination allowlist
// entry using a Privy identity token. API-token auth is not used for this endpoint.
func (s *PartnerAccountService) DeleteWithdrawalAddress(ctx context.Context, identityToken string, address string) error {
	if identityToken == "" {
		return fmt.Errorf("identity token is required for DeleteWithdrawalAddress")
	}
	if address == "" {
		return fmt.Errorf("address is required for DeleteWithdrawalAddress")
	}

	s.logger.Debug("Deleting partner withdrawal address", map[string]any{"address": address})

	return s.client.DeleteWithIdentity(ctx, "/portfolio/withdrawal-addresses/"+url.PathEscape(address), identityToken, nil)
}

func (s *PartnerAccountService) requireHMACAuth(operation string, errorMessage string) error {
	if err := s.client.requireAuth(operation); err != nil {
		return err
	}
	if s.client.HMACCredentials() == nil {
		return errors.New(errorMessage)
	}
	return nil
}

func partnerAccountsPath(params ListPartnerAccountsParams) (string, error) {
	query := url.Values{}

	if params.Account != "" {
		account := strings.TrimSpace(params.Account)
		if account == "" {
			return "", fmt.Errorf("Account must be a non-empty string")
		}
		query.Set("account", account)
	}

	if params.Limit < 0 {
		return "", fmt.Errorf("Limit must be zero or a positive integer")
	}
	if params.Limit > 0 {
		limit := params.Limit
		if limit > partnerAccountsMaxLimit {
			limit = partnerAccountsMaxLimit
		}
		query.Set("limit", strconv.Itoa(limit))
	}

	if params.Page < 0 {
		return "", fmt.Errorf("Page must be zero or a positive integer")
	}
	if params.Page > 0 {
		query.Set("page", strconv.Itoa(params.Page))
	}

	encoded := query.Encode()
	if encoded == "" {
		return "/profiles/partner-accounts", nil
	}
	return "/profiles/partner-accounts?" + encoded, nil
}

func partnerAccountAllowancesPath(profileID int) (string, error) {
	if profileID <= 0 {
		return "", fmt.Errorf("ProfileID must be a positive integer")
	}
	return "/profiles/partner-accounts/" + strconv.Itoa(profileID) + "/allowances", nil
}
