# T14 — Unit Tests: Todo Ownership Usecase

**Status:** [ ] Not started

## Depends on
- T11 (todo_owned_usecase_impl)

## File to create

### `internal/usecase/todo_owned_usecase_impl_test.go`

Use mocks for `ITodoOwnedRepository`, `IUserRepository`, `ITodoCommentRepository`, `IFileStorage`.

| Test case | Input | Expected |
|---|---|---|
| `GetMine_ok` | correct ownerID | returns `OwnedTodoOutput` |
| `GetMine_wrongOwner` | ownerID doesn't match todo | `apperror.Forbidden` |
| `GetMine_notFound` | non-existent todo ID | `apperror.NotFound` |
| `CreateMine_ok` | valid input with ownerID | returns created `OwnedTodoOutput` |
| `UpdateMine_wrongOwner` | ownerID doesn't match | `apperror.Forbidden` |
| `DeleteMine_softDeletes` | valid ownerID | `SoftDeleteOwned` called, no error |
| `DeleteMine_wrongOwner` | ownerID doesn't match | `apperror.Forbidden` |
| `BulkDelete_ok` | valid IDs | `BulkSoftDelete` called, no error |
| `BulkSetStatus_ok` | valid IDs + done=true | `BulkSetStatus` called, no error |
| `ShareTodo_ok` | valid ownerID + existing email | `Share` called, no error |
| `ShareTodo_unknownEmail` | email not in DB | `apperror.NotFound` |
| `ShareTodo_notOwner` | caller is not todo owner | `apperror.Forbidden` |
| `AddComment_ok` | valid todoID + callerID | returns `CommentOutput` |
| `DeleteComment_ownComment` | callerID == comment.UserID | deleted, no error |
| `DeleteComment_otherComment_asAdmin` | callerID != comment.UserID, isAdmin=true | deleted, no error |
| `DeleteComment_otherComment_notAdmin` | callerID != comment.UserID, isAdmin=false | `apperror.Forbidden` |

## Verification

```bash
go test ./internal/usecase/... -run TestTodoOwned -v
# All test cases pass

go test ./internal/usecase/...
# Expected: ok, no failures
```

## Done when
- [ ] `todo_owned_usecase_impl_test.go` covers all 16 test cases
- [ ] `go test ./internal/usecase/...` passes with no failures
