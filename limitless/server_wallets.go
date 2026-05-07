package limitless

import (
	"context"
	"fmt"
	"math/big"
	"regexp"
)

var (
	serverWalletConditionIDRegex = regexp.MustCompile(`^0x[a-fA-F0-9]{64}$`)
	serverWalletIntegerRegex     = regexp.MustCompile(`^[0-9]+$`)
)

const serverWalletHMACOnlyError = "Server wallet redeem/withdraw require HMAC-scoped API token auth; legacy API keys are not supported."

// ServerWalletService manages redeem and withdraw operations for server-managed child wallets.
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

	var resp RedeemServerWalletResponse
	if err := s.client.Post(ctx, "/portfolio/redeem", redeemServerWalletRequest{
		ConditionID: params.ConditionID,
		OnBehalfOf:  params.OnBehalfOf,
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Withdraw submits a server-wallet withdraw request.
func (s *ServerWalletService) Withdraw(ctx context.Context, params WithdrawServerWalletParams) (*WithdrawServerWalletResponse, error) {
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

	var resp WithdrawServerWalletResponse
	if err := s.client.Post(ctx, "/portfolio/withdraw", withdrawServerWalletRequest{
		Amount:      params.Amount,
		OnBehalfOf:  params.OnBehalfOf,
		Token:       optionalStringPtr(params.Token),
		Destination: optionalStringPtr(params.Destination),
	}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
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
