package limitless

import (
	"context"
	"fmt"
	"net/url"
)

// PortfolioFetcher retrieves user positions and portfolio information.
type PortfolioFetcher struct {
	client *HttpClient
	logger Logger
}

// PortfolioOption configures a PortfolioFetcher.
type PortfolioOption func(*PortfolioFetcher)

// WithPortfolioLogger sets the logger for the PortfolioFetcher.
func WithPortfolioLogger(l Logger) PortfolioOption {
	return func(pf *PortfolioFetcher) {
		pf.logger = l
	}
}

// NewPortfolioFetcher creates a new portfolio fetcher.
func NewPortfolioFetcher(client *HttpClient, opts ...PortfolioOption) *PortfolioFetcher {
	pf := &PortfolioFetcher{
		client: client,
		logger: NewNoOpLogger(),
	}
	for _, opt := range opts {
		opt(pf)
	}
	return pf
}

func (pf *PortfolioFetcher) getProfileAtPath(ctx context.Context, operation, path string, fields map[string]any, result any) error {
	if err := pf.client.requireAuth("GetProfile"); err != nil {
		return err
	}

	pf.logger.Debug(operation, fields)

	if err := pf.client.Get(ctx, path, result); err != nil {
		pf.logger.Error("Failed to fetch user profile", err, fields)
		return err
	}

	pf.logger.Info("User profile fetched successfully", fields)
	return nil
}

func (pf *PortfolioFetcher) getProfileRawAtPath(ctx context.Context, operation, path string, fields map[string]any) (*RawResponse, error) {
	if err := pf.client.requireAuth("GetProfile"); err != nil {
		return nil, err
	}

	pf.logger.Debug(operation, fields)

	raw, err := pf.client.GetRaw(ctx, path)
	if err != nil {
		pf.logger.Error("Failed to fetch user profile", err, fields)
		return nil, err
	}

	pf.logger.Info("User profile fetched successfully", fields)
	return raw, nil
}

// getProfile fetches a user profile by wallet address and decodes into result.
// This is unexported because it's used internally by OrderClient.
func (pf *PortfolioFetcher) getProfile(ctx context.Context, address string, result any) error {
	return pf.getProfileAtPath(
		ctx,
		"Fetching user profile",
		"/profiles/"+url.PathEscape(address),
		map[string]any{"address": address},
		result,
	)
}

// GetProfile fetches a user profile by wallet address.
func (pf *PortfolioFetcher) GetProfile(ctx context.Context, address string) (*UserProfile, error) {
	result, err := pf.GetProfileWithRawResponse(ctx, address)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetProfileWithRawResponse is the raw-response variant of GetProfile.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (pf *PortfolioFetcher) GetProfileWithRawResponse(ctx context.Context, address string) (*RawResult[UserProfile], error) {
	raw, err := pf.getProfileRawAtPath(
		ctx,
		"Fetching user profile",
		"/profiles/"+url.PathEscape(address),
		map[string]any{"address": address},
	)
	return decodeRawResult[UserProfile](raw, err)
}

// GetCurrentProfile fetches the authenticated caller's private profile.
func (pf *PortfolioFetcher) GetCurrentProfile(ctx context.Context) (*UserProfile, error) {
	result, err := pf.GetCurrentProfileWithRawResponse(ctx)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetCurrentProfileWithRawResponse is the raw-response variant of GetCurrentProfile.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (pf *PortfolioFetcher) GetCurrentProfileWithRawResponse(ctx context.Context) (*RawResult[UserProfile], error) {
	raw, err := pf.getProfileRawAtPath(
		ctx,
		"Fetching current user profile",
		"/profiles/me",
		map[string]any{},
	)
	return decodeRawResult[UserProfile](raw, err)
}

// GetPositions fetches the raw portfolio positions response.
func (pf *PortfolioFetcher) GetPositions(ctx context.Context) (*PortfolioPositionsResponse, error) {
	result, err := pf.GetPositionsWithRawResponse(ctx)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetPositionsWithRawResponse is the raw-response variant of GetPositions.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (pf *PortfolioFetcher) GetPositionsWithRawResponse(ctx context.Context) (*RawResult[PortfolioPositionsResponse], error) {
	if err := pf.client.requireAuth("GetPositions"); err != nil {
		return nil, err
	}

	pf.logger.Debug("Fetching user positions")

	raw, err := pf.client.GetRaw(ctx, "/portfolio/positions")
	result, err := decodeRawResult[PortfolioPositionsResponse](raw, err)
	if err != nil {
		pf.logger.Error("Failed to fetch positions", err)
		return nil, err
	}

	pf.logger.Info("Positions fetched successfully", map[string]any{
		"clobCount": len(result.Data.CLOB),
		"ammCount":  len(result.Data.AMM),
	})

	return result, nil
}

// GetCLOBPositions fetches only CLOB positions.
func (pf *PortfolioFetcher) GetCLOBPositions(ctx context.Context) ([]CLOBPosition, error) {
	resp, err := pf.GetPositions(ctx)
	if err != nil {
		return nil, err
	}
	if resp.CLOB == nil {
		return []CLOBPosition{}, nil
	}
	return resp.CLOB, nil
}

// GetAMMPositions fetches only AMM positions.
func (pf *PortfolioFetcher) GetAMMPositions(ctx context.Context) ([]AMMPosition, error) {
	resp, err := pf.GetPositions(ctx)
	if err != nil {
		return nil, err
	}
	if resp.AMM == nil {
		return []AMMPosition{}, nil
	}
	return resp.AMM, nil
}

// GetUserHistory fetches user history with cursor-based pagination.
// Pass an empty cursor for the first page; use the returned NextCursor for subsequent pages.
// Defaults to limit=20 when zero value is passed.
func (pf *PortfolioFetcher) GetUserHistory(ctx context.Context, cursor string, limit int) (*HistoryResponse, error) {
	result, err := pf.GetUserHistoryWithRawResponse(ctx, cursor, limit)
	if err != nil {
		return nil, err
	}
	return &result.Data, nil
}

// GetUserHistoryWithRawResponse is the raw-response variant of GetUserHistory.
// It returns the decoded value alongside the full HTTP response (status, headers, body).
func (pf *PortfolioFetcher) GetUserHistoryWithRawResponse(ctx context.Context, cursor string, limit int) (*RawResult[HistoryResponse], error) {
	if err := pf.client.requireAuth("GetUserHistory"); err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 20
	}

	pf.logger.Debug("Fetching user history", map[string]any{"cursor": cursor, "limit": limit})

	// Always send cursor=, using an empty value on the first page.
	query := url.Values{}
	query.Set("cursor", cursor)
	query.Set("limit", fmt.Sprintf("%d", limit))
	path := "/portfolio/history?" + query.Encode()

	raw, err := pf.client.GetRaw(ctx, path)
	result, err := decodeRawResult[HistoryResponse](raw, err)
	if err != nil {
		pf.logger.Error("Failed to fetch user history", err, map[string]any{"cursor": cursor, "limit": limit})
		return nil, err
	}

	pf.logger.Info("User history fetched successfully")
	return result, nil
}
