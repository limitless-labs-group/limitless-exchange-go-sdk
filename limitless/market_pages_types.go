package limitless

import "encoding/json"

// NavigationNode represents a node in the navigation tree.
type NavigationNode struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Slug     string           `json:"slug"`
	Path     string           `json:"path"`
	Icon     *string          `json:"icon,omitempty"`
	Children []NavigationNode `json:"children"`
}

// FilterGroupOption represents an option within a filter group.
type FilterGroupOption struct {
	Label    string                 `json:"label"`
	Value    string                 `json:"value"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// FilterGroup represents a filter group for market pages.
type FilterGroup struct {
	Name          *string                `json:"name,omitempty"`
	Slug          *string                `json:"slug,omitempty"`
	AllowMultiple *bool                  `json:"allowMultiple,omitempty"`
	Presentation  *string                `json:"presentation,omitempty"`
	Options       []FilterGroupOption    `json:"options,omitempty"`
	Source        map[string]interface{} `json:"source,omitempty"`
}

// BreadcrumbItem represents a breadcrumb in the market page path.
type BreadcrumbItem struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
	Path string `json:"path"`
}

// MarketPage represents a market page resolved by path.
type MarketPage struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Slug         string                 `json:"slug"`
	FullPath     string                 `json:"fullPath"`
	Description  *string                `json:"description"`
	BaseFilter   map[string]interface{} `json:"baseFilter"`
	FilterGroups []FilterGroup          `json:"filterGroups"`
	Metadata     map[string]interface{} `json:"metadata"`
	Breadcrumb   []BreadcrumbItem       `json:"breadcrumb"`
}

// PropertyOption represents an option for a property key.
type PropertyOption struct {
	ID              string                 `json:"id"`
	PropertyKeyID   string                 `json:"propertyKeyId"`
	Value           string                 `json:"value"`
	Label           string                 `json:"label"`
	SortOrder       int                    `json:"sortOrder"`
	ParentOptionID  *string                `json:"parentOptionId"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
}

// PropertyKey represents a property key for market filtering.
type PropertyKey struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	Type      string                 `json:"type"` // "select" or "multi-select"
	Metadata  map[string]interface{} `json:"metadata"`
	IsSystem  bool                   `json:"isSystem"`
	Options   []PropertyOption       `json:"options,omitempty"`
	CreatedAt string                 `json:"createdAt"`
	UpdatedAt string                 `json:"updatedAt"`
}

// OffsetPagination contains offset-based pagination metadata.
type OffsetPagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// CursorPagination contains cursor-based pagination metadata.
type CursorPagination struct {
	NextCursor *string `json:"nextCursor"`
}

// MarketPageSortField constrains sort fields for market page listings.
type MarketPageSortField string

const (
	SortFieldCreatedAt MarketPageSortField = "createdAt"
	SortFieldUpdatedAt MarketPageSortField = "updatedAt"
	SortFieldDeadline  MarketPageSortField = "deadline"
	SortFieldID        MarketPageSortField = "id"
)

// MarketPageSort is a sort value (field or "-field" for descending).
type MarketPageSort string

// MarketPageMarketsParams contains query parameters for fetching markets from a market page.
type MarketPageMarketsParams struct {
	Page    *int
	Limit   *int
	Sort    MarketPageSort
	Cursor  string
	Filters map[string]any // key → value (string, bool, number, or []any for multi-value)
}

// MarketPageMarketsOffsetResponse is the offset-paginated response for market page markets.
type MarketPageMarketsOffsetResponse struct {
	Data       []Market         `json:"data"`
	Pagination OffsetPagination `json:"pagination"`
}

// MarketPageMarketsCursorResponse is the cursor-paginated response for market page markets.
type MarketPageMarketsCursorResponse struct {
	Data   []Market         `json:"data"`
	Cursor CursorPagination `json:"cursor"`
}

// MarketPageMarketsResponse is the raw response from the API.
// It may contain either pagination (offset) or cursor metadata.
type MarketPageMarketsResponse struct {
	Data       []Market          `json:"data"`
	Pagination *OffsetPagination `json:"pagination,omitempty"`
	Cursor     *CursorPagination `json:"cursor,omitempty"`

	// RawData holds the raw market JSON entries for custom parsing if needed.
	RawData []json.RawMessage `json:"-"`
}
