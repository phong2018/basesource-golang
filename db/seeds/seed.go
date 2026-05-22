package seeds

import (
	"log/slog"
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
)

func Run(db *database.Client) error {
	slog.Info("running seeders")
	if err := seedTodos(db); err != nil {
		return err
	}
	slog.Info("seeders completed")
	return nil
}
