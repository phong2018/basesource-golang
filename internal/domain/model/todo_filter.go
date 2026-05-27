package model

// Value Object: TodoFilter describes query criteria for filtering todos.
// No identity, no db: tags — compared by value.
type TodoFilter struct {
	Done   *bool
	Search *string
	SortBy *string
}
