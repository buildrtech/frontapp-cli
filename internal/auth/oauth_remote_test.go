package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"github.com/dedene/frontapp-cli/internal/config"
)

func TestBeginRemoteAuthorizationReturnsAuthURLAndState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.WriteClientCredentials("default", config.OAuthCredentials{
		ClientID:     "front-client-id",
		ClientSecret: "front-client-secret",
		RedirectURI:  "https://localhost:8484/callback",
	}); err != nil {
		t.Fatalf("WriteClientCredentials: %v", err)
	}

	oldRandomStateFn := randomStateFn
	randomStateFn = func() (string, error) {
		return "state-123", nil
	}
	t.Cleanup(func() {
		randomStateFn = oldRandomStateFn
	})

	result, err := BeginRemoteAuthorization(context.Background(), RemoteAuthorizeOptions{
		Client:       "default",
		Email:        "alice@example.com",
		ForceConsent: true,
	})
	if err != nil {
		t.Fatalf("BeginRemoteAuthorization: %v", err)
	}

	if result.AuthURL == "" {
		t.Fatal("expected auth URL")
	}

	if result.State != "state-123" {
		t.Fatalf("expected state-123, got %q", result.State)
	}

	if result.Email != "alice@example.com" {
		t.Fatalf("expected email to round-trip, got %q", result.Email)
	}

	if result.ClientName != "default" {
		t.Fatalf("expected default client, got %q", result.ClientName)
	}

	if want := "state=state-123"; !strings.Contains(result.AuthURL, want) {
		t.Fatalf("expected auth URL to contain %q, got %q", want, result.AuthURL)
	}

	if want := "prompt=consent"; !strings.Contains(result.AuthURL, want) {
		t.Fatalf("expected auth URL to contain %q, got %q", want, result.AuthURL)
	}
}

func TestCompleteRemoteAuthorizationExchangesCodeAndReturnsRefreshToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.WriteClientCredentials("default", config.OAuthCredentials{
		ClientID:     "front-client-id",
		ClientSecret: "front-client-secret",
		RedirectURI:  "https://localhost:8484/callback",
	}); err != nil {
		t.Fatalf("WriteClientCredentials: %v", err)
	}

	oldExchangeAuthCodeFn := exchangeAuthCodeFn
	exchangeAuthCodeFn = func(_ context.Context, cfg oauth2.Config, code string) (*oauth2.Token, error) {
		if cfg.ClientID != "front-client-id" {
			t.Fatalf("expected config to include credentials, got %q", cfg.ClientID)
		}

		if code != "code-123" {
			t.Fatalf("expected code-123, got %q", code)
		}

		return &oauth2.Token{RefreshToken: "refresh-123"}, nil
	}
	t.Cleanup(func() {
		exchangeAuthCodeFn = oldExchangeAuthCodeFn
	})

	refreshToken, err := CompleteRemoteAuthorization(context.Background(), RemoteCompleteOptions{
		Client:      "default",
		State:       "state-123",
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=state-123",
	})
	if err != nil {
		t.Fatalf("CompleteRemoteAuthorization: %v", err)
	}

	if refreshToken != "refresh-123" {
		t.Fatalf("expected refresh-123, got %q", refreshToken)
	}
}

func TestCompleteRemoteAuthorizationRejectsMissingOrMismatchedState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := config.WriteClientCredentials("default", config.OAuthCredentials{
		ClientID:     "front-client-id",
		ClientSecret: "front-client-secret",
		RedirectURI:  "https://localhost:8484/callback",
	}); err != nil {
		t.Fatalf("WriteClientCredentials: %v", err)
	}

	_, err := CompleteRemoteAuthorization(context.Background(), RemoteCompleteOptions{
		Client:      "default",
		State:       "",
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=state-123",
	})
	if !errors.Is(err, errStateMismatch) {
		t.Fatalf("expected errStateMismatch for missing state, got %v", err)
	}

	_, err = CompleteRemoteAuthorization(context.Background(), RemoteCompleteOptions{
		Client:      "default",
		State:       "state-123",
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=other-state",
	})
	if !errors.Is(err, errStateMismatch) {
		t.Fatalf("expected errStateMismatch for mismatched state, got %v", err)
	}
}
