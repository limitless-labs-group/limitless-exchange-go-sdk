package limitless

import "fmt"

// MaxRecvWindowMS is the largest client-supplied receive window accepted by the API.
const MaxRecvWindowMS int64 = 10_000

func normalizeReceiveWindowOptions(opts ReceiveWindowOptions, now func() int64) (ReceiveWindowOptions, error) {
	if opts.Timestamp != nil && *opts.Timestamp < 0 {
		return ReceiveWindowOptions{}, fmt.Errorf("timestamp must be a non-negative integer")
	}

	if opts.RecvWindow != nil {
		if *opts.RecvWindow < 1 || *opts.RecvWindow > MaxRecvWindowMS {
			return ReceiveWindowOptions{}, fmt.Errorf("recvWindow must be between 1 and %d milliseconds", MaxRecvWindowMS)
		}
	}

	if opts.Timestamp == nil && opts.RecvWindow == nil {
		return ReceiveWindowOptions{}, nil
	}

	normalized := opts
	if normalized.RecvWindow != nil && normalized.Timestamp == nil {
		timestamp := now()
		normalized.Timestamp = &timestamp
	}

	return normalized, nil
}
