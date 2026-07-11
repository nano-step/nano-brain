package codesummarize

import (
	"errors"
	"testing"
)

// TestClassifyError_524 covers #552 D5: Cloudflare 524 ("A Timeout Occurred")
// must classify as transient/retryable, alongside the existing transient
// codes, while permanent client-error codes remain non-transient.
func TestClassifyError_524(t *testing.T) {
	tests := []struct {
		name    string
		errText string
		want    ErrorClass
	}{
		{"524 cloudflare timeout is transient", "unexpected status code: 524", ErrorTransient},
		{"503 service unavailable is transient", "unexpected status code: 503", ErrorTransient},
		{"429 too many requests is transient", "unexpected status code: 429", ErrorTransient},
		{"400 bad request is not transient", "unexpected status code: 400", ErrorPermanent},
		{"404 not found is not transient", "unexpected status code: 404", ErrorPermanent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyError(errors.New(tt.errText))
			if got != tt.want {
				t.Errorf("ClassifyError(%q) = %v, want %v", tt.errText, got, tt.want)
			}
		})
	}
}
