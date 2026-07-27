package contacts

import (
	"testing"

	"sirtom/server/errs"
)

func TestIsAccessError(t *testing.T) {
	tests := []struct {
		name string
		err  *errs.GoogleAPIError
		want bool
	}{
		{"nil error", nil, false},
		{"contacts scope not granted", &errs.GoogleAPIError{Code: 403, Status: "PERMISSION_DENIED"}, true},
		{"token rejected", &errs.GoogleAPIError{Code: 401, Status: "UNAUTHENTICATED"}, true},
		{
			"no google account linked",
			&errs.GoogleAPIError{Code: 400, Status: "FAILED_PRECONDITION", Message: "No Google calendar account linked"},
			true,
		},
		{"malformed request", &errs.GoogleAPIError{Code: 400, Status: "INVALID_ARGUMENT"}, false},
		{"google outage", &errs.GoogleAPIError{Code: 500, Status: "INTERNAL"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAccessError(tt.err); got != tt.want {
				t.Errorf("IsAccessError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
