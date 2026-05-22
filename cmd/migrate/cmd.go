package migrate

import (
	"log/slog"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
	"github.com/yourname/go-clean-base/config"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

const migrationsDir = "db/migrations"

func NewMigrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations (goose up)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.NewConfig()
			if err != nil {
				return err
			}
			cl, err := database.NewClient(cfg)
			if err != nil {
				return err
			}
			defer func() {
				if err := cl.Close(); err != nil {
					slog.Error("close db", "err", err)
				}
			}()
			if err := goose.SetDialect("mysql"); err != nil {
				return err
			}
			return goose.Up(cl.DB.DB, migrationsDir)
		},
	}
}
