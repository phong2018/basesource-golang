package dto

type CreateTodoInput struct {
	Title       string  `json:"title"       validate:"required,max=255"`
	Description *string `json:"description"`
}

type UpdateTodoInput struct {
	ID          uint    `json:"-"`
	Title       string  `json:"title"       validate:"required,max=255"`
	Description *string `json:"description"`
	Done        bool    `json:"done"`
}

type ListTodoInput struct {
	Done   *bool   `query:"done"`
	Search *string `query:"search"`
	SortBy *string `query:"sort_by"`
	Page   int     `query:"page"  validate:"min=1"`
	Limit  int     `query:"limit" validate:"min=1,max=100"`
}

type TodoOutput struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Done        bool    `json:"done"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}
