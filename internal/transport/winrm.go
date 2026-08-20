package transport

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/masterzen/winrm"

	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/config"
)

type winrmRunner struct {
	client *winrm.Client
}

func NewWinRMRunner(cfg config.Config) (Runner, error) {
	endpoint := winrm.NewEndpoint(
		cfg.Host,
		cfg.Port,
		cfg.WinRMUseTLS,
		cfg.Insecure,
		nil,
		nil,
		nil,
		0,
	)

	params := winrm.DefaultParameters
	switch strings.ToLower(cfg.WinRMAuth) {
	case "", "basic":
	case "ntlm":
		params.TransportDecorator = func() winrm.Transporter {
			return &winrm.ClientNTLM{}
		}
	case "kerberos":
		proto := "http"
		if cfg.WinRMUseTLS {
			proto = "https"
		}

		spn := cfg.WinRMKerberosSPN
		if spn == "" {
			spn = fmt.Sprintf("HTTP/%s", cfg.Host)
		}

		params.TransportDecorator = func() winrm.Transporter {
			return &winrm.ClientKerberos{
				Username:  cfg.Username,
				Password:  cfg.Password,
				Realm:     cfg.WinRMKerberosRealm,
				Hostname:  cfg.Host,
				Port:      cfg.Port,
				Proto:     proto,
				SPN:       spn,
				KrbConf:   cfg.WinRMKerberosConfig,
				KrbCCache: cfg.WinRMKerberosCCache,
			}
		}
	default:
		return nil, fmt.Errorf("unsupported WinRM auth %q", cfg.WinRMAuth)
	}

	client, err := winrm.NewClientWithParameters(endpoint, cfg.Username, cfg.Password, params)
	if err != nil {
		return nil, fmt.Errorf("creating WinRM client: %w", err)
	}

	return &winrmRunner{client: client}, nil
}

func (r *winrmRunner) Run(ctx context.Context, command string) (Result, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode, err := r.client.RunWithContext(ctx, command, &stdout, &stderr)
	result := Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}

	if err != nil {
		return result, err
	}

	return result, nil
}
