package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/dedene/frontapp-cli/internal/config"
)

func TestTokenSourceCachesStoredRefreshTokenAcrossInvalidation(t *testing.T) {
	withTestClientCredentials(t)

	requests := newTokenRefreshRecorder([]tokenResponse{
		{AccessToken: "access-1", ExpiresIn: 3600},
		{AccessToken: "access-2", ExpiresIn: 3600},
	})
	withTokenEndpoint(t, requests.handler)

	store := &countingStore{
		token: Token{
			Client:       "default",
			Email:        "alice@example.com",
			RefreshToken: "refresh-a",
		},
	}
	ts := NewTokenSource("default", "alice@example.com", store)

	if _, err := ts.Token(); err != nil {
		t.Fatalf("first Token: %v", err)
	}

	ts.Invalidate()

	if _, err := ts.Token(); err != nil {
		t.Fatalf("second Token: %v", err)
	}

	if store.getTokenCalls != 1 {
		t.Fatalf("GetToken calls = %d, want 1", store.getTokenCalls)
	}

	if got := requests.refreshTokens(); !reflect.DeepEqual(got, []string{"refresh-a", "refresh-a"}) {
		t.Fatalf("refresh tokens = %#v, want refresh-a twice", got)
	}
}

func TestTokenSourceCachesRotatedRefreshToken(t *testing.T) {
	withTestClientCredentials(t)

	requests := newTokenRefreshRecorder([]tokenResponse{
		{AccessToken: "access-1", RefreshToken: "refresh-b", ExpiresIn: 3600},
		{AccessToken: "access-2", ExpiresIn: 3600},
	})
	withTokenEndpoint(t, requests.handler)

	store := &countingStore{
		token: Token{
			Client:       "default",
			Email:        "alice@example.com",
			RefreshToken: "refresh-a",
		},
	}
	ts := NewTokenSource("default", "alice@example.com", store)

	if _, err := ts.Token(); err != nil {
		t.Fatalf("first Token: %v", err)
	}

	ts.Invalidate()

	if _, err := ts.Token(); err != nil {
		t.Fatalf("second Token: %v", err)
	}

	if store.getTokenCalls != 1 {
		t.Fatalf("GetToken calls = %d, want 1", store.getTokenCalls)
	}

	if store.setTokenCalls != 1 {
		t.Fatalf("SetToken calls = %d, want 1", store.setTokenCalls)
	}

	if store.token.RefreshToken != "refresh-b" {
		t.Fatalf("stored refresh token = %q, want refresh-b", store.token.RefreshToken)
	}

	if got := requests.refreshTokens(); !reflect.DeepEqual(got, []string{"refresh-a", "refresh-b"}) {
		t.Fatalf("refresh tokens = %#v, want refresh-a then refresh-b", got)
	}
}

func TestTokenSourceUsesRotatedRefreshTokenWhenStoreUpdateFails(t *testing.T) {
	withTestClientCredentials(t)

	requests := newTokenRefreshRecorder([]tokenResponse{
		{AccessToken: "access-1", RefreshToken: "refresh-b", ExpiresIn: 3600},
		{AccessToken: "access-2", ExpiresIn: 3600},
	})
	withTokenEndpoint(t, requests.handler)

	store := &countingStore{
		token: Token{
			Client:       "default",
			Email:        "alice@example.com",
			RefreshToken: "refresh-a",
		},
		setTokenErr: errStoreUnavailable,
	}
	ts := NewTokenSource("default", "alice@example.com", store)

	if _, err := ts.Token(); err != nil {
		t.Fatalf("first Token: %v", err)
	}

	ts.Invalidate()

	if _, err := ts.Token(); err != nil {
		t.Fatalf("second Token: %v", err)
	}

	if store.getTokenCalls != 1 {
		t.Fatalf("GetToken calls = %d, want 1", store.getTokenCalls)
	}

	if store.setTokenCalls != 1 {
		t.Fatalf("SetToken calls = %d, want 1", store.setTokenCalls)
	}

	if store.token.RefreshToken != "refresh-a" {
		t.Fatalf("stored refresh token = %q, want unchanged refresh-a", store.token.RefreshToken)
	}

	if got := requests.refreshTokens(); !reflect.DeepEqual(got, []string{"refresh-a", "refresh-b"}) {
		t.Fatalf("refresh tokens = %#v, want refresh-a then refresh-b", got)
	}
}

func TestTokenSourceReturnsCachedAccessTokenWithoutStoreRead(t *testing.T) {
	withTestClientCredentials(t)

	requests := newTokenRefreshRecorder([]tokenResponse{
		{AccessToken: "access-1", ExpiresIn: 3600},
	})
	withTokenEndpoint(t, requests.handler)

	store := &countingStore{
		token: Token{
			Client:       "default",
			Email:        "alice@example.com",
			RefreshToken: "refresh-a",
		},
	}
	ts := NewTokenSource("default", "alice@example.com", store)

	first, err := ts.Token()
	if err != nil {
		t.Fatalf("first Token: %v", err)
	}

	second, err := ts.Token()
	if err != nil {
		t.Fatalf("second Token: %v", err)
	}

	if first.AccessToken != "access-1" || second.AccessToken != "access-1" {
		t.Fatalf("access tokens = %q, %q; want access-1", first.AccessToken, second.AccessToken)
	}

	if store.getTokenCalls != 1 {
		t.Fatalf("GetToken calls = %d, want 1", store.getTokenCalls)
	}

	if got := requests.refreshTokens(); !reflect.DeepEqual(got, []string{"refresh-a"}) {
		t.Fatalf("refresh tokens = %#v, want refresh-a once", got)
	}
}

var errStoreUnavailable = errors.New("keychain unavailable")

type countingStore struct {
	token         Token
	getTokenErr   error
	setTokenErr   error
	getTokenCalls int
	setTokenCalls int
}

func (s *countingStore) Keys() ([]string, error) {
	return nil, nil
}

func (s *countingStore) SetToken(client, email string, tok Token) error {
	s.setTokenCalls++
	if s.setTokenErr != nil {
		return s.setTokenErr
	}
	s.token = tok

	return nil
}

func (s *countingStore) GetToken(client, email string) (Token, error) {
	s.getTokenCalls++
	if s.getTokenErr != nil {
		return Token{}, s.getTokenErr
	}

	return s.token, nil
}

func (s *countingStore) DeleteToken(client, email string) error {
	return nil
}

func (s *countingStore) ListTokens() ([]Token, error) {
	return nil, nil
}

type tokenRefreshRecorder struct {
	mu       sync.Mutex
	tokens   []string
	sequence []tokenResponse
}

type tokenResponse struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

func newTokenRefreshRecorder(sequence []tokenResponse) *tokenRefreshRecorder {
	return &tokenRefreshRecorder{sequence: sequence}
}

func (r *tokenRefreshRecorder) handler(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tokens = append(r.tokens, req.Form.Get("refresh_token"))
	if len(r.tokens) > len(r.sequence) {
		http.Error(w, "unexpected token refresh", http.StatusInternalServerError)
		return
	}

	resp := r.sequence[len(r.tokens)-1]
	if resp.ExpiresIn == 0 {
		resp.ExpiresIn = 3600
	}

	payload := map[string]any{
		"access_token": resp.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   resp.ExpiresIn,
	}
	if resp.RefreshToken != "" {
		payload["refresh_token"] = resp.RefreshToken
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		panic(fmt.Sprintf("encode token response: %v", err))
	}
}

func (r *tokenRefreshRecorder) refreshTokens() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]string, len(r.tokens))
	copy(out, r.tokens)

	return out
}

func withTokenEndpoint(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldEndpoint := frontEndpoint
	frontEndpoint.TokenURL = server.URL

	t.Cleanup(func() { frontEndpoint = oldEndpoint })
}

func withTestClientCredentials(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	oldHome := os.Getenv("HOME")
	_ = os.Setenv("HOME", home)

	t.Cleanup(func() { _ = os.Setenv("HOME", oldHome) })

	if err := config.WriteClientCredentials("default", config.OAuthCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURI:  "https://localhost:8484/callback",
	}); err != nil {
		t.Fatalf("write client credentials: %v", err)
	}
}
