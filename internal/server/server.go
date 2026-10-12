package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/Malvore/nhl-stats-vim/internal/nhl"
	"github.com/Malvore/nhl-stats-vim/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
)

type Keys struct{ allowed map[string]struct{} }

func ParseKeys(data []byte) (Keys, error) {
	k := Keys{allowed: map[string]struct{}{}}
	for i, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		pub, _, _, rest, err := ssh.ParseAuthorizedKey(line)
		if err != nil || len(bytes.TrimSpace(rest)) != 0 {
			return Keys{}, fmt.Errorf("authorized keys line %d: invalid public key", i+1)
		}
		k.allowed[string(pub.Marshal())] = struct{}{}
	}
	if len(k.allowed) == 0 {
		return Keys{}, errors.New("no authorized SSH keys")
	}
	return k, nil
}
func (k Keys) Allows(pub ssh.PublicKey) bool { _, ok := k.allowed[string(pub.Marshal())]; return ok }
func Serve(ctx context.Context, addr, keyFile, hostKeyFile string, log *slog.Logger) error {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("read authorized keys: %w", err)
	}
	keys, err := ParseKeys(data)
	if err != nil {
		return err
	}
	hostPEM, err := os.ReadFile(hostKeyFile)
	if err != nil {
		return fmt.Errorf("read SSH host key: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	client := nhl.NewClient(log)
	guard := func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			if len(s.Command()) > 0 || s.Subsystem() != "" {
				wish.Fatalln(s, "NHL Stats Vim does not accept commands")
				return
			}
			log.Info("SSH session open", "user", s.User(), "remote", s.RemoteAddr().String())
			defer log.Info("SSH session close", "user", s.User())
			next(s)
		}
	}
	handler := func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		return ui.NewWithContext(s.Context(), client), []tea.ProgramOption{tea.WithAltScreen()}
	}
	srv, err := wish.NewServer(wish.WithAddress(addr), wish.WithHostKeyPEM(hostPEM), wish.WithPublicKeyAuth(func(c ssh.Context, p ssh.PublicKey) bool { return c.User() == "nhl" && keys.Allows(p) }), wish.WithIdleTimeout(30*time.Minute), wish.WithMaxTimeout(4*time.Hour), wish.WithMiddleware(bm.Middleware(handler), guard))
	if err != nil {
		return fmt.Errorf("create SSH server: %w", err)
	}
	log.Info("NHL Stats Vim SSH listening", "address", addr, "authorized_keys", len(keys.allowed))
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err = srv.ListenAndServe(); err != nil && !strings.Contains(err.Error(), "Server closed") {
		return fmt.Errorf("SSH server: %w", err)
	}
	return nil
}
