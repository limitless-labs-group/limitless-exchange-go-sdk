package limitless

import (
	"encoding/json"
	"fmt"
)

// RawResult pairs a decoded response value with the full raw HTTP response
// (status code, headers, and original body). It is returned by the opt-in
// ...WithRawResponse variant of every API-backed service method, keeping the
// common path allocation-free while making status and header inspection
// available when a caller needs it.
type RawResult[T any] struct {
	// Data is the decoded response value, identical to what the base method returns.
	Data T
	// Raw is the underlying HTTP response: Status, Headers, and the original Body.
	Raw *RawResponse
}

// decodeRawResult unmarshals raw.Body into a value of type T and pairs it with
// the full raw response. It mirrors decodeServerWalletOperationResponse for
// error and empty-body handling, and additionally invokes setRawJSON when T
// preserves its original body (e.g. server-wallet split/merge responses).
func decodeRawResult[T any](raw *RawResponse, err error) (*RawResult[T], error) {
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, fmt.Errorf("empty raw response")
	}

	var data T
	if len(raw.Body) > 0 {
		if err := json.Unmarshal(raw.Body, &data); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	if setter, ok := any(&data).(interface{ setRawJSON(json.RawMessage) }); ok {
		setter.setRawJSON(raw.Body)
	}

	return &RawResult[T]{Data: data, Raw: raw}, nil
}
