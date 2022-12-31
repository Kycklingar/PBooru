package session

import (
	"testing"
)

func TestGenerateKey(t *testing.T) {
	k := generateKey(64)
	if len(k) != 64 {
		t.Fatal("generated key of incorrect length")
	}

	t.Log(k)
}
