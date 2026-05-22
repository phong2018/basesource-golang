package database

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	"github.com/yourname/go-clean-base/config"
	_ "github.com/go-sql-driver/mysql"
)

type Client struct {
	DB *sqlx.DB
}

func NewClient(cfg *config.Config) (*Client, error) {
	db, err := sqlx.Open("mysql", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Client{DB: db}, nil
}

func (c *Client) Close() error { return c.DB.Close() }
