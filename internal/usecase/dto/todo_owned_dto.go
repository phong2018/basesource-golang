package dto

type CreateOwnedTodoInput struct {
	OwnerID     int64   `json:"-"`
	Title       string  `json:"title"       validate:"required,max=255"`
	Description *string `json:"description"`
}

type UpdateOwnedTodoInput struct {
	ID      uint   `json:"-"`
	OwnerID int64  `json:"-"`
	Title   string `json:"title" validate:"required,max=255"`
	Done    bool   `json:"done"`
}

type OwnedTodoOutput struct {
	ID            uint    `json:"id"`
	Title         string  `json:"title"`
	Description   *string `json:"description"`
	Done          bool    `json:"done"`
	AttachmentURL *string `json:"attachment_url"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`
}

type BulkDeleteInput struct {
	IDs []uint `json:"ids" validate:"required,min=1"`
}

type BulkStatusInput struct {
	IDs  []uint `json:"ids"  validate:"required,min=1"`
	Done bool   `json:"done"`
}

type ShareTodoInput struct {
	TodoID      uint   `json:"-"`
	OwnerID     int64  `json:"-"`
	TargetEmail string `json:"email" validate:"required,email"`
}

type AddCommentInput struct {
	TodoID   uint   `json:"-"`
	CallerID int64  `json:"-"`
	Body     string `json:"body" validate:"required,max=2000"`
}

type CommentOutput struct {
	ID        uint   `json:"id"`
	TodoID    uint   `json:"todo_id"`
	UserID    int64  `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}
