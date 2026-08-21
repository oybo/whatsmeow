package mobilereg

import "testing"

func TestCalculateRegistrationToken(t *testing.T) {
	token, err := CalculateRegistrationToken("53085444")
	if err != nil {
		t.Fatalf("CalculateRegistrationToken returned error: %v", err)
	}
	const expected = "/DU9PZ+wyR95OBdD2esLWsFC2s4="
	if token != expected {
		t.Fatalf("unexpected token: got %q, want %q", token, expected)
	}
}
