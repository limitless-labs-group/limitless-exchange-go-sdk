package limitless

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var (
	serverWalletConditionIDRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{64}$`)
	serverWalletIntegerRegex     = regexp.MustCompile(`^[0-9]+$`)
)

const serverWalletHMACOnlyError = "Server wallet operations require HMAC-scoped API token auth; legacy API keys are not supported."

// ServerWalletService manages server-wallet portfolio operations.
type ServerWalletService struct {
	client *HttpClient
	logger Logger
}

// ServerWalletServiceOption configures a ServerWalletService.
type ServerWalletServiceOption func(*ServerWalletService)

// WithServerWalletLogger sets the logger for the ServerWalletService.
func WithServerWalletLogger(l Logger) ServerWalletServiceOption {
	return func(s *ServerWalletService) {
		s.logger = l
	}
}

// NewServerWalletService creates a new server-wallet service.
func NewServerWalletService(client *HttpClient, opts ...ServerWalletServiceOption) *ServerWalletService {
	s := &ServerWalletService{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// RedeemPositions submits a server-wallet redeem request for a resolved market condition.
func (s *ServerWalletService) RedeemPositions(ctx context.Context, params RedeemServerWalletParams) (*RedeemServerWalletResponse, error) {
	result, err := s.RedeemPositionsWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// RedeemPositionsWithRawResponse is the raw-response variant of RedeemPositions.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ServerWalletService) RedeemPositionsWithRawResponse(ctx context.Context, params RedeemServerWalletParams) (*RawResult[RedeemServerWalletResponse], error) {
	if err := s.requireHMACAuth("RedeemServerWalletPositions"); err != nil {
		return nil, err
	}
	if err := validateServerWalletConditionID(params.ConditionID); err != nil {
		return nil, err
	}
	if err := validateServerWalletOnBehalfOf(params.OnBehalfOf); err != nil {
		return nil, err
	}

	s.logger.Debug("Redeeming server-wallet positions", map[string]any{
		"conditionId": params.ConditionID,
		"onBehalfOf":  params.OnBehalfOf,
	})

	raw, err := s.client.PostRaw(ctx, "/portfolio/redeem", redeemServerWalletRequest{
		ConditionID: params.ConditionID,
		OnBehalfOf:  params.OnBehalfOf,
	})
	return decodeRawResult[RedeemServerWalletResponse](raw, err)
}

// SplitPositions submits a server-wallet split request for a CLOB or NegRisk market.
func (s *ServerWalletService) SplitPositions(ctx context.Context, params SplitServerWalletParams) (*SplitServerWalletResponse, error) {
	result, err := s.SplitPositionsWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// SplitPositionsWithRawResponse is the raw-response variant of SplitPositions.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ServerWalletService) SplitPositionsWithRawResponse(ctx context.Context, params SplitServerWalletParams) (*RawResult[SplitServerWalletResponse], error) {
	if err := s.requireHMACAuth("SplitServerWalletPositions"); err != nil {
		return nil, err
	}
	request, err := buildSplitMergeServerWalletRequest(params.ConditionID, params.Amount, params.Venue, params.OnBehalfOf)
	if err != nil {
		return nil, err
	}

	logMeta := map[string]any{
		"conditionId": request.ConditionID,
		"amount":      request.Amount,
		"onBehalfOf":  request.OnBehalfOf,
	}
	if request.Venue != nil {
		if request.Venue.Exchange != "" {
			logMeta["venue.exchange"] = request.Venue.Exchange
		}
		if request.Venue.Adapter != "" {
			logMeta["venue.adapter"] = request.Venue.Adapter
		}
	}
	s.logger.Debug("Splitting server-wallet positions", logMeta)

	raw, err := s.client.PostRaw(ctx, "/portfolio/split", request)
	return decodeRawResult[SplitServerWalletResponse](raw, err)
}

// MergePositions submits a server-wallet merge request for a CLOB or NegRisk market.
func (s *ServerWalletService) MergePositions(ctx context.Context, params MergeServerWalletParams) (*MergeServerWalletResponse, error) {
	result, err := s.MergePositionsWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// MergePositionsWithRawResponse is the raw-response variant of MergePositions.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ServerWalletService) MergePositionsWithRawResponse(ctx context.Context, params MergeServerWalletParams) (*RawResult[MergeServerWalletResponse], error) {
	if err := s.requireHMACAuth("MergeServerWalletPositions"); err != nil {
		return nil, err
	}
	request, err := buildSplitMergeServerWalletRequest(params.ConditionID, params.Amount, params.Venue, params.OnBehalfOf)
	if err != nil {
		return nil, err
	}

	logMeta := map[string]any{
		"conditionId": request.ConditionID,
		"amount":      request.Amount,
		"onBehalfOf":  request.OnBehalfOf,
	}
	if request.Venue != nil {
		if request.Venue.Exchange != "" {
			logMeta["venue.exchange"] = request.Venue.Exchange
		}
		if request.Venue.Adapter != "" {
			logMeta["venue.adapter"] = request.Venue.Adapter
		}
	}
	s.logger.Debug("Merging server-wallet positions", logMeta)

	raw, err := s.client.PostRaw(ctx, "/portfolio/merge", request)
	return decodeRawResult[MergeServerWalletResponse](raw, err)
}

// Withdraw submits a server-wallet withdraw request.
func (s *ServerWalletService) Withdraw(ctx context.Context, params WithdrawServerWalletParams) (*WithdrawServerWalletResponse, error) {
	result, err := s.WithdrawWithRawResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// WithdrawWithRawResponse is the raw-response variant of Withdraw.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *ServerWalletService) WithdrawWithRawResponse(ctx context.Context, params WithdrawServerWalletParams) (*RawResult[WithdrawServerWalletResponse], error) {
	if err := s.requireHMACAuth("WithdrawServerWalletFunds"); err != nil {
		return nil, err
	}
	if err := validateServerWalletAmount(params.Amount); err != nil {
		return nil, err
	}
	if params.OnBehalfOf != 0 {
		if err := validateServerWalletOnBehalfOf(params.OnBehalfOf); err != nil {
			return nil, err
		}
	}
	if params.Token != "" && !isValidAddress(params.Token) {
		return nil, fmt.Errorf("Token must be a valid EVM address")
	}
	if params.Destination != "" && !isValidAddress(params.Destination) {
		return nil, fmt.Errorf("Destination must be a valid EVM address")
	}
	if params.OnBehalfOf == 0 && params.Destination == "" {
		return nil, fmt.Errorf("OnBehalfOf or Destination is required for withdraw")
	}

	s.logger.Debug("Withdrawing from server wallet", map[string]any{
		"amount":      params.Amount,
		"onBehalfOf":  params.OnBehalfOf,
		"token":       params.Token,
		"destination": params.Destination,
	})

	raw, err := s.client.PostRaw(ctx, "/portfolio/withdraw", withdrawServerWalletRequest{
		Amount:      params.Amount,
		OnBehalfOf:  params.OnBehalfOf,
		Token:       optionalStringPtr(params.Token),
		Destination: optionalStringPtr(params.Destination),
	})
	return decodeRawResult[WithdrawServerWalletResponse](raw, err)
}

func buildSplitMergeServerWalletRequest(
	conditionID string,
	amount string,
	venue *ServerWalletVenue,
	onBehalfOf int,
) (splitMergeServerWalletRequest, error) {
	conditionID = strings.TrimSpace(conditionID)
	amount = strings.TrimSpace(amount)

	if conditionID == "" {
		return splitMergeServerWalletRequest{}, fmt.Errorf("ConditionID is required")
	}
	if err := validateServerWalletConditionID(conditionID); err != nil {
		return splitMergeServerWalletRequest{}, err
	}
	if err := validateServerWalletAmount(amount); err != nil {
		return splitMergeServerWalletRequest{}, err
	}
	if onBehalfOf < 0 {
		return splitMergeServerWalletRequest{}, fmt.Errorf("OnBehalfOf must be a positive integer")
	}
	requestVenue, err := buildSplitMergeServerWalletVenue(venue)
	if err != nil {
		return splitMergeServerWalletRequest{}, err
	}

	return splitMergeServerWalletRequest{
		ConditionID: conditionID,
		Amount:      amount,
		Venue:       requestVenue,
		OnBehalfOf:  onBehalfOf,
	}, nil
}

func buildSplitMergeServerWalletVenue(venue *ServerWalletVenue) (*ServerWalletVenue, error) {
	if venue == nil {
		return nil, fmt.Errorf("Venue is required")
	}

	exchange := strings.TrimSpace(venue.Exchange)
	adapter := strings.TrimSpace(venue.Adapter)
	if exchange != "" && !isValidAddress(exchange) {
		return nil, fmt.Errorf("Venue.Exchange must be a valid EVM address")
	}
	if adapter != "" && !isValidAddress(adapter) {
		return nil, fmt.Errorf("Venue.Adapter must be a valid EVM address")
	}
	if adapter == "" && exchange == "" {
		return nil, fmt.Errorf("Venue.Exchange is required when Venue.Adapter is not provided")
	}

	return &ServerWalletVenue{Exchange: exchange, Adapter: adapter}, nil
}

func (s *ServerWalletService) requireHMACAuth(operation string) error {
	if err := s.client.requireAuth(operation); err != nil {
		return err
	}
	if s.client.HMACCredentials() == nil {
		return fmt.Errorf(serverWalletHMACOnlyError)
	}
	return nil
}

func validateServerWalletConditionID(conditionID string) error {
	if !serverWalletConditionIDRegex.MatchString(conditionID) {
		return fmt.Errorf("ConditionID must be a 0x-prefixed 32-byte hex string")
	}
	return nil
}

func validateServerWalletOnBehalfOf(onBehalfOf int) error {
	if onBehalfOf <= 0 {
		return fmt.Errorf("OnBehalfOf must be a positive integer")
	}
	return nil
}

func validateServerWalletAmount(amount string) error {
	if !serverWalletIntegerRegex.MatchString(amount) {
		return fmt.Errorf("Amount must be a positive integer string in the token smallest unit")
	}

	value, ok := new(big.Int).SetString(amount, 10)
	if !ok || value.Sign() <= 0 {
		return fmt.Errorf("Amount must be a positive integer string in the token smallest unit")
	}
	return nil
}

func optionalStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	v := value
	return &v
}
