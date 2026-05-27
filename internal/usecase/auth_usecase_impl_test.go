package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domainModel "github.com/yourname/go-clean-base/internal/domain/model"
	repoMock "github.com/yourname/go-clean-base/internal/domain/repository/mock"
	svcMock "github.com/yourname/go-clean-base/internal/domain/service/mock"
	"github.com/yourname/go-clean-base/internal/usecase"
	"github.com/yourname/go-clean-base/internal/usecase/dto"
)

func fixedUser(id int64, role string) *domainModel.User {
	return &domainModel.User{ID: id, Email: "test@example.com", Password: "$2a$12$placeholder", Role: role}
}

func newAuthUsecase(userRepo *repoMock.UserRepositoryMock, tokenSvc *svcMock.TokenServiceMock) usecase.IAuthUsecase {
	return usecase.NewAuthUsecase(userRepo, tokenSvc)
}

func defaultTokenSvc() *svcMock.TokenServiceMock {
	return &svcMock.TokenServiceMock{
		GenerateAccessTokenFn:  func(_ *domainModel.User) (string, error) { return "access-token", nil },
		GenerateRefreshTokenFn: func() (string, error) { return "refresh-token", nil },
		HashTokenFn:            func(t string) string { return "hashed-" + t },
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestRegister_ok(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				return nil, errors.New("not found")
			},
			CreateFn: func(_ context.Context, _ *domainModel.User) error { return nil },
		},
		defaultTokenSvc(),
	)
	if err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "Pass123!"}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRegister_emailTaken(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				return fixedUser(1, domainModel.RoleUser), nil
			},
		},
		defaultTokenSvc(),
	)
	err := uc.Register(context.Background(), dto.RegisterRequest{Email: "a@b.com", Password: "Pass123!"})
	if !errors.Is(err, domainModel.ErrEmailTaken) {
		t.Errorf("expected ErrEmailTaken, got %v", err)
	}
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_invalidCredentials_userNotFound(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				return nil, errors.New("not found")
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Login(context.Background(), dto.LoginRequest{Email: "x@x.com", Password: "wrong"})
	if !errors.Is(err, domainModel.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_invalidCredentials_wrongPassword(t *testing.T) {
	// bcrypt hash of "correct-password"
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByEmailFn: func(_ context.Context, _ string) (*domainModel.User, error) {
				// hash for "correct-password" pre-computed
				return &domainModel.User{ID: 1, Email: "a@b.com", Password: "$2a$12$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", Role: domainModel.RoleUser}, nil
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Login(context.Background(), dto.LoginRequest{Email: "a@b.com", Password: "wrong-password"})
	if !errors.Is(err, domainModel.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

// ── Refresh ───────────────────────────────────────────────────────────────────

func TestRefresh_ok(t *testing.T) {
	user := fixedUser(1, domainModel.RoleUser)
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindRefreshTokenFn: func(_ context.Context, _ string) (*domainModel.RefreshToken, error) {
				return &domainModel.RefreshToken{UserID: 1, Revoked: false, ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			RevokeRefreshTokenFn: func(_ context.Context, _ string) error { return nil },
			FindByIDFn:           func(_ context.Context, _ int64) (*domainModel.User, error) { return user, nil },
			SaveRefreshTokenFn:   func(_ context.Context, _ int64, _ string, _ time.Time) error { return nil },
		},
		defaultTokenSvc(),
	)
	resp, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "old-token"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.AccessToken == "" {
		t.Error("expected access token in response")
	}
}

func TestRefresh_tokenRevoked(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindRefreshTokenFn: func(_ context.Context, _ string) (*domainModel.RefreshToken, error) {
				return &domainModel.RefreshToken{UserID: 1, Revoked: true, ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "revoked"})
	if !errors.Is(err, domainModel.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestRefresh_tokenExpired(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindRefreshTokenFn: func(_ context.Context, _ string) (*domainModel.RefreshToken, error) {
				return &domainModel.RefreshToken{UserID: 1, Revoked: false, ExpiresAt: time.Now().Add(-time.Hour)}, nil
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "expired"})
	if !errors.Is(err, domainModel.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestRefresh_tokenNotFound(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindRefreshTokenFn: func(_ context.Context, _ string) (*domainModel.RefreshToken, error) {
				return nil, errors.New("not found")
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Refresh(context.Background(), dto.RefreshRequest{RefreshToken: "unknown"})
	if !errors.Is(err, domainModel.ErrTokenInvalid) {
		t.Errorf("expected ErrTokenInvalid, got %v", err)
	}
}

// ── Me ────────────────────────────────────────────────────────────────────────

func TestMe_ok(t *testing.T) {
	user := fixedUser(1, domainModel.RoleAdmin)
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByIDFn: func(_ context.Context, _ int64) (*domainModel.User, error) { return user, nil },
		},
		defaultTokenSvc(),
	)
	resp, err := uc.Me(context.Background(), 1)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.Role != domainModel.RoleAdmin {
		t.Errorf("expected role=admin, got %s", resp.Role)
	}
}

func TestMe_notFound(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			FindByIDFn: func(_ context.Context, _ int64) (*domainModel.User, error) {
				return nil, errors.New("not found")
			},
		},
		defaultTokenSvc(),
	)
	_, err := uc.Me(context.Background(), 99)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestLogout_alwaysSucceeds(t *testing.T) {
	uc := newAuthUsecase(
		&repoMock.UserRepositoryMock{
			RevokeRefreshTokenFn: func(_ context.Context, _ string) error {
				return errors.New("db error — should be swallowed")
			},
		},
		defaultTokenSvc(),
	)
	if err := uc.Logout(context.Background(), "any-token"); err != nil {
		t.Errorf("expected no error from Logout, got %v", err)
	}
}
