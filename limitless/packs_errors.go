package limitless

import (
	"encoding/json"
	"errors"
)

type PackErrorKind string

const (
	PackErrorOther               PackErrorKind = "OTHER"
	PackErrorAllowanceRequired   PackErrorKind = "ALLOWANCE_REQUIRED"
	PackErrorNotSubmitted        PackErrorKind = "NOT_SUBMITTED"
	PackErrorPriceDrifted        PackErrorKind = "PRICE_DRIFTED"
	PackErrorIntentExpired       PackErrorKind = "INTENT_EXPIRED"
	PackErrorWalletBusy          PackErrorKind = "WALLET_BUSY"
	PackErrorRateLimited         PackErrorKind = "RATE_LIMITED"
	PackErrorSubmissionUnknown   PackErrorKind = "SUBMISSION_UNKNOWN"
	PackErrorTransactionReverted PackErrorKind = "TRANSACTION_REVERTED"
)

// PackErrorDetails exposes recovery metadata without retrying or changing a key.
// Optional fields are empty when absent or not strings in the API response.
type PackErrorDetails struct {
	Kind              PackErrorKind
	Code              string
	Message           string
	SubmissionState   string
	ParlayID          string
	Recovery          string
	TokenAddress      string
	Spender           string
	RequiredAllowance string
}

// ClassifyPackError accepts API errors and wrapped API errors. OTHER does not
// imply the purchase was unsubmitted; retain the saved request on unknown errors.
func ClassifyPackError(err error) PackErrorDetails {
	var api interface{ PackErrorDetails() PackErrorDetails }
	if errors.As(err, &api) {
		return api.PackErrorDetails()
	}
	return PackErrorDetails{Kind: PackErrorOther}
}

// PackErrorDetails is promoted to the SDK's typed API error variants.
func (e *APIError) PackErrorDetails() PackErrorDetails {
	var data map[string]any
	_ = json.Unmarshal(e.Data, &data)
	str := func(key string) string { value, _ := data[key].(string); return value }
	d := PackErrorDetails{
		Kind: PackErrorOther, Code: str("code"), Message: e.Message,
		SubmissionState: str("submissionState"), ParlayID: str("parlayId"),
		Recovery: str("recovery"), TokenAddress: str("tokenAddress"),
		Spender: str("spender"), RequiredAllowance: str("requiredAllowance"),
	}
	switch {
	case e.Status == 409 && d.Code == "PACK_SUBMISSION_UNKNOWN":
		d.Kind = PackErrorSubmissionUnknown
	case e.Status >= 400 && e.Status < 500 && e.Status != 429 && d.SubmissionState == "NOT_SUBMITTED":
		d.Kind = PackErrorNotSubmitted
	case e.Status == 409 && d.Code == "PACK_ALLOWANCE_REQUIRED":
		d.Kind = PackErrorAllowanceRequired
	case e.Status == 409 && d.Code == "PRICE_DRIFTED":
		d.Kind = PackErrorPriceDrifted
	case e.Status == 401 && e.Message == "Intent expired — please retry":
		d.Kind = PackErrorIntentExpired
	case e.Status == 409 && e.Message == "Another Pack operation is in progress for this wallet; retry the same request":
		d.Kind = PackErrorWalletBusy
	case e.Status == 429:
		d.Kind = PackErrorRateLimited
	case e.Status == 409 && d.Code == "PACK_TRANSACTION_REVERTED":
		d.Kind = PackErrorTransactionReverted
	}
	return d
}
