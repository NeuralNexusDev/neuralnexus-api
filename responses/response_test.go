package responses

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRR01_TooManyRequestsRetryAfterIsHTTPDate(t *testing.T) {
	prev := time.Local
	time.Local = time.FixedZone("EDT", -4*60*60)
	t.Cleanup(func() { time.Local = prev })

	t.Run("RR-01_RetryAfterParsesAsGMTHTTPDate", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		before := time.Now()

		TooManyRequests(w, r, 60, "slow down")

		if w.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
		}
		got := w.Header().Get("Retry-After")
		if !strings.HasSuffix(got, " GMT") {
			t.Errorf("Retry-After = %q, want an HTTP-date ending in GMT", got)
		}
		parsed, err := http.ParseTime(got)
		if err != nil {
			t.Fatalf("Retry-After = %q does not parse as an HTTP-date: %v", got, err)
		}
		if d := parsed.Sub(before); d < 59*time.Second || d > 62*time.Second {
			t.Errorf("Retry-After = %v is %v after the request, want about 60s", parsed, d)
		}
	})
}
