package limitless

import "testing"

func TestBuildHMACMessage(t *testing.T) {
	t.Parallel()

	got := buildHMACMessage("2026-03-23T12:34:56.789Z", "post", "/orders?market=btc", `{"foo":"bar"}`)
	want := "2026-03-23T12:34:56.789Z\nPOST\n/orders?market=btc\n{\"foo\":\"bar\"}"
	if got != want {
		t.Fatalf("unexpected HMAC message:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestComputeHMACSignature(t *testing.T) {
	t.Parallel()

	got, err := computeHMACSignature(
		"MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
		"2026-03-23T12:34:56.789Z",
		"POST",
		"/orders?market=btc",
		`{"foo":"bar"}`,
	)
	if err != nil {
		t.Fatalf("computeHMACSignature returned error: %v", err)
	}

	want := "oMeoGw/k9lOlgmdkx4Upqmm99pJ2pHBYkt1CRO5d6bk="
	if got != want {
		t.Fatalf("unexpected signature: want %q, got %q", want, got)
	}
}
