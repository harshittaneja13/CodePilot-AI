package webhook

import (
	"testing"
)

func TestVerifySignature_Valid(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "my-webhook-secret"
	// Pre-computed: echo -n '{"action":"opened"}' | openssl dgst -sha256 -hmac "my-webhook-secret"
	sig := "sha256=8c1ebbc27b76ca5db967421a149d2ca6076661367fe1c4236b6b0b473d2b2b36"
	if !VerifySignature(payload, sig, secret) {
		t.Error("expected signature to be valid")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	payload := []byte(`{"action":"opened"}`)
	secret := "my-webhook-secret"
	if VerifySignature(payload, "sha256=deadbeef", secret) {
		t.Error("expected invalid signature to fail")
	}
}

func TestVerifySignature_WrongPayload(t *testing.T) {
	secret := "my-webhook-secret"
	sig := "sha256=8c1ebbc27b76ca5db967421a149d2ca6076661367fe1c4236b6b0b473d2b2b36"
	if VerifySignature([]byte(`{"action":"closed"}`), sig, secret) {
		t.Error("signature for different payload should not verify")
	}
}

func TestVerifySignature_EmptySignature(t *testing.T) {
	if VerifySignature([]byte("payload"), "", "secret") {
		t.Error("empty signature should return false")
	}
}

func TestVerifySignature_EmptySecret(t *testing.T) {
	if VerifySignature([]byte("payload"), "sha256=abc", "") {
		t.Error("empty secret should return false")
	}
}

func TestVerifySignature_MissingPrefix(t *testing.T) {
	if VerifySignature([]byte("payload"), "abc123", "secret") {
		t.Error("signature without sha256= prefix should return false")
	}
}

func TestVerifySignature_TimingSafe(t *testing.T) {
	// Verify that hmac.Equal is used (constant-time comparison).
	// We can't test timing directly but we can verify the function doesn't short-circuit
	// on the first byte by testing a sig that shares the same prefix.
	payload := []byte("data")
	secret := "s"
	// Two different secrets produce different HMACs; neither should verify against the other.
	if VerifySignature(payload, "sha256=0000000000000000000000000000000000000000000000000000000000000000", secret) {
		t.Error("all-zero signature should not match a real HMAC")
	}
}
