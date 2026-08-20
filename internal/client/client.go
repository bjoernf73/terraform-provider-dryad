package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/config"
	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/powershell"
	"github.com/bjoernf73/dry.module.ad/tf/terraform-provider-dryad/internal/transport"
)

type Client struct {
	config config.Config
	runner transport.Runner
}

func New(cfg config.Config) (*Client, error) {
	runner, err := transport.NewRunner(cfg)
	if err != nil {
		return nil, err
	}

	return &Client{
		config: cfg,
		runner: runner,
	}, nil
}

func (c *Client) Config() config.Config {
	return c.config
}

func (c *Client) RunPowerShell(ctx context.Context, script string) (transport.Result, error) {
	command, err := powershell.BuildCommand(c.config.PowerShellPath, script)
	if err != nil {
		return transport.Result{}, err
	}

	result, err := c.runner.Run(ctx, command)
	result.Stderr = powershell.DecodeCLIXML(result.Stderr)

	if err != nil {
		return result, fmt.Errorf("running remote PowerShell: %w", err)
	}

	if result.ExitCode != 0 {
		errText := strings.TrimSpace(result.Stderr)
		if errText == "" {
			errText = strings.TrimSpace(result.Stdout)
		}

		return result, fmt.Errorf("remote PowerShell exited with code %d: %s", result.ExitCode, errText)
	}

	return result, nil
}

func (c *Client) RunPowerShellJSON(ctx context.Context, script string, target any) error {
	result, err := c.RunPowerShell(ctx, script)
	if err != nil {
		return err
	}

	if strings.TrimSpace(result.Stdout) == "" {
		return fmt.Errorf("remote PowerShell returned no JSON output")
	}

	if err := json.Unmarshal([]byte(result.Stdout), target); err != nil {
		return fmt.Errorf("decoding remote JSON output: %w; output: %s", err, result.Stdout)
	}

	return nil
}
