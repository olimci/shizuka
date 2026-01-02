package cmd

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"
)

func runVersion(ctx context.Context, cmd *cli.Command) error {
	fmt.Printf("shizuka version %s\n", Version)
	return nil
}
