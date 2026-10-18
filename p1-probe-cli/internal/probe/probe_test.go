package probe

import (
	"context"
	"net"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		resp Response
		want bool
	}{
		{"dns error not retried", Response{err: &net.DNSError{IsNotFound: true}}, false},
		{"context deadline exceed not retried", Response{err: context.DeadlineExceeded}, false},
		{"context canceled not retried", Response{err: context.Canceled}, false},
		{"5xx retried", Response{StatusCode: 503}, true},
		{"429 retried", Response{StatusCode: 429}, true},
		{"404 not retried", Response{StatusCode: 404}, false},
		{"200 not retried", Response{StatusCode: 200}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.resp); got != tt.want {
				t.Errorf("isRetryable(%+v) = %v, want %v", tt.resp, got, tt.want)
			}
		})
	}
}
