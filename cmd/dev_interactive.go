package cmd

import (
	"context"

	"github.com/olimci/prompter"
	"github.com/urfave/cli/v3"
)

func runDevInteractive(ctx context.Context, cmd *cli.Command) error {
	configPath, cfg, port, err := loadDevConfig(cmd)
	if err != nil {
		return err
	}

	return prompter.Start(func(p *prompter.Prompter) error {
		status, err := p.Status("Starting dev server")
		if err != nil {
			return err
		}

		return runDevServer(p.Ctx, configPath, cfg, port, devServerHooks{
			Log:     p.Log,
			Working: status.Working,
			Idle:    status.Idle,
			Message: status.Message,
		})
	}, prompter.WithContext(ctx))
}
