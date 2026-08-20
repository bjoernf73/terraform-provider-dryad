package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/config"
)

type sshRunner struct {
	config config.Config
}

func NewSSHRunner(cfg config.Config) (Runner, error) {
	if strings.TrimSpace(cfg.Username) == "" {
		return nil, fmt.Errorf("username is required for SSH transport")
	}

	if strings.TrimSpace(cfg.Password) == "" && strings.TrimSpace(cfg.SSHPrivateKeyPEM) == "" {
		return nil, fmt.Errorf("either password or ssh_private_key_pem is required for SSH transport")
	}

	return &sshRunner{config: cfg}, nil
}

func (r *sshRunner) Run(_ context.Context, command string) (Result, error) {
	sshConfig, err := r.buildClientConfig()
	if err != nil {
		return Result{}, err
	}

	address := net.JoinHostPort(r.config.Host, fmt.Sprintf("%d", r.config.Port))
	conn, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return Result{}, fmt.Errorf("dialing SSH target: %w", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("creating SSH session: %w", err)
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	session.Stdout = &stdout
	session.Stderr = &stderr

	runErr := session.Run(command)
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if runErr == nil {
		return result, nil
	}

	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitStatus()
		return result, nil
	}

	return result, runErr
}

func (r *sshRunner) buildClientConfig() (*ssh.ClientConfig, error) {
	authMethods := make([]ssh.AuthMethod, 0, 2)

	if r.config.Password != "" {
		authMethods = append(authMethods, ssh.Password(r.config.Password))
	}

	if r.config.SSHPrivateKeyPEM != "" {
		signer, err := ssh.ParsePrivateKey([]byte(r.config.SSHPrivateKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parsing ssh_private_key_pem: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	hostKeyCallback, err := r.hostKeyCallback()
	if err != nil {
		return nil, err
	}

	timeout := r.config.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return &ssh.ClientConfig{
		User:            r.config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}, nil
}

func (r *sshRunner) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if r.config.Insecure {
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if r.config.SSHKnownHostsPath != "" {
		callback, err := knownhosts.New(r.config.SSHKnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("loading known hosts: %w", err)
		}
		return callback, nil
	}

	if r.config.SSHHostKey != "" {
		publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.config.SSHHostKey))
		if err != nil {
			return nil, fmt.Errorf("parsing ssh_host_key: %w", err)
		}

		return func(_ string, _ net.Addr, presented ssh.PublicKey) error {
			if bytes.Equal(publicKey.Marshal(), presented.Marshal()) {
				return nil
			}
			return fmt.Errorf("ssh host key mismatch")
		}, nil
	}

	homeDir, err := os.UserHomeDir()
	if err == nil {
		defaultKnownHosts := homeDir + "/.ssh/known_hosts"
		if _, statErr := os.Stat(defaultKnownHosts); statErr == nil {
			callback, callbackErr := knownhosts.New(defaultKnownHosts)
			if callbackErr == nil {
				return callback, nil
			}
		}
	}

	return nil, fmt.Errorf("ssh host key verification is required unless insecure is true")
}
