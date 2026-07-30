package runner

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"fastlogin/internal/config"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
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

// defaultKeyFiles are the conventional SSH private-key locations tried when
// the user requests pem: default.
var defaultKeyFiles = []string{
	"~/.ssh/id_ed25519",
	"~/.ssh/id_ecdsa",
	"~/.ssh/id_rsa",
	"~/.ssh/id_dsa",
}

// defaultAuthMethods returns auth methods derived from the local machine's SSH
// configuration: the SSH agent, then any host-specific IdentityFile from
// ~/.ssh/config, then the conventional default private-key files under ~/.ssh/.
func defaultAuthMethods(e config.Entry) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	// 1. SSH agent — the conn is kept open; signers query it lazily during Dial.
	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		if conn, err := net.Dial("unix", socket); err == nil {
			agentClient := agent.NewClient(conn)
			if signers, err := agentClient.Signers(); err == nil && len(signers) > 0 {
				methods = append(methods, ssh.PublicKeys(signers...))
			}
		}
	}

	// 2. Host-specific IdentityFile from ~/.ssh/config.
	if pem := config.SSHConfigIdentityFile(e.Host); pem != "" {
		if path, err := config.ExpandPath(pem); err == nil {
			if key, err := os.ReadFile(path); err == nil {
				if signer, err := ssh.ParsePrivateKey(key); err == nil {
					methods = append(methods, ssh.PublicKeys(signer))
				}
			}
		}
	}

	// 3. Default key files under ~/.ssh/.
	for _, p := range defaultKeyFiles {
		path, err := config.ExpandPath(p)
		if err != nil {
			continue
		}
		key, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			continue
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	return methods
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
	if e.Auth.PEM == "default" {
		methods = append(methods, defaultAuthMethods(e)...)
	} else if e.Auth.PEM != "" {
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
