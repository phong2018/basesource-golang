package model

// Value Object: Pagination holds page/limit parameters for list queries.
type Pagination struct {
	Page  int
	Limit int
}

func (p Pagination) Offset() int {
	if p.Page <= 1 {
		return 0
	}
	return (p.Page - 1) * p.Limit
}
