package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dedene/frontapp-cli/internal/auth"
)

func TestAuthLoginRemoteStep1WritesExpectedJSON(t *testing.T) {
	oldBeginRemoteAuthorizationFn := beginRemoteAuthorizationFn
	beginRemoteAuthorizationFn = func(_ context.Context, opts auth.RemoteAuthorizeOptions) (auth.RemoteAuthorization, error) {
		if opts.Email != "alice@example.com" {
			t.Fatalf("expected email to be forwarded, got %q", opts.Email)
		}

		if opts.Client != "default" {
			t.Fatalf("expected default client, got %q", opts.Client)
		}

		if !opts.ForceConsent {
			t.Fatal("expected force consent to be forwarded")
		}

		return auth.RemoteAuthorization{
			AuthURL:    "https://app.frontapp.com/oauth/authorize?state=state-123",
			State:      "state-123",
			Email:      "alice@example.com",
			ClientName: "default",
		}, nil
	}
	t.Cleanup(func() {
		beginRemoteAuthorizationFn = oldBeginRemoteAuthorizationFn
	})

	restoreStdout := captureFile(t, &os.Stdout)

	cmd := AuthLoginCmd{
		ClientName:   "default",
		Email:        "alice@example.com",
		ForceConsent: true,
		Remote:       true,
		Step:         1,
	}
	if err := cmd.Run(&RootFlags{JSON: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var output map[string]string
	if err := json.Unmarshal([]byte(restoreStdout()), &output); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}

	if output["auth_url"] != "https://app.frontapp.com/oauth/authorize?state=state-123" {
		t.Fatalf("unexpected auth_url: %#v", output)
	}

	if output["state"] != "state-123" || output["email"] != "alice@example.com" || output["client_name"] != "default" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestAuthLoginRemoteStep2StoresTokenAndWritesExpectedJSON(t *testing.T) {
	store := &fakeAuthStore{}

	oldCompleteRemoteAuthorizationFn := completeRemoteAuthorizationFn
	oldOpenAuthStoreFn := openAuthStoreFn
	completeRemoteAuthorizationFn = func(_ context.Context, opts auth.RemoteCompleteOptions) (string, error) {
		if opts.State != "state-123" {
			t.Fatalf("expected state-123, got %q", opts.State)
		}

		if opts.RedirectURL != "https://localhost:8484/callback?code=code-123&state=state-123" {
			t.Fatalf("unexpected redirect URL: %q", opts.RedirectURL)
		}

		return "refresh-123", nil
	}
	openAuthStoreFn = func() (auth.Store, error) {
		return store, nil
	}
	t.Cleanup(func() {
		completeRemoteAuthorizationFn = oldCompleteRemoteAuthorizationFn
		openAuthStoreFn = oldOpenAuthStoreFn
	})

	restoreStdout := captureFile(t, &os.Stdout)

	cmd := AuthLoginCmd{
		ClientName:  "default",
		Email:       "alice@example.com",
		Remote:      true,
		Step:        2,
		State:       "state-123",
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=state-123",
	}
	if err := cmd.Run(&RootFlags{JSON: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if store.setToken.Client != "default" || store.setToken.Email != "alice@example.com" {
		t.Fatalf("expected token to be stored for alice/default, got %#v", store.setToken)
	}

	if store.setToken.Token.RefreshToken != "refresh-123" {
		t.Fatalf("expected refresh token to be stored, got %#v", store.setToken.Token)
	}

	var output map[string]string
	if err := json.Unmarshal([]byte(restoreStdout()), &output); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}

	if output["status"] != "ok" || output["email"] != "alice@example.com" || output["client_name"] != "default" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

func TestAuthLoginRemoteStep2RejectsMissingOrMismatchedState(t *testing.T) {
	cmd := AuthLoginCmd{
		ClientName:  "default",
		Email:       "alice@example.com",
		Remote:      true,
		Step:        2,
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=state-123",
	}
	if err := cmd.Run(&RootFlags{JSON: true}); err == nil || !strings.Contains(err.Error(), "--state") {
		t.Fatalf("expected missing state error, got %v", err)
	}

	oldCompleteRemoteAuthorizationFn := completeRemoteAuthorizationFn
	completeRemoteAuthorizationFn = func(_ context.Context, _ auth.RemoteCompleteOptions) (string, error) {
		return "", errors.New("state mismatch")
	}
	t.Cleanup(func() {
		completeRemoteAuthorizationFn = oldCompleteRemoteAuthorizationFn
	})

	cmd.State = "state-123"
	if err := cmd.Run(&RootFlags{JSON: true}); err == nil || !strings.Contains(err.Error(), "state mismatch") {
		t.Fatalf("expected state mismatch error, got %v", err)
	}
}

func TestAuthLoginRemoteRejectsMissingStep(t *testing.T) {
	cmd := AuthLoginCmd{
		ClientName: "default",
		Email:      "alice@example.com",
		Remote:     true,
	}

	if err := cmd.Run(&RootFlags{JSON: true}); err == nil || !strings.Contains(err.Error(), "--step is required") {
		t.Fatalf("expected missing step error, got %v", err)
	}
}

func TestAuthLoginRejectsRemoteOnlyFlagsWithoutRemote(t *testing.T) {
	cmd := AuthLoginCmd{
		ClientName:  "default",
		Email:       "alice@example.com",
		Step:        2,
		State:       "state-123",
		RedirectURL: "https://localhost:8484/callback?code=code-123&state=state-123",
	}

	if err := cmd.Run(&RootFlags{JSON: true}); err == nil || !strings.Contains(err.Error(), "require --remote") {
		t.Fatalf("expected remote-only flags error, got %v", err)
	}
}

func TestAuthListWritesJSONAccounts(t *testing.T) {
	oldOpenAuthStoreFn := openAuthStoreFn
	openAuthStoreFn = func() (auth.Store, error) {
		return &fakeAuthStore{tokens: []auth.Token{{
			Client:    "default",
			Email:     "alice@example.com",
			CreatedAt: time.Unix(1700000000, 0).UTC(),
		}}}, nil
	}
	t.Cleanup(func() {
		openAuthStoreFn = oldOpenAuthStoreFn
	})

	restoreStdout := captureFile(t, &os.Stdout)

	if err := (&AuthListCmd{}).Run(&RootFlags{JSON: true}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var output struct {
		Accounts []auth.Token `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(restoreStdout()), &output); err != nil {
		t.Fatalf("unmarshal stdout: %v", err)
	}

	if len(output.Accounts) != 1 || output.Accounts[0].Email != "alice@example.com" {
		t.Fatalf("unexpected output: %#v", output)
	}
}

type fakeAuthStore struct {
	tokens   []auth.Token
	setToken fakeStoredToken
	deleted  []string
}

type fakeStoredToken struct {
	Client string
	Email  string
	Token  auth.Token
}

func (f *fakeAuthStore) Keys() ([]string, error) {
	return nil, nil
}

func (f *fakeAuthStore) SetToken(client, email string, tok auth.Token) error {
	f.setToken = fakeStoredToken{Client: client, Email: email, Token: tok}
	return nil
}

func (f *fakeAuthStore) GetToken(client, email string) (auth.Token, error) {
	for _, tok := range f.tokens {
		if tok.Client == client && tok.Email == email {
			return tok, nil
		}
	}
	return auth.Token{}, errors.New("not found")
}

func (f *fakeAuthStore) DeleteToken(_ string, email string) error {
	f.deleted = append(f.deleted, email)
	return nil
}

func (f *fakeAuthStore) ListTokens() ([]auth.Token, error) {
	return f.tokens, nil
}
