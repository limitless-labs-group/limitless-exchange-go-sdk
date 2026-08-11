package limitless

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ammHMACOnlyError                = "AMM operations require HMAC-scoped API token auth or an explicit Privy identity token; legacy API keys are not supported."
	defaultAMMAllowancePollInterval = 2 * time.Second
	ammIdempotencyKeyMaxLength      = 128
	ammMarketMaxLength              = 255
	ammAmountMaxLength              = 78
	ammMaxOnBehalfOf                = 2147483647
)

var (
	ammPositiveIntegerRegex = regexp.MustCompile(`^[1-9][0-9]*$`)
	ammMaxUint256           = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
)

// AMMService manages AMM market approvals and server-wallet buy/sell submissions.
type AMMService struct {
	client *HttpClient
	logger Logger
}

// AMMServiceOption configures an AMMService.
type AMMServiceOption func(*AMMService)

// WithAMMLogger sets the logger for the AMMService.
func WithAMMLogger(l Logger) AMMServiceOption {
	return func(s *AMMService) {
		s.logger = l
	}
}

// NewAMMService creates a new AMM service.
func NewAMMService(client *HttpClient, opts ...AMMServiceOption) *AMMService {
	s := &AMMService{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CheckAllowance reads the live BUY or SELL approval state using configured HMAC auth.
func (s *AMMService) CheckAllowance(ctx context.Context, params AMMAllowanceParams) (*AMMAllowanceResponse, error) {
	if err := s.requireHMACAuth("CheckAMMAllowance"); err != nil {
		return nil, err
	}
	return s.checkAllowance(ctx, params, "")
}

// CheckAllowanceWithRawResponse is the raw-response variant of CheckAllowance.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (s *AMMService) CheckAllowanceWithRawResponse(ctx context.Context, params AMMAllowanceParams) (*RawResult[AMMAllowanceResponse], error) {
	if err := s.requireHMACAuth("CheckAMMAllowance"); err != nil {
		return nil, err
	}
	return s.checkAllowanceRaw(ctx, params, "")
}

// CheckAllowanceWithIdentity reads the live BUY or SELL approval state using Privy identity auth.
func (s *AMMService) CheckAllowanceWithIdentity(ctx context.Context, identityToken string, params AMMAllowanceParams) (*AMMAllowanceResponse, error) {
	result, err := s.CheckAllowanceWithIdentityWithRawResponse(ctx, identityToken, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// CheckAllowanceWithIdentityWithRawResponse is the raw-response variant of CheckAllowanceWithIdentity.
func (s *AMMService) CheckAllowanceWithIdentityWithRawResponse(ctx context.Context, identityToken string, params AMMAllowanceParams) (*RawResult[AMMAllowanceResponse], error) {
	identityToken, err := requireAMMIdentityToken(identityToken, "CheckAMMAllowance")
	if err != nil {
		return nil, err
	}
	return s.checkAllowanceRaw(ctx, params, identityToken)
}

// ApproveAllowance submits a missing BUY or SELL approval using configured HMAC auth.
// A submitted response is not confirmation; poll CheckAllowance until Confirmed is true.
func (s *AMMService) ApproveAllowance(ctx context.Context, params AMMAllowanceParams) (*AMMAllowanceResponse, error) {
	if err := s.requireHMACAuth("ApproveAMMAllowance"); err != nil {
		return nil, err
	}
	return s.approveAllowance(ctx, params, "")
}

// ApproveAllowanceWithRawResponse is the raw-response variant of ApproveAllowance.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
// Approvals land as 200 (already confirmed) or 202 (newly submitted); both are 2xx.
func (s *AMMService) ApproveAllowanceWithRawResponse(ctx context.Context, params AMMAllowanceParams) (*RawResult[AMMAllowanceResponse], error) {
	if err := s.requireHMACAuth("ApproveAMMAllowance"); err != nil {
		return nil, err
	}
	return s.approveAllowanceRaw(ctx, params, "")
}

// ApproveAllowanceWithIdentity submits a missing BUY or SELL approval using Privy identity auth.
// A submitted response is not confirmation; poll CheckAllowanceWithIdentity until Confirmed is true.
func (s *AMMService) ApproveAllowanceWithIdentity(ctx context.Context, identityToken string, params AMMAllowanceParams) (*AMMAllowanceResponse, error) {
	result, err := s.ApproveAllowanceWithIdentityWithRawResponse(ctx, identityToken, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// ApproveAllowanceWithIdentityWithRawResponse is the raw-response variant of ApproveAllowanceWithIdentity.
func (s *AMMService) ApproveAllowanceWithIdentityWithRawResponse(ctx context.Context, identityToken string, params AMMAllowanceParams) (*RawResult[AMMAllowanceResponse], error) {
	identityToken, err := requireAMMIdentityToken(identityToken, "ApproveAMMAllowance")
	if err != nil {
		return nil, err
	}
	return s.approveAllowanceRaw(ctx, params, identityToken)
}

// EnsureAllowance checks an allowance, approves it at most once when missing,
// and then polls allowance check until confirmation using configured HMAC auth.
// Buy and Sell never call this workflow automatically.
func (s *AMMService) EnsureAllowance(ctx context.Context, params AMMAllowanceParams, options ...AMMAllowancePollOptions) (*AMMAllowanceResponse, error) {
	if err := s.requireHMACAuth("EnsureAMMAllowance"); err != nil {
		return nil, err
	}
	interval, err := resolveAMMAllowancePollInterval(options)
	if err != nil {
		return nil, err
	}
	return s.ensureAllowance(ctx, params, interval, "")
}

// EnsureAllowanceWithIdentity checks an allowance, approves it at most once when
// missing, and then polls allowance check until confirmation using Privy identity auth.
// Buy and Sell never call this workflow automatically.
func (s *AMMService) EnsureAllowanceWithIdentity(
	ctx context.Context,
	identityToken string,
	params AMMAllowanceParams,
	options ...AMMAllowancePollOptions,
) (*AMMAllowanceResponse, error) {
	identityToken, err := requireAMMIdentityToken(identityToken, "EnsureAMMAllowance")
	if err != nil {
		return nil, err
	}
	interval, err := resolveAMMAllowancePollInterval(options)
	if err != nil {
		return nil, err
	}
	return s.ensureAllowance(ctx, params, interval, identityToken)
}

// Buy submits an exact-collateral AMM buy using configured HMAC auth.
// It does not check or submit allowances. Reuse the same params when retrying so
// the serialized body and idempotency key remain unchanged.
func (s *AMMService) Buy(ctx context.Context, params AMMBuyParams) (*AMMBuyResponse, error) {
	if err := s.requireHMACAuth("BuyAMMShares"); err != nil {
		return nil, err
	}
	return s.buy(ctx, params, "")
}

// BuyWithRawResponse is the raw-response variant of Buy. It returns the decoded
// value alongside the full HTTP response (status, headers, body). A submitted buy
// lands as 201.
func (s *AMMService) BuyWithRawResponse(ctx context.Context, params AMMBuyParams) (*RawResult[AMMBuyResponse], error) {
	if err := s.requireHMACAuth("BuyAMMShares"); err != nil {
		return nil, err
	}
	return s.buyRaw(ctx, params, "")
}

// BuyWithIdentity submits an exact-collateral AMM buy using Privy identity auth.
// It does not check or submit allowances. Reuse the same params when retrying so
// the serialized body and idempotency key remain unchanged.
func (s *AMMService) BuyWithIdentity(ctx context.Context, identityToken string, params AMMBuyParams) (*AMMBuyResponse, error) {
	result, err := s.BuyWithIdentityWithRawResponse(ctx, identityToken, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// BuyWithIdentityWithRawResponse is the raw-response variant of BuyWithIdentity.
func (s *AMMService) BuyWithIdentityWithRawResponse(ctx context.Context, identityToken string, params AMMBuyParams) (*RawResult[AMMBuyResponse], error) {
	identityToken, err := requireAMMIdentityToken(identityToken, "BuyAMMShares")
	if err != nil {
		return nil, err
	}
	return s.buyRaw(ctx, params, identityToken)
}

// Sell submits an exact-collateral-return AMM sell using configured HMAC auth.
// It does not check or submit allowances. Reuse the same params when retrying so
// the serialized body and idempotency key remain unchanged.
func (s *AMMService) Sell(ctx context.Context, params AMMSellParams) (*AMMSellResponse, error) {
	if err := s.requireHMACAuth("SellAMMShares"); err != nil {
		return nil, err
	}
	return s.sell(ctx, params, "")
}

// SellWithRawResponse is the raw-response variant of Sell. It returns the decoded
// value alongside the full HTTP response (status, headers, body). A submitted sell
// lands as 201.
func (s *AMMService) SellWithRawResponse(ctx context.Context, params AMMSellParams) (*RawResult[AMMSellResponse], error) {
	if err := s.requireHMACAuth("SellAMMShares"); err != nil {
		return nil, err
	}
	return s.sellRaw(ctx, params, "")
}

// SellWithIdentity submits an exact-collateral-return AMM sell using Privy identity auth.
// It does not check or submit allowances. Reuse the same params when retrying so
// the serialized body and idempotency key remain unchanged.
func (s *AMMService) SellWithIdentity(ctx context.Context, identityToken string, params AMMSellParams) (*AMMSellResponse, error) {
	result, err := s.SellWithIdentityWithRawResponse(ctx, identityToken, params)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// SellWithIdentityWithRawResponse is the raw-response variant of SellWithIdentity.
func (s *AMMService) SellWithIdentityWithRawResponse(ctx context.Context, identityToken string, params AMMSellParams) (*RawResult[AMMSellResponse], error) {
	identityToken, err := requireAMMIdentityToken(identityToken, "SellAMMShares")
	if err != nil {
		return nil, err
	}
	return s.sellRaw(ctx, params, identityToken)
}

func (s *AMMService) checkAllowance(ctx context.Context, params AMMAllowanceParams, identityToken string) (*AMMAllowanceResponse, error) {
	result, err := s.checkAllowanceRaw(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *AMMService) checkAllowanceRaw(ctx context.Context, params AMMAllowanceParams, identityToken string) (*RawResult[AMMAllowanceResponse], error) {
	request, err := buildAMMAllowanceRequest(params)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Checking AMM allowance", map[string]any{
		"market":     request.Market,
		"side":       request.Side,
		"onBehalfOf": request.OnBehalfOf,
	})

	raw, err := s.postRaw(ctx, "/amm/allowances/check", request, identityToken)
	return decodeRawResult[AMMAllowanceResponse](raw, err)
}

func (s *AMMService) approveAllowance(ctx context.Context, params AMMAllowanceParams, identityToken string) (*AMMAllowanceResponse, error) {
	result, err := s.approveAllowanceRaw(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *AMMService) approveAllowanceRaw(ctx context.Context, params AMMAllowanceParams, identityToken string) (*RawResult[AMMAllowanceResponse], error) {
	request, err := buildAMMAllowanceRequest(params)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Approving AMM allowance", map[string]any{
		"market":     request.Market,
		"side":       request.Side,
		"onBehalfOf": request.OnBehalfOf,
	})

	raw, err := s.postRaw(ctx, "/amm/allowances/approve", request, identityToken)
	return decodeRawResult[AMMAllowanceResponse](raw, err)
}

func (s *AMMService) ensureAllowance(
	ctx context.Context,
	params AMMAllowanceParams,
	interval time.Duration,
	identityToken string,
) (*AMMAllowanceResponse, error) {
	checked, err := s.checkAllowance(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	if checked.Confirmed {
		return checked, nil
	}

	approved, err := s.approveAllowance(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	if approved.Confirmed {
		return approved, nil
	}

	for {
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return approved, ctx.Err()
		case <-timer.C:
		}

		checked, err = s.checkAllowance(ctx, params, identityToken)
		if err != nil {
			return nil, err
		}
		if checked.Confirmed {
			return checked, nil
		}
	}
}

func (s *AMMService) buy(ctx context.Context, params AMMBuyParams, identityToken string) (*AMMBuyResponse, error) {
	result, err := s.buyRaw(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *AMMService) buyRaw(ctx context.Context, params AMMBuyParams, identityToken string) (*RawResult[AMMBuyResponse], error) {
	request, err := buildAMMBuyRequest(params)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Buying AMM shares", map[string]any{
		"market":       request.Market,
		"outcomeIndex": request.OutcomeIndex,
		"onBehalfOf":   request.OnBehalfOf,
	})

	raw, err := s.postRaw(ctx, "/amm/buy", request, identityToken)
	return decodeRawResult[AMMBuyResponse](raw, err)
}

func (s *AMMService) sell(ctx context.Context, params AMMSellParams, identityToken string) (*AMMSellResponse, error) {
	result, err := s.sellRaw(ctx, params, identityToken)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

func (s *AMMService) sellRaw(ctx context.Context, params AMMSellParams, identityToken string) (*RawResult[AMMSellResponse], error) {
	request, err := buildAMMSellRequest(params)
	if err != nil {
		return nil, err
	}

	s.logger.Debug("Selling AMM shares", map[string]any{
		"market":       request.Market,
		"outcomeIndex": request.OutcomeIndex,
		"onBehalfOf":   request.OnBehalfOf,
	})

	raw, err := s.postRaw(ctx, "/amm/sell", request, identityToken)
	return decodeRawResult[AMMSellResponse](raw, err)
}

func (s *AMMService) postRaw(ctx context.Context, path string, body any, identityToken string) (*RawResponse, error) {
	if identityToken != "" {
		return s.client.PostRawWithIdentity(ctx, path, identityToken, body)
	}
	return s.client.PostRaw(ctx, path, body)
}

func (s *AMMService) requireHMACAuth(operation string) error {
	if err := s.client.requireAuth(operation); err != nil {
		return err
	}
	if s.client.HMACCredentials() == nil {
		return errors.New(ammHMACOnlyError)
	}
	return nil
}

func requireAMMIdentityToken(identityToken string, operation string) (string, error) {
	identityToken = strings.TrimSpace(identityToken)
	if identityToken == "" {
		return "", fmt.Errorf("identity token is required for %s", operation)
	}
	return identityToken, nil
}

func resolveAMMAllowancePollInterval(options []AMMAllowancePollOptions) (time.Duration, error) {
	if len(options) > 1 {
		return 0, fmt.Errorf("only one AMMAllowancePollOptions value may be provided")
	}
	if len(options) == 0 || options[0].Interval == 0 {
		return defaultAMMAllowancePollInterval, nil
	}
	if options[0].Interval < 0 {
		return 0, fmt.Errorf("allowance poll interval must be positive")
	}
	return options[0].Interval, nil
}

func buildAMMAllowanceRequest(params AMMAllowanceParams) (ammAllowanceRequest, error) {
	market, err := validateAMMMarket(params.Market)
	if err != nil {
		return ammAllowanceRequest{}, err
	}
	if params.Side != AMMAllowanceSideBuy && params.Side != AMMAllowanceSideSell {
		return ammAllowanceRequest{}, fmt.Errorf("Side must be BUY or SELL")
	}
	if err := validateAMMOnBehalfOf(params.OnBehalfOf); err != nil {
		return ammAllowanceRequest{}, err
	}
	return ammAllowanceRequest{
		Market:     market,
		Side:       params.Side,
		OnBehalfOf: params.OnBehalfOf,
	}, nil
}

func buildAMMBuyRequest(params AMMBuyParams) (ammBuyRequest, error) {
	market, err := validateAMMTradeParams(
		params.Market,
		params.OutcomeIndex,
		params.CollateralAmount,
		"CollateralAmount",
		params.SlippageBps,
		params.IdempotencyKey,
		params.OnBehalfOf,
	)
	if err != nil {
		return ammBuyRequest{}, err
	}
	return ammBuyRequest{
		Market:           market,
		OutcomeIndex:     params.OutcomeIndex,
		CollateralAmount: params.CollateralAmount,
		SlippageBps:      copyOptionalInt(params.SlippageBps),
		IdempotencyKey:   params.IdempotencyKey,
		OnBehalfOf:       params.OnBehalfOf,
	}, nil
}

func buildAMMSellRequest(params AMMSellParams) (ammSellRequest, error) {
	market, err := validateAMMTradeParams(
		params.Market,
		params.OutcomeIndex,
		params.CollateralReturnAmount,
		"CollateralReturnAmount",
		params.SlippageBps,
		params.IdempotencyKey,
		params.OnBehalfOf,
	)
	if err != nil {
		return ammSellRequest{}, err
	}
	return ammSellRequest{
		Market:                 market,
		OutcomeIndex:           params.OutcomeIndex,
		CollateralReturnAmount: params.CollateralReturnAmount,
		SlippageBps:            copyOptionalInt(params.SlippageBps),
		IdempotencyKey:         params.IdempotencyKey,
		OnBehalfOf:             params.OnBehalfOf,
	}, nil
}

func validateAMMTradeParams(
	market string,
	outcomeIndex AMMOutcomeIndex,
	amount string,
	amountField string,
	slippageBps *int,
	idempotencyKey string,
	onBehalfOf int,
) (string, error) {
	market, err := validateAMMMarket(market)
	if err != nil {
		return "", err
	}
	if outcomeIndex != AMMOutcomeYes && outcomeIndex != AMMOutcomeNo {
		return "", fmt.Errorf("OutcomeIndex must be 0 (YES) or 1 (NO)")
	}
	if err := validateAMMPositiveInteger(amount, amountField); err != nil {
		return "", err
	}
	if slippageBps != nil && (*slippageBps < 0 || *slippageBps > 1000) {
		return "", fmt.Errorf("SlippageBps must be between 0 and 1000")
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return "", fmt.Errorf("IdempotencyKey is required")
	}
	if utf8.RuneCountInString(idempotencyKey) > ammIdempotencyKeyMaxLength {
		return "", fmt.Errorf("IdempotencyKey must be at most %d characters", ammIdempotencyKeyMaxLength)
	}
	if err := validateAMMOnBehalfOf(onBehalfOf); err != nil {
		return "", err
	}
	return market, nil
}

func validateAMMMarket(market string) (string, error) {
	market = strings.TrimSpace(market)
	if market == "" {
		return "", fmt.Errorf("Market is required")
	}
	if utf8.RuneCountInString(market) > ammMarketMaxLength {
		return "", fmt.Errorf("Market must be at most %d characters", ammMarketMaxLength)
	}
	return market, nil
}

func validateAMMPositiveInteger(value string, field string) error {
	if len(value) > ammAmountMaxLength || !ammPositiveIntegerRegex.MatchString(value) {
		return fmt.Errorf("%s must be a positive integer string in the collateral token base unit", field)
	}
	parsed, ok := new(big.Int).SetString(value, 10)
	if !ok || parsed.Sign() <= 0 || parsed.Cmp(ammMaxUint256) > 0 {
		return fmt.Errorf("%s must be a positive integer string in the collateral token base unit", field)
	}
	return nil
}

func validateAMMOnBehalfOf(onBehalfOf int) error {
	if onBehalfOf < 0 || onBehalfOf > ammMaxOnBehalfOf {
		return fmt.Errorf("OnBehalfOf must be zero or a positive 32-bit integer")
	}
	return nil
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
