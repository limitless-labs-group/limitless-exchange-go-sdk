package limitless

import (
	"encoding/json"
	"testing"
)

func TestRealAPIJSON_ActiveMarkets_DeserializesWithFlexibleFields(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"data": [
			{
				"id": 101,
				"slug": "btc-above-100k",
				"title": "BTC above $100k?",
				"createdAt": "2026-03-17T00:00:00.000Z",
				"updatedAt": "2026-03-17T00:00:00.000Z",
				"collateralToken": {"address":"0x1","decimals":6,"symbol":"USDC"},
				"expirationDate": "2026-12-31T00:00:00.000Z",
				"expirationTimestamp": 1798675200000,
				"categories": ["crypto"],
				"status": "ACTIVE",
				"creator": {"name":"Limitless"},
				"tags": ["btc"],
				"tradeType": "clob",
				"marketType": "single",
				"priorityIndex": 1,
				"metadata": {"fee": true},
				"settings": {
					"minSize": "0.001",
					"maxSpread": 0.05,
					"dailyReward": "0",
					"rewardsEpoch": "epoch-12",
					"c": 0.5,
					"rebateRate": 0.1
				},
				"venue": {
					"exchange": "0xa4409D988CA2218d956BeEFD3874100F444f0DC3",
					"adapter": null
				},
				"tokens": {"yes":"11","no":"12"},
				"prices": [0.55, 0.45]
			}
		],
		"totalMarketsCount": 1
	}`)

	var resp ActiveMarketsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("failed to unmarshal active markets payload: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Fatalf("expected one market, got %d", len(resp.Data))
	}
	m := resp.Data[0]
	if m.Settings == nil {
		t.Fatal("expected market settings to be present")
	}
	if _, ok := m.Settings.RewardsEpoch.(string); !ok {
		t.Fatalf("expected rewardsEpoch to deserialize as string, got %T", m.Settings.RewardsEpoch)
	}
	if _, ok := m.Settings.C.(float64); !ok {
		t.Fatalf("expected c to deserialize as float64, got %T", m.Settings.C)
	}
	if m.Venue == nil || m.Venue.Exchange == "" {
		t.Fatalf("expected venue exchange, got %+v", m.Venue)
	}
}

func TestRealAPIJSON_PortfolioPositions_Deserializes(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"amm": [],
		"group": [],
		"clob": [
			{
				"market": {
					"id": "57493",
					"slug": "btc-above-100k",
					"title": "BTC above $100k?",
					"closed": false,
					"deadline": "2026-12-31T00:00:00.000Z"
				},
				"makerAddress": "0xa00BCB04073B243E8A55f3B5899AefF596bF17C6",
				"positions": {
					"yes": {"cost":"10","fillPrice":"0.5","marketValue":"11","realisedPnl":"0","unrealizedPnl":"1"},
					"no": {"cost":"0","fillPrice":"0","marketValue":"0","realisedPnl":"0","unrealizedPnl":"0"}
				},
				"tokensBalance": {"yes":"20","no":"0"},
				"latestTrade": {"latestYesPrice":0.55,"latestNoPrice":0.45,"outcomeTokenPrice":0.55}
			}
		]
	}`)

	var resp PortfolioPositionsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("failed to unmarshal portfolio payload: %v", err)
	}

	if len(resp.CLOB) != 1 {
		t.Fatalf("expected one CLOB position, got %d", len(resp.CLOB))
	}
	if string(resp.CLOB[0].Market.ID) != `"57493"` {
		t.Fatalf("expected raw market id \"57493\", got %s", string(resp.CLOB[0].Market.ID))
	}
	if resp.CLOB[0].Market.Slug != "btc-above-100k" {
		t.Fatalf("unexpected market slug: %s", resp.CLOB[0].Market.Slug)
	}
}
