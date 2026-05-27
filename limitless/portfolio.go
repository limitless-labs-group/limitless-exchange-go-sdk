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
	var profile UserProfile
	if err := pf.getProfile(ctx, address, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetCurrentProfile fetches the authenticated caller's private profile.
func (pf *PortfolioFetcher) GetCurrentProfile(ctx context.Context) (*UserProfile, error) {
	var profile UserProfile
	if err := pf.getProfileAtPath(
		ctx,
		"Fetching current user profile",
		"/profiles/me",
		map[string]any{},
		&profile,
	); err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetPositions fetches the raw portfolio positions response.
func (pf *PortfolioFetcher) GetPositions(ctx context.Context) (*PortfolioPositionsResponse, error) {
	if err := pf.client.requireAuth("GetPositions"); err != nil {
		return nil, err
	}

	pf.logger.Debug("Fetching user positions")

	var resp PortfolioPositionsResponse
	if err := pf.client.Get(ctx, "/portfolio/positions", &resp); err != nil {
		pf.logger.Error("Failed to fetch positions", err)
		return nil, err
	}

	pf.logger.Info("Positions fetched successfully", map[string]any{
		"clobCount": len(resp.CLOB),
		"ammCount":  len(resp.AMM),
	})

	return &resp, nil
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

	var resp HistoryResponse
	if err := pf.client.Get(ctx, path, &resp); err != nil {
		pf.logger.Error("Failed to fetch user history", err, map[string]any{"cursor": cursor, "limit": limit})
		return nil, err
	}

	pf.logger.Info("User history fetched successfully")
	return &resp, nil
}
