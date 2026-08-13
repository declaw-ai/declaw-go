package declaw

import (
	"crypto/rand"
	"encoding/hex"
)

// Machine-readable error codes the API returns on POST /sandboxes.
//
// Exported so callers can branch without hardcoding strings; compare against
// SandboxError.Code.
//
// Branch on these, never on the message. 409 means two unrelated things on this
// endpoint, and only one of them is retryable.
const (
	// CodeIdempotencyInProgress (409): the original create carrying this key is
	// still running. Retrying the IDENTICAL request is correct and is how a
	// caller recovers the sandbox ID after a lost response. Retry-After is set.
	CodeIdempotencyInProgress = "idempotency_in_progress"

	// CodeIdempotencyKeyReused (422): this key was already used with different
	// parameters. Not retryable — the caller must generate a fresh key per
	// logical create.
	CodeIdempotencyKeyReused = "idempotency_key_reused"

	// CodeTemplateNotReady (409): unrelated to idempotency, needs a template
	// rebuild. Retrying unchanged cannot fix it.
	CodeTemplateNotReady = "template_not_ready"
)

// newIdempotencyKey returns a random 128-bit key as a UUIDv4-shaped string.
//
// Generated ONCE per logical create and reused across that call's retries — a
// fresh key per attempt would defeat the entire mechanism, since the server
// would see each retry as a new create and the caller would get the duplicate
// sandboxes this exists to prevent.
//
// crypto/rand rather than math/rand because the key is a correctness boundary:
// a collision between two tenants' concurrent creates would return one caller
// the other's sandbox. math/rand's global source is seeded per process and its
// output is predictable, which is the wrong property here. Formatted as a UUID
// only for operator familiarity in logs; nothing parses it as one.
//
// Returns "" if the system CSPRNG fails. Callers send no header in that case
// rather than sending a weak key — the request then behaves exactly as it did
// before idempotency existed.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant

	h := make([]byte, 32)
	hex.Encode(h, b[:])
	return string(h[0:8]) + "-" + string(h[8:12]) + "-" + string(h[12:16]) + "-" +
		string(h[16:20]) + "-" + string(h[20:32])
}
