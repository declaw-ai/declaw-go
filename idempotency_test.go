package declaw

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"
)

var uuidV4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewIdempotencyKey(t *testing.T) {
	t.Run("uuidv4 shape", func(t *testing.T) {
		k := newIdempotencyKey()
		if !uuidV4.MatchString(k) {
			t.Fatalf("key %q is not a v4 UUID (version/variant bits must be set)", k)
		}
	})

	// A collision between two tenants' concurrent creates would hand one caller
	// the other's sandbox, so this is a correctness boundary, not cosmetics.
	t.Run("unique across many", func(t *testing.T) {
		const n = 10000
		seen := make(map[string]bool, n)
		for i := 0; i < n; i++ {
			k := newIdempotencyKey()
			if seen[k] {
				t.Fatalf("duplicate key after %d draws: %s", i, k)
			}
			seen[k] = true
		}
	})
}

// THE TEST THIS WHOLE CHANGE RESTS ON.
//
// The key must be generated once per logical create and reused across retries.
// Generating it per attempt would leave every retry looking like a brand-new
// create to the server — exactly the duplicate-sandbox bug (#649) the header
// exists to fix — and every other test here would still pass.
func TestCreate_ReusesOneKeyAcrossRetries(t *testing.T) {
	var mu sync.Mutex
	var keys []string
	attempts := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		attempts++
		n := attempts
		mu.Unlock()

		if n < 3 { // two transient failures, then success
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"sandbox_id": "sbx-1", "status": "running"})
	}))
	defer srv.Close()

	_, err := Create(context.Background(), WithSandboxAPIKey("k"), WithSandboxAPIURL(srv.URL))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(keys) < 2 {
		t.Fatalf("expected retries, saw %d attempt(s)", len(keys))
	}
	for i, k := range keys {
		if k == "" {
			t.Fatalf("attempt %d sent no Idempotency-Key", i)
		}
		if k != keys[0] {
			t.Fatalf("attempt %d used a DIFFERENT key (%s vs %s) — every retry would create a new sandbox",
				i, k, keys[0])
		}
	}
}

// 409 means two unrelated things on this endpoint. Only idempotency_in_progress
// is retryable; retrying template_not_ready is pointless and just burns the
// caller's budget waiting for something that will never change.
func TestCreate_RetriesOnlyTheRetryable409(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		code        string
		wantRetried bool
	}{
		{"idempotency_in_progress retries", http.StatusConflict, CodeIdempotencyInProgress, true},
		{"template_not_ready does not", http.StatusConflict, CodeTemplateNotReady, false},
		{"key_reused does not", http.StatusUnprocessableEntity, CodeIdempotencyKeyReused, false},
		{"409 with no code does not", http.StatusConflict, "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			attempts := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				attempts++
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				json.NewEncoder(w).Encode(map[string]any{"message": "nope", "code": tc.code})
			}))
			defer srv.Close()

			_, err := Create(context.Background(), WithSandboxAPIKey("k"), WithSandboxAPIURL(srv.URL))
			if err == nil {
				t.Fatal("expected an error")
			}

			mu.Lock()
			n := attempts
			mu.Unlock()

			if tc.wantRetried && n < 2 {
				t.Fatalf("%s: only %d attempt(s) — the caller never gets the chance to recover the sandbox ID", tc.code, n)
			}
			if !tc.wantRetried && n != 1 {
				t.Fatalf("%s: %d attempts, want exactly 1 — retrying cannot fix this", tc.code, n)
			}
		})
	}
}

// The code is what callers branch on. Parsing only `message` (as this SDK did)
// makes the two different 409s indistinguishable without string matching.
func TestErrorCodeIsParsed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"message": "key reused with different parameters",
			"code":    CodeIdempotencyKeyReused,
		})
	}))
	defer srv.Close()

	_, err := Create(context.Background(), WithSandboxAPIKey("k"), WithSandboxAPIURL(srv.URL))
	if err == nil {
		t.Fatal("expected an error")
	}
	var se *SandboxError
	if !errors.As(err, &se) {
		t.Fatalf("error %T does not carry a *SandboxError", err)
	}
	if se.Code != CodeIdempotencyKeyReused {
		t.Fatalf("Code = %q, want %q", se.Code, CodeIdempotencyKeyReused)
	}
}

func TestRetryJitter(t *testing.T) {
	// Equal jitter: never below half, never above the full delay. Below half
	// would hammer the server harder than configured; above would stretch the
	// caller's deadline past what maxRetries implies.
	t.Run("stays within [d/2, d]", func(t *testing.T) {
		const d = 400 * time.Millisecond
		for i := 0; i < 500; i++ {
			got := retryJitter(d)
			if got < d/2 || got > d {
				t.Fatalf("jitter %v outside [%v, %v]", got, d/2, d)
			}
		}
	})

	// The entire point: two clients failing together must not come back in
	// lockstep. A constant would pass every bound check above.
	t.Run("actually varies", func(t *testing.T) {
		seen := make(map[time.Duration]bool)
		for i := 0; i < 200; i++ {
			seen[retryJitter(time.Second)] = true
		}
		if len(seen) < 10 {
			t.Fatalf("only %d distinct delays in 200 draws — retries are still synchronised", len(seen))
		}
	})

	t.Run("zero stays zero", func(t *testing.T) {
		if got := retryJitter(0); got != 0 {
			t.Fatalf("retryJitter(0) = %v", got)
		}
	})
}

func TestRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		hdr    string
		want   time.Duration
		wantOK bool
	}{
		{"", 0, false},
		{"2", 2 * time.Second, true},
		// A genuine 0 means "retry now" and must be distinguishable from absent,
		// or the caller silently falls back to its own backoff instead.
		{"0", 0, true},
		{"-5", 0, false},
		{"garbage", 0, false},
		{"Wed, 21 Oct 2026 07:28:00 GMT", 0, false}, // HTTP-date form unsupported
		{"9999", 60 * time.Second, true},            // clamped
	} {
		resp := &http.Response{Header: http.Header{}}
		if tc.hdr != "" {
			resp.Header.Set("Retry-After", tc.hdr)
		}
		got, ok := retryAfter(resp)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("Retry-After %q -> (%v, %v), want (%v, %v)", tc.hdr, got, ok, tc.want, tc.wantOK)
		}
	}
}
