package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Commands: []*cli.Command{
			{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "display version",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					fmt.Println("0.0.1")
					return nil
				},
			},
			{
				Name:    "start",
				Aliases: []string{"t"},
				Usage:   "options for task templates",
				Action: func(ctx context.Context, c *cli.Command) error {
					return nil
				},
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Println(err)
	}
}
