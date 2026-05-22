package seeds

import (
	"github.com/yourname/go-clean-base/internal/infrastructure/database"
	"github.com/yourname/go-clean-base/pkg/helper"
)

func seedTodos(db *database.Client) error {
	todos := []struct {
		Title       string
		Description *string
		Done        bool
	}{
		{"Buy groceries", helper.Ptr("Milk, eggs, bread"), false},
		{"Write unit tests", helper.Ptr("Cover usecase layer"), false},
		{"Deploy to staging", nil, true},
	}

	for _, t := range todos {
		_, err := db.DB.Exec(
			"INSERT INTO todos (title, description, done) VALUES (?, ?, ?)",
			t.Title, t.Description, t.Done,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
