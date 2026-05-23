package main

import (
	"github.com/yourname/go-clean-base/cmd/api"
	"github.com/yourname/go-clean-base/cmd/migrate"
	"github.com/yourname/go-clean-base/cmd/seed"
	"github.com/yourname/go-clean-base/cmd/worker"
	"github.com/spf13/cobra"
	"os"
	"log"
)

func main() {
	root := &cobra.Command{Use: "app"}
	root.AddCommand(api.NewAPICommand())
	root.AddCommand(migrate.NewMigrateCommand())
	root.AddCommand(seed.NewSeedCommand())
	root.AddCommand(worker.NewWorkerCommand())
	if err := root.Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
