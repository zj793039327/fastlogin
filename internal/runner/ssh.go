package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"fastlogin/internal/config"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// SSHRunner connects via the ssh library and attaches the local terminal.
type SSHRunner struct{}

func (r *SSHRunner) Run(ctx context.Context, e config.Entry) error {
	methods, err := authMethods(e)
	if err != nil {
		return err
	}
	cfg := &ssh.ClientConfig{
		User:            e.User,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := net.JoinHostPort(e.Host, fmt.Sprintf("%d", e.Port))

	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh session: %w", err)
	}
	defer session.Close()

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return fmt.Errorf("stdin is not a terminal")
	}
	if restore := makeRawStdin(); restore != nil {
		defer restore()
	}

	w, h, _ := term.GetSize(fd)
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", h, w, modes); err != nil {
		return fmt.Errorf("request pty: %w", err)
	}
	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Shell(); err != nil {
		return fmt.Errorf("start shell: %w", err)
	}
	return session.Wait()
}

// authMethods builds ssh.AuthMethod list from the entry's Auth.
func authMethods(e config.Entry) ([]ssh.AuthMethod, error) {
	if e.Auth == nil {
		return nil, fmt.Errorf("entry %q has no auth", e.Name)
	}
	var methods []ssh.AuthMethod
	if e.Auth.Password != "" {
		methods = append(methods, ssh.Password(e.Auth.Password))
	}
	if e.Auth.PEM != "" {
		path, err := config.ExpandPath(e.Auth.PEM)
		if err != nil {
			return nil, fmt.Errorf("expand pem path: %w", err)
		}
		key, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read pem %s: %w", path, err)
		}
		var signer ssh.Signer
		if e.Auth.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(e.Auth.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(key)
		}
		if err != nil {
			return nil, fmt.Errorf("parse pem: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, fmt.Errorf("entry %q auth has neither password nor pem", e.Name)
	}
	return methods, nil
}
