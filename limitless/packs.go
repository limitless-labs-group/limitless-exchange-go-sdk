package limitless

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var packAmountPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var packIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// PacksService handles curated partner Packs. Reuse one service per partner;
// requests are serialized and spaced to stay under 10 requests per 10 seconds.
// No method automatically retries a write or changes an idempotency key.
// Persist PackBuyParams in application storage before calling Buy.
type PacksService struct {
	client  *HttpClient
	gate    chan struct{}
	next    time.Time
	spacing time.Duration
}

func NewPacksService(client *HttpClient) *PacksService {
	return &PacksService{client: client, gate: make(chan struct{}, 1), spacing: 1100 * time.Millisecond}
}

func (s *PacksService) call(ctx context.Context, method, path string, body any, profile int, result any) error {
	if s.client == nil {
		return fmt.Errorf("Packs HTTP client is not configured")
	}
	if strings.HasPrefix(path, "/parlay/partner/") && s.client.HMACCredentials() == nil {
		return fmt.Errorf("partner Packs require HMAC credentials with trading and delegated_signing scopes")
	}
	// Reject a global delegation header: it would leak into quote/buy/claim writes.
	for k := range s.client.headers {
		if strings.EqualFold(k, "x-on-behalf-of") {
			return fmt.Errorf("configure onBehalfOf on Packs methods, not global headers")
		}
	}
	select {
	case s.gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-s.gate }()
	if err := packDelay(ctx, time.Until(s.next)); err != nil {
		return err
	}
	s.next = time.Now().Add(s.spacing)
	headers := map[string]string{}
	if profile > 0 {
		headers["x-on-behalf-of"] = strconv.Itoa(profile)
	}
	raw, err := s.client.doRequestRawWithConfig(ctx, method, path, body, requestExecutionConfig{extraHeaders: headers})
	if err != nil {
		if _, ok := err.(*RateLimitError); ok {
			s.next = time.Now().Add(10 * time.Second)
		}
		return err
	}
	return json.Unmarshal(raw.Body, result)
}

func (s *PacksService) List(ctx context.Context) ([]Pack, error) {
	var out []Pack
	err := s.call(ctx, http.MethodGet, "/parlay/packs", nil, 0, &out)
	return out, err
}
func (s *PacksService) Search(ctx context.Context, query string) ([]Pack, error) {
	var out []Pack
	err := s.call(ctx, http.MethodGet, "/parlay/packs/search?query="+url.QueryEscape(query), nil, 0, &out)
	return out, err
}
func (s *PacksService) Get(ctx context.Context, id string) (*Pack, error) {
	if err := validatePackID(id); err != nil {
		return nil, err
	}
	var out Pack
	err := s.call(ctx, http.MethodGet, "/parlay/packs/"+id, nil, 0, &out)
	return &out, err
}
func (s *PacksService) Quote(ctx context.Context, params PackQuoteParams) (*PackQuote, error) {
	if err := validatePackID(params.PackID); err != nil {
		return nil, err
	}
	if err := validatePackAmount(params.StakeAmount); err != nil {
		return nil, err
	}
	var out PackQuote
	err := s.call(ctx, http.MethodPost, "/parlay/partner/quote", params, 0, &out)
	return &out, err
}
func (s *PacksService) Buy(ctx context.Context, params PackBuyParams) (*PackBuyResponse, error) {
	if err := validatePackBuy(params); err != nil {
		return nil, err
	}
	var out PackBuyResponse
	err := s.call(ctx, http.MethodPost, "/parlay/partner/buy", params, 0, &out)
	return &out, err
}
func (s *PacksService) ListPositions(ctx context.Context, onBehalfOf, limit, offset int) (*PackPositions, error) {
	if err := validatePackProfile(onBehalfOf); err != nil {
		return nil, err
	}
	limit = max(1, min(50, limit))
	offset = max(0, min(5000, offset))
	var out PackPositions
	err := s.call(ctx, http.MethodGet, fmt.Sprintf("/parlay/partner/positions?limit=%d&offset=%d", limit, offset), nil, onBehalfOf, &out)
	return &out, err
}
func (s *PacksService) GetPosition(ctx context.Context, onBehalfOf int, id string) (*PackPosition, error) {
	if err := validatePackProfile(onBehalfOf); err != nil {
		return nil, err
	}
	if err := validatePackID(id); err != nil {
		return nil, err
	}
	var out PackPosition
	err := s.call(ctx, http.MethodGet, "/parlay/partner/positions/"+id, nil, onBehalfOf, &out)
	return &out, err
}

// WaitForPosition waits for minting, not settlement, for at most 60 checks.
// Context cancellation can stop polling sooner.
func (s *PacksService) WaitForPosition(ctx context.Context, onBehalfOf int, id string) (*PackPosition, error) {
	for attempt := 0; attempt < 60; attempt++ {
		out, err := s.GetPosition(ctx, onBehalfOf, id)
		if err != nil {
			return nil, err
		}
		switch out.Status {
		case "ACTIVE", "WON", "LOST":
			return out, nil
		case "VAULT_REVERTED":
			return out, fmt.Errorf("Pack position %s is %s", id, out.Status)
		case "PENDING", "FAILED":
		default:
			return out, fmt.Errorf("unknown Pack position status %q", out.Status)
		}
		if err := packDelay(ctx, 5*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("Pack mint polling exhausted; replay the identical saved buy to distinguish NOT_SUBMITTED from recovery; do not rotate its key")
}

// ClaimOnce advances at most one sponsored transaction step.
func (s *PacksService) ClaimOnce(ctx context.Context, onBehalfOf int, id string) (*PackClaimResponse, error) {
	return s.claimCall(ctx, onBehalfOf, id, "", "")
}

// Claim converges to CLAIMED, stopping on any error (including PACK_SUBMISSION_UNKNOWN).
// Stops after 60 calls; use a context deadline to stop sooner. Recover unknown submissions with ConfirmClaim and a mined tx hash.
func (s *PacksService) Claim(ctx context.Context, onBehalfOf int, id string) (*PackClaimResponse, error) {
	for attempt := 0; attempt < 60; attempt++ {
		out, err := s.ClaimOnce(ctx, onBehalfOf, id)
		if err != nil {
			return nil, err
		}
		switch out.Status {
		case "CLAIMED":
			return out, nil
		case "APPROVAL_PENDING", "CLAIM_PENDING":
		default:
			return out, fmt.Errorf("unknown Pack claim status %q", out.Status)
		}
		if err := packDelay(ctx, 5*time.Second); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("Pack polling exhausted; resume with the same position/request")
}
func (s *PacksService) ConfirmClaim(ctx context.Context, onBehalfOf int, id, txHash string) (*PackClaimResponse, error) {
	if !regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`).MatchString(txHash) {
		return nil, fmt.Errorf("txHash must be a mined transaction hash")
	}
	return s.claimCall(ctx, onBehalfOf, id, "/confirm", txHash)
}
func (s *PacksService) claimCall(ctx context.Context, profile int, id, suffix, hash string) (*PackClaimResponse, error) {
	if err := validatePackProfile(profile); err != nil {
		return nil, err
	}
	if err := validatePackID(id); err != nil {
		return nil, err
	}
	body := struct {
		OnBehalfOf int    `json:"onBehalfOf"`
		TxHash     string `json:"txHash,omitempty"`
	}{profile, hash}
	var out PackClaimResponse
	err := s.call(ctx, http.MethodPost, "/parlay/partner/positions/"+id+"/claim"+suffix, body, 0, &out)
	return &out, err
}

// PackMinPayout floors the slippage-adjusted payout using integer arithmetic only.
func PackMinPayout(payout string, slippageBps int) (string, error) {
	if err := validatePackAmount(payout); err != nil {
		return "", err
	}
	if slippageBps < 0 || slippageBps >= 10000 {
		return "", fmt.Errorf("slippageBps must be between 0 and 9999")
	}
	n, _ := new(big.Int).SetString(payout, 10)
	n.Mul(n, big.NewInt(int64(10000-slippageBps))).Div(n, big.NewInt(10000))
	if n.Sign() == 0 {
		return "", fmt.Errorf("minimum payout rounds to zero")
	}
	return n.String(), nil
}
func validatePackProfile(id int) error {
	if id < 1 || id > 2147483647 {
		return fmt.Errorf("onBehalfOf must be a positive 32-bit integer")
	}
	return nil
}
func validatePackID(id string) error {
	if !packIDPattern.MatchString(id) {
		return fmt.Errorf("Pack/position ID must be a UUID")
	}
	return nil
}
func validatePackAmount(value string) error {
	if !packAmountPattern.MatchString(value) {
		return fmt.Errorf("Pack amounts and deadlines must be positive integer strings of at most 20 digits")
	}
	return nil
}
func validatePackBuy(p PackBuyParams) error {
	if err := validatePackProfile(p.OnBehalfOf); err != nil {
		return err
	}
	if err := validatePackID(p.PackID); err != nil {
		return err
	}
	if len(p.Deadline) > 12 {
		return fmt.Errorf("deadline must contain at most 12 digits")
	}
	for _, value := range []string{p.StakeAmount, p.MinPayoutAmount, p.Deadline} {
		if err := validatePackAmount(value); err != nil {
			return err
		}
	}
	// Do not check deadline against the clock: expired requests must remain replayable.
	if len(p.IdempotencyKey) < 1 || len(p.IdempotencyKey) > 128 {
		return fmt.Errorf("idempotencyKey must contain 1 to 128 printable ASCII characters without spaces")
	}
	for _, c := range []byte(p.IdempotencyKey) {
		if c < 33 || c > 126 {
			return fmt.Errorf("idempotencyKey must contain printable ASCII without spaces")
		}
	}
	return nil
}
func packDelay(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
