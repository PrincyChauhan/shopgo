package handlers

import "testing"

// The expected value was computed independently (Python hmac), so this checks
// our signing code against a known-good answer.
func TestSignatureKnownAnswer(t *testing.T) {
	const want = "15656b40fea6f2159b578efa459e969de9f5e223fb8a08393e274ac578d9d005"
	if got := sign("test_secret", "order_ABC|pay_XYZ"); got != want {
		t.Fatalf("sign() = %s, want %s", got, want)
	}
	if !validSignature("test_secret", "order_ABC|pay_XYZ", want) {
		t.Fatal("a correct signature was rejected")
	}
	if validSignature("test_secret", "order_ABC|pay_XYZ", "deadbeef") {
		t.Fatal("a wrong signature was accepted")
	}
	if validSignature("other_secret", "order_ABC|pay_XYZ", want) {
		t.Fatal("a signature from a different secret was accepted")
	}
}
