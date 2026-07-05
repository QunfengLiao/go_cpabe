package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-cpabe/backend/internal/domain"
	"go-cpabe/backend/internal/pkg/auth"
	"go-cpabe/backend/internal/pkg/response"
)

func TestLoginRules(t *testing.T) {
	repo := newMemoryUserRepo()
	manager := auth.NewManager("secret", time.Minute)
	store := auth.NewMemoryTokenStore()
	svc := NewAuthService(repo, manager, store, time.Hour)
	ctx := context.Background()

	_, err := svc.Register(ctx, RegisterInput{
		Email: "user@example.com", Password: "Passw0rd!", ConfirmPassword: "Passw0rd!", Nickname: "用户", Role: domain.RoleDataUser,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	pair, user, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "Passw0rd!"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" || pair.TokenType != "Bearer" {
		t.Fatalf("unexpected token pair: %+v", pair)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	if _, err := manager.ParseAccessToken(pair.AccessToken); err != nil {
		t.Fatalf("parse access token: %v", err)
	}

	if _, _, err := svc.Login(ctx, LoginInput{Email: "user@example.com", Password: "wrong"}); !errors.Is(err, response.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
