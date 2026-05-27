package model

// Value Object: JWT claims extracted from a validated access token.
type TokenClaims struct {
	UserID int64
	Role   string
}
