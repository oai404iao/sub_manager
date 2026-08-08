package share

import "testing"

func TestSignerRejectsTampering(t *testing.T) {
	signer := New("01234567890123456789012345678901")
	signature := signer.Sign("kind=node&id=1&content=nodes&exp=123")
	if !signer.Verify("kind=node&id=1&content=nodes&exp=123", signature) {
		t.Fatal("valid signature was rejected")
	}
	if signer.Verify("kind=node&id=2&content=nodes&exp=123", signature) {
		t.Fatal("tampered message was accepted")
	}
}
