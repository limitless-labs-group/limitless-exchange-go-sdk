package limitless

import (
	"context"
	"fmt"
	"net/url"
)

// ApiTokenService manages partner self-service API tokens.
type ApiTokenService struct {
	client *HttpClient
	logger Logger
}

// ApiTokenServiceOption configures an ApiTokenService.
type ApiTokenServiceOption func(*ApiTokenService)

// WithApiTokenLogger sets the logger for the ApiTokenService.
func WithApiTokenLogger(l Logger) ApiTokenServiceOption {
	return func(s *ApiTokenService) {
		s.logger = l
	}
}

// NewApiTokenService creates a new self-service API token client.
func NewApiTokenService(client *HttpClient, opts ...ApiTokenServiceOption) *ApiTokenService {
	s := &ApiTokenService{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type messageResponse struct {
	Message string `json:"message"`
}

// DeriveToken creates a scoped API token using a Privy identity token.
func (s *ApiTokenService) DeriveToken(ctx context.Context, identityToken string, input DeriveApiTokenInput) (*DeriveApiTokenResponse, error) {
	if identityToken == "" {
		return nil, fmt.Errorf("identity token is required for DeriveToken")
	}

	var resp DeriveApiTokenResponse
	if err := s.client.PostWithIdentity(ctx, "/auth/api-tokens/derive", identityToken, input, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListTokens lists active tokens for the authenticated partner profile.
func (s *ApiTokenService) ListTokens(ctx context.Context) ([]ApiToken, error) {
	if err := s.client.requireAuth("ListTokens"); err != nil {
		return nil, err
	}

	var resp []ApiToken
	if err := s.client.Get(ctx, "/auth/api-tokens", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetCapabilities retrieves self-service capability configuration using a Privy identity token.
func (s *ApiTokenService) GetCapabilities(ctx context.Context, identityToken string) (*PartnerCapabilities, error) {
	if identityToken == "" {
		return nil, fmt.Errorf("identity token is required for GetCapabilities")
	}

	var resp PartnerCapabilities
	if err := s.client.GetWithIdentity(ctx, "/auth/api-tokens/capabilities", identityToken, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RevokeToken revokes a token owned by the authenticated partner profile.
func (s *ApiTokenService) RevokeToken(ctx context.Context, tokenID string) (string, error) {
	if err := s.client.requireAuth("RevokeToken"); err != nil {
		return "", err
	}

	var resp messageResponse
	if err := s.client.Delete(ctx, "/auth/api-tokens/"+url.PathEscape(tokenID), &resp); err != nil {
		return "", err
	}
	return resp.Message, nil
}
