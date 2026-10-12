package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type lockedBuffer struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *lockedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.data.String() }

func TestSSHSessionAndAccessRules(t *testing.T) {
	_, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostBlock, err := ssh.MarshalPrivateKey(hostPrivate, "nhl-stats-vim-test")
	if err != nil {
		t.Fatal(err)
	}
	_, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientPrivate)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPrivate)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	keyPath, hostPath := filepath.Join(directory, "authorized_keys"), filepath.Join(directory, "host_key")
	if err := os.WriteFile(keyPath, ssh.MarshalAuthorizedKey(clientSigner.PublicKey()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostPath, pem.EncodeToMemory(hostBlock), 0600); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = Serve(ctx, address, keyPath, hostPath, nil) }()
	config := func(user string, signer ssh.Signer) *ssh.ClientConfig {
		return &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, Timeout: time.Second,
			HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
				if !bytes.Equal(key.Marshal(), hostSigner.PublicKey().Marshal()) {
					return ssh.ErrNoAuth
				}
				return nil
			}}
	}
	var client *ssh.Client
	for attempt := 0; attempt < 40; attempt++ {
		client, err = ssh.Dial("tcp", address, config("nhl", clientSigner))
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("SSH server did not become ready: %v", err)
	}
	defer client.Close()
	if denied, err := ssh.Dial("tcp", address, config("other", clientSigner)); err == nil {
		denied.Close()
		t.Fatal("wrong username was accepted")
	}
	_, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherSigner, err := ssh.NewSignerFromKey(otherPrivate)
	if err != nil {
		t.Fatal(err)
	}
	if denied, err := ssh.Dial("tcp", address, config("nhl", otherSigner)); err == nil {
		denied.Close()
		t.Fatal("unapproved key was accepted")
	}
	command, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Run("id"); err == nil {
		t.Fatal("remote command was accepted")
	}
	command.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var output lockedBuffer
	session.Stdout, session.Stderr = &output, &output
	input, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestPty("xterm", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !strings.Contains(output.String(), "NHL Hockey in your terminal") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(output.String(), "NHL Hockey in your terminal") {
		t.Fatal("authorized session did not render the welcome screen")
	}
	if _, err := input.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
}
