package reqctx

import (
	"bytes"
	"context"
	"log"
	"testing"
)

func captureLog(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	prevOut := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return buf.String
}

func baseCtx() context.Context {
	ctx := context.WithValue(context.Background(), RemoteAddrKey, "127.0.0.1")
	return context.WithValue(ctx, RequestIDKey, 42)
}

func TestLog(t *testing.T) {
	t.Run("RQ-01_UserIDInContextIsLogged", func(t *testing.T) {
		getLog := captureLog(t)
		ctx := context.WithValue(baseCtx(), UserIDKey, "u1")

		Log(ctx, "hello", "world")

		if got, want := getLog(), "42 u1 127.0.0.1 hello world\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("RQ-02_MissingUserIDLogsNA", func(t *testing.T) {
		getLog := captureLog(t)

		Log(baseCtx(), "hello", "world")

		if got, want := getLog(), "42 N/A 127.0.0.1 hello world\n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("RQ-03_NoMessageLogsEmptySegment", func(t *testing.T) {
		getLog := captureLog(t)

		Log(baseCtx())

		if got, want := getLog(), "42 N/A 127.0.0.1 \n"; got != want {
			t.Errorf("log = %q, want %q", got, want)
		}
	})

	t.Run("RQ-04_MissingRemoteAddrPanics", func(t *testing.T) {
		getLog := captureLog(t)
		ctx := context.WithValue(context.Background(), RequestIDKey, 42)

		defer func() {
			if recover() == nil {
				t.Error("expected a panic for a missing remote address")
			}
			if out := getLog(); out != "" {
				t.Errorf("expected nothing logged, got %q", out)
			}
		}()
		Log(ctx, "msg")
	})

	t.Run("RQ-05_MissingRequestIDPanics", func(t *testing.T) {
		getLog := captureLog(t)
		ctx := context.WithValue(context.Background(), RemoteAddrKey, "127.0.0.1")

		defer func() {
			if recover() == nil {
				t.Error("expected a panic for a missing request ID")
			}
			if out := getLog(); out != "" {
				t.Errorf("expected nothing logged, got %q", out)
			}
		}()
		Log(ctx, "msg")
	})
}

func TestKeys(t *testing.T) {
	t.Run("RQ-06_KeyValuesFollowIotaOrder", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			got  key
			want key
		}{
			{"SessionKey", SessionKey, 0},
			{"RequestIDKey", RequestIDKey, 1},
			{"RemoteAddrKey", RemoteAddrKey, 2},
			{"UserIDKey", UserIDKey, 3},
		} {
			if tc.got != tc.want {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		}
	})
}
