package limitless

import (
	"context"
	"fmt"
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

// getProfile fetches a user profile by wallet address and decodes into result.
// This is unexported because it's used internally by OrderClient.
func (pf *PortfolioFetcher) getProfile(ctx context.Context, address string, result any) error {
	pf.logger.Debug("Fetching user profile", map[string]any{"address": address})

	if err := pf.client.Get(ctx, "/profiles/"+address, result); err != nil {
		pf.logger.Error("Failed to fetch user profile", fmt.Errorf("address: %s: %w", address, err))
		return err
	}

	pf.logger.Info("User profile fetched successfully", map[string]any{"address": address})
	return nil
}

// GetProfile fetches a user profile by wallet address.
func (pf *PortfolioFetcher) GetProfile(ctx context.Context, address string) (*UserProfile, error) {
	var profile UserProfile
	if err := pf.getProfile(ctx, address, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetPositions fetches the raw portfolio positions response.
func (pf *PortfolioFetcher) GetPositions(ctx context.Context) (*PortfolioPositionsResponse, error) {
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

// GetUserHistory fetches paginated user history.
// Defaults to page=1, limit=10 when zero values are passed.
func (pf *PortfolioFetcher) GetUserHistory(ctx context.Context, page, limit int) (*HistoryResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 10
	}

	pf.logger.Debug("Fetching user history", map[string]any{"page": page, "limit": limit})

	path := fmt.Sprintf("/portfolio/history?page=%d&limit=%d", page, limit)

	var resp HistoryResponse
	if err := pf.client.Get(ctx, path, &resp); err != nil {
		pf.logger.Error("Failed to fetch user history", err, map[string]any{"page": page, "limit": limit})
		return nil, err
	}

	pf.logger.Info("User history fetched successfully")
	return &resp, nil
}
