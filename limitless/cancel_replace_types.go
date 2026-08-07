package limitless

import (
	"encoding/json"
	"fmt"
)

type CancelReplaceMode string

const (
	CancelReplaceAllowFailure  CancelReplaceMode = "ALLOW_FAILURE"
	CancelReplaceStopOnFailure CancelReplaceMode = "STOP_ON_FAILURE"
)

type STPPolicy string

const (
	STPCancelBoth  STPPolicy = "cancel_both"
	STPCancelMaker STPPolicy = "cancel_maker"
	STPCancelTaker STPPolicy = "cancel_taker"
)

type CancelTarget struct {
	orderID       string
	clientOrderID string
}

func CancelByOrderID(orderID string) CancelTarget {
	return CancelTarget{orderID: orderID}
}

func CancelByClientOrderID(clientOrderID string) CancelTarget {
	return CancelTarget{clientOrderID: clientOrderID}
}

func (t CancelTarget) MarshalJSON() ([]byte, error) {
	if (t.orderID == "") == (t.clientOrderID == "") {
		return nil, fmt.Errorf("cancel target requires exactly one of order ID or client order ID")
	}
	if t.orderID != "" {
		return json.Marshal(struct {
			OrderID string `json:"orderId"`
		}{t.orderID})
	}
	return json.Marshal(struct {
		ClientOrderID string `json:"clientOrderId"`
	}{t.clientOrderID})
}

type CancelReplaceOrder struct {
	Order         OrderSubmission `json:"order"`
	OwnerID       int             `json:"ownerId"`
	OrderType     OrderType       `json:"orderType"`
	MarketSlug    string          `json:"marketSlug"`
	PostOnly      *bool           `json:"postOnly,omitempty"`
	ClientOrderID string          `json:"clientOrderId,omitempty"`
	Timestamp     *int64          `json:"timestamp,omitempty"`
	RecvWindow    *int64          `json:"recvWindow,omitempty"`
	STPPolicy     STPPolicy       `json:"stpPolicy,omitempty"`
}

type CancelReplaceRequest struct {
	Cancel      CancelTarget       `json:"cancel"`
	Replacement CancelReplaceOrder `json:"replacement"`
	Mode        CancelReplaceMode  `json:"mode"`
	OnBehalfOf  *int               `json:"onBehalfOf,omitempty"`
}

type CancelReplaceBatchRequest struct {
	Operations []CancelReplaceRequest `json:"operations"`
}

type CancelReplaceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CancelReplaceCancelStatus string

const (
	CancelReplaceCancelSuccess CancelReplaceCancelStatus = "SUCCESS"
	CancelReplaceCancelFailure CancelReplaceCancelStatus = "FAILURE"
	CancelReplaceCancelUnknown CancelReplaceCancelStatus = "UNKNOWN"
)

type CancelReplaceCancelSuccessData struct {
	OrderID       string  `json:"orderId"`
	ClientOrderID *string `json:"clientOrderId,omitempty"`
}

type CancelReplaceCancelResult struct {
	status  CancelReplaceCancelStatus
	success *CancelReplaceCancelSuccessData
	failure *CancelReplaceError
}

func (r CancelReplaceCancelResult) Status() CancelReplaceCancelStatus { return r.status }
func (r CancelReplaceCancelResult) Success() (*CancelReplaceCancelSuccessData, bool) {
	return r.success, r.success != nil
}
func (r CancelReplaceCancelResult) Error() (*CancelReplaceError, bool) {
	return r.failure, r.failure != nil
}

func (r *CancelReplaceCancelResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Status        CancelReplaceCancelStatus `json:"status"`
		OrderID       *string                   `json:"orderId"`
		ClientOrderID *string                   `json:"clientOrderId"`
		Error         *CancelReplaceError       `json:"error"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch wire.Status {
	case CancelReplaceCancelSuccess:
		if wire.OrderID == nil || *wire.OrderID == "" || wire.Error != nil {
			return fmt.Errorf("cancel SUCCESS requires orderId and forbids error")
		}
		*r = CancelReplaceCancelResult{status: wire.Status, success: &CancelReplaceCancelSuccessData{OrderID: *wire.OrderID, ClientOrderID: wire.ClientOrderID}}
	case CancelReplaceCancelFailure, CancelReplaceCancelUnknown:
		if wire.Error == nil || wire.OrderID != nil || wire.ClientOrderID != nil {
			return fmt.Errorf("cancel %s requires error and forbids success fields", wire.Status)
		}
		*r = CancelReplaceCancelResult{status: wire.Status, failure: wire.Error}
	default:
		return fmt.Errorf("unknown cancel status %q", wire.Status)
	}
	return nil
}

type CancelReplaceExecutionData struct {
	Order        CreatedOrder   `json:"order"`
	MakerMatches []OrderMatch   `json:"makerMatches,omitempty"`
	Execution    OrderExecution `json:"execution"`
}

func (d *CancelReplaceExecutionData) UnmarshalJSON(data []byte) error {
	type wireData struct {
		Order        json.RawMessage `json:"order"`
		MakerMatches []OrderMatch    `json:"makerMatches,omitempty"`
		Execution    json.RawMessage `json:"execution"`
	}
	var wire wireData
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.Order) == 0 || string(wire.Order) == "null" {
		return fmt.Errorf("replacement success data requires order")
	}
	if len(wire.Execution) == 0 || string(wire.Execution) == "null" {
		return fmt.Errorf("replacement success data requires execution")
	}
	if err := json.Unmarshal(wire.Order, &d.Order); err != nil {
		return fmt.Errorf("order: %w", err)
	}
	if err := json.Unmarshal(wire.Execution, &d.Execution); err != nil {
		return fmt.Errorf("execution: %w", err)
	}
	d.MakerMatches = wire.MakerMatches
	return nil
}

type CancelReplaceReplacementStatus string

const (
	CancelReplaceReplacementSuccess      CancelReplaceReplacementStatus = "SUCCESS"
	CancelReplaceReplacementFailure      CancelReplaceReplacementStatus = "FAILURE"
	CancelReplaceReplacementUnknown      CancelReplaceReplacementStatus = "UNKNOWN"
	CancelReplaceReplacementNotAttempted CancelReplaceReplacementStatus = "NOT_ATTEMPTED"
)

type CancelReplaceReplacementResult struct {
	status  CancelReplaceReplacementStatus
	success *CancelReplaceExecutionData
	failure *CancelReplaceError
}

func (r CancelReplaceReplacementResult) Status() CancelReplaceReplacementStatus { return r.status }
func (r CancelReplaceReplacementResult) Success() (*CancelReplaceExecutionData, bool) {
	return r.success, r.success != nil
}
func (r CancelReplaceReplacementResult) Error() (*CancelReplaceError, bool) {
	return r.failure, r.failure != nil
}

func (r *CancelReplaceReplacementResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Status CancelReplaceReplacementStatus `json:"status"`
		Data   *CancelReplaceExecutionData    `json:"data"`
		Error  *CancelReplaceError            `json:"error"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	switch wire.Status {
	case CancelReplaceReplacementSuccess:
		if wire.Data == nil || wire.Error != nil {
			return fmt.Errorf("replacement SUCCESS requires data and forbids error")
		}
		*r = CancelReplaceReplacementResult{status: wire.Status, success: wire.Data}
	case CancelReplaceReplacementFailure, CancelReplaceReplacementUnknown:
		if wire.Error == nil || wire.Data != nil {
			return fmt.Errorf("replacement %s requires error and forbids data", wire.Status)
		}
		*r = CancelReplaceReplacementResult{status: wire.Status, failure: wire.Error}
	case CancelReplaceReplacementNotAttempted:
		if wire.Data != nil || wire.Error != nil {
			return fmt.Errorf("replacement NOT_ATTEMPTED forbids data and error")
		}
		*r = CancelReplaceReplacementResult{status: wire.Status}
	default:
		return fmt.Errorf("unknown replacement status %q", wire.Status)
	}
	return nil
}

type CancelReplaceResult struct {
	Cancel      CancelReplaceCancelResult      `json:"cancel"`
	Replacement CancelReplaceReplacementResult `json:"replacement"`
}

func (r *CancelReplaceResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Cancel      json.RawMessage `json:"cancel"`
		Replacement json.RawMessage `json:"replacement"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.Cancel) == 0 || len(wire.Replacement) == 0 {
		return fmt.Errorf("cancel-replace response requires cancel and replacement")
	}
	if err := json.Unmarshal(wire.Cancel, &r.Cancel); err != nil {
		return err
	}
	if err := json.Unmarshal(wire.Replacement, &r.Replacement); err != nil {
		return err
	}
	return nil
}

type CancelReplaceBatchResult struct {
	Index       int                            `json:"index"`
	Cancel      CancelReplaceCancelResult      `json:"cancel"`
	Replacement CancelReplaceReplacementResult `json:"replacement"`
}

type CancelReplaceBatchResponse struct {
	Results []CancelReplaceBatchResult `json:"results"`
}

func (r *CancelReplaceBatchResult) UnmarshalJSON(data []byte) error {
	var wire struct {
		Index       *int            `json:"index"`
		Cancel      json.RawMessage `json:"cancel"`
		Replacement json.RawMessage `json:"replacement"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if wire.Index == nil || *wire.Index < 0 {
		return fmt.Errorf("batch result requires a non-negative index")
	}
	if len(wire.Cancel) == 0 || len(wire.Replacement) == 0 {
		return fmt.Errorf("batch result requires cancel and replacement")
	}
	r.Index = *wire.Index
	if err := json.Unmarshal(wire.Cancel, &r.Cancel); err != nil {
		return err
	}
	if err := json.Unmarshal(wire.Replacement, &r.Replacement); err != nil {
		return err
	}
	return nil
}

type CancelReplaceOrderParams struct {
	MarketSlug    string
	OrderType     OrderType
	Args          OrderArgs
	ClientOrderID string
	ReceiveWindow ReceiveWindowOptions
	STPPolicy     STPPolicy
}

type CancelReplaceParams struct {
	Cancel      CancelTarget
	Replacement CancelReplaceOrderParams
	Mode        CancelReplaceMode
}

type DelegatedCancelReplaceParams struct {
	Cancel      CancelTarget
	Replacement CancelReplaceOrderParams
	Mode        CancelReplaceMode
	OnBehalfOf  int
	FeeRateBps  int
}
