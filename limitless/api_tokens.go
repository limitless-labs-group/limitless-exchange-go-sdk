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

// MessageResponse is a generic API response carrying a single message field.
type MessageResponse struct {
	Message string `json:"message"`
}

// DeriveToken creates a scoped API token using a Privy identity token.
func (s *ApiTokenService) DeriveToken(ctx context.Context, identityToken string, input DeriveApiTokenInput) (*DeriveApiTokenResponse, error) {
	result, err := s.DeriveTokenWithRawResponse(ctx, identityToken, input)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// DeriveTokenWithRawResponse is the raw-response variant of DeriveToken.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ApiTokenService) DeriveTokenWithRawResponse(ctx context.Context, identityToken string, input DeriveApiTokenInput) (*RawResult[DeriveApiTokenResponse], error) {
	if identityToken == "" {
		return nil, fmt.Errorf("identity token is required for DeriveToken")
	}

	raw, err := s.client.PostRawWithIdentity(ctx, "/auth/api-tokens/derive", identityToken, input)
	return decodeRawResult[DeriveApiTokenResponse](raw, err)
}

// ListTokens lists active tokens for the authenticated partner profile.
func (s *ApiTokenService) ListTokens(ctx context.Context) ([]ApiToken, error) {
	result, err := s.ListTokensWithRawResponse(ctx)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

// ListTokensWithRawResponse is the raw-response variant of ListTokens.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ApiTokenService) ListTokensWithRawResponse(ctx context.Context) (*RawResult[[]ApiToken], error) {
	if err := s.client.requireAuth("ListTokens"); err != nil {
		return nil, err
	}

	raw, err := s.client.GetRaw(ctx, "/auth/api-tokens")
	return decodeRawResult[[]ApiToken](raw, err)
}

// GetCapabilities retrieves self-service capability configuration using a Privy identity token.
func (s *ApiTokenService) GetCapabilities(ctx context.Context, identityToken string) (*PartnerCapabilities, error) {
	result, err := s.GetCapabilitiesWithRawResponse(ctx, identityToken)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetCapabilitiesWithRawResponse is the raw-response variant of GetCapabilities.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ApiTokenService) GetCapabilitiesWithRawResponse(ctx context.Context, identityToken string) (*RawResult[PartnerCapabilities], error) {
	if identityToken == "" {
		return nil, fmt.Errorf("identity token is required for GetCapabilities")
	}

	raw, err := s.client.GetRawWithIdentity(ctx, "/auth/api-tokens/capabilities", identityToken)
	return decodeRawResult[PartnerCapabilities](raw, err)
}

// RevokeToken revokes a token owned by the authenticated partner profile.
func (s *ApiTokenService) RevokeToken(ctx context.Context, tokenID string) (string, error) {
	result, err := s.RevokeTokenWithRawResponse(ctx, tokenID)
	if err != nil {
		return "", err
	}
	return result.Data.Message, nil
}

// RevokeTokenWithRawResponse is the raw-response variant of RevokeToken.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ApiTokenService) RevokeTokenWithRawResponse(ctx context.Context, tokenID string) (*RawResult[MessageResponse], error) {
	if err := s.client.requireAuth("RevokeToken"); err != nil {
		return nil, err
	}

	raw, err := s.client.DeleteRaw(ctx, "/auth/api-tokens/"+url.PathEscape(tokenID))
	return decodeRawResult[MessageResponse](raw, err)
}
