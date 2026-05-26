package database

import (
	"context"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/yourname/go-clean-base/config"
)

type Client struct {
	DB *sqlx.DB
}

func NewClient(cfg *config.Config) (*Client, error) {
	db, err := sqlx.Open("mysql", cfg.Database.DSN)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &Client{DB: db}, nil
}

func (c *Client) Close() error { return c.DB.Close() }
