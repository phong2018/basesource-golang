package seed

import (
	"github.com/spf13/cobra"
	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/db/seeds"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

func NewSeedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Run database seeders (dev/test only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfig()
			if err != nil {
				return err
			}
			cl, err := database.NewClient(cfg)
			if err != nil {
				return err
			}
			defer func() { _ = cl.Close() }()
			return seeds.Run(cl)
		},
	}
}
