package server

import (
	"golang.org/x/crypto/ssh"
	"testing"

	"crypto/ed25519"
	"crypto/rand"
)

func TestAuth(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := ParseKeys(append([]byte("# owner\n"), ssh.MarshalAuthorizedKey(sshPub)...))
	if err != nil {
		t.Fatal(err)
	}
	if !keys.Allows(sshPub) {
		t.Fatal("authorized key rejected")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	otherSSH, _ := ssh.NewPublicKey(other)
	if keys.Allows(otherSSH) {
		t.Fatal("unauthorized key accepted")
	}
	if _, err = ParseKeys([]byte("garbage")); err == nil {
		t.Fatal("invalid key accepted")
	}
	if _, err = ParseKeys([]byte("# only comment")); err == nil {
		t.Fatal("empty keys accepted")
	}
}
