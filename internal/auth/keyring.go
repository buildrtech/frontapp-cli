package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/99designs/keyring"
	"golang.org/x/term"

	"github.com/dedene/frontapp-cli/internal/config"
)

type Store interface {
	Keys() ([]string, error)
	SetToken(client, email string, tok Token) error
	GetToken(client, email string) (Token, error)
	DeleteToken(client, email string) error
	ListTokens() ([]Token, error)
}

type KeyringStore struct {
	ring keyring.Keyring
}

type Token struct {
	Client       string    `json:"client,omitempty"`
	Email        string    `json:"email"`
	Scopes       []string  `json:"scopes,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	RefreshToken string    `json:"-"`
}

const (
	keyringPasswordEnv = "FRONT_KEYRING_PASSWORD" //nolint:gosec // env var name
	keyringBackendEnv  = "FRONT_KEYRING_BACKEND"  //nolint:gosec // env var name
)

var (
	errMissingEmail        = errors.New("missing email")
	errMissingRefreshToken = errors.New("missing refresh token")
	errNoTTY               = errors.New("no TTY available for keyring password prompt")
	errInvalidBackend      = errors.New("invalid keyring backend")
	errKeyringTimeout      = errors.New("keyring connection timed out")
	openKeyringFunc        = openKeyring
	keyringOpenFunc        = keyring.Open
)

// Singleton store to avoid multiple keychain prompts per process.
var (
	defaultStore     Store
	defaultStoreOnce sync.Once
	defaultStoreErr  error
)

const keyringOpenTimeout = 5 * time.Second

func openKeyring() (keyring.Keyring, error) {
	keyringDir, err := config.EnsureKeyringDir()
	if err != nil {
		return nil, fmt.Errorf("ensure keyring dir: %w", err)
	}

	backend := normalizeBackend(os.Getenv(keyringBackendEnv))

	backends, err := allowedBackends(backend)
	if err != nil {
		return nil, err
	}

	dbusAddr := os.Getenv("DBUS_SESSION_BUS_ADDRESS")
	if shouldForceFileBackend(runtime.GOOS, backend, dbusAddr) {
		backends = []keyring.BackendType{keyring.FileBackend}
	}

	cfg := keyring.Config{
		ServiceName:              config.AppName,
		KeychainTrustApplication: true,
		AllowedBackends:          backends,
		FileDir:                  keyringDir,
		FilePasswordFunc:         fileKeyringPasswordFunc(),
	}

	if shouldUseTimeout(runtime.GOOS, backend, dbusAddr) {
		return openKeyringWithTimeout(cfg, keyringOpenTimeout)
	}

	ring, err := keyringOpenFunc(cfg)
	if err != nil {
		return nil, fmt.Errorf("open keyring: %w", err)
	}

	return ring, nil
}

func normalizeBackend(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func allowedBackends(backend string) ([]keyring.BackendType, error) {
	switch backend {
	case "", "auto":
		return nil, nil
	case "keychain":
		return []keyring.BackendType{keyring.KeychainBackend}, nil
	case "file":
		return []keyring.BackendType{keyring.FileBackend}, nil
	default:
		return nil, fmt.Errorf("%w: %q", errInvalidBackend, backend)
	}
}

func shouldForceFileBackend(goos, backend, dbusAddr string) bool {
	return goos == "linux" && (backend == "" || backend == "auto") && dbusAddr == ""
}

func shouldUseTimeout(goos, backend, dbusAddr string) bool {
	return goos == "linux" && (backend == "" || backend == "auto") && dbusAddr != ""
}

func fileKeyringPasswordFunc() keyring.PromptFunc {
	password := os.Getenv(keyringPasswordEnv)
	if password != "" {
		return keyring.FixedStringPrompt(password)
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		return keyring.TerminalPrompt
	}

	return func(_ string) (string, error) {
		return "", fmt.Errorf("%w; set %s", errNoTTY, keyringPasswordEnv)
	}
}

type keyringResult struct {
	ring keyring.Keyring
	err  error
}

func openKeyringWithTimeout(cfg keyring.Config, timeout time.Duration) (keyring.Keyring, error) {
	ch := make(chan keyringResult, 1)

	go func() {
		ring, err := keyringOpenFunc(cfg)
		ch <- keyringResult{ring, err}
	}()

	select {
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("open keyring: %w", res.err)
		}

		return res.ring, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("%w after %v; set %s=file and %s=<password>",
			errKeyringTimeout, timeout, keyringBackendEnv, keyringPasswordEnv)
	}
}

func OpenDefault() (Store, error) {
	defaultStoreOnce.Do(func() {
		ring, err := openKeyringFunc()
		if err != nil {
			defaultStoreErr = err
			return
		}
		defaultStore = &KeyringStore{ring: ring}
	})

	return defaultStore, defaultStoreErr
}

// ResetDefaultStore resets the singleton store for testing.
func ResetDefaultStore() {
	defaultStoreOnce = sync.Once{}
	defaultStore = nil
	defaultStoreErr = nil
}

type storedToken struct {
	RefreshToken string    `json:"refresh_token"`
	Scopes       []string  `json:"scopes,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
}

func (s *KeyringStore) Keys() ([]string, error) {
	keys, err := s.ring.Keys()
	if err != nil {
		return nil, fmt.Errorf("list keyring keys: %w", err)
	}

	return keys, nil
}

func (s *KeyringStore) SetToken(client, email string, tok Token) error {
	email = normalize(email)
	if email == "" {
		return errMissingEmail
	}

	if tok.RefreshToken == "" {
		return errMissingRefreshToken
	}

	normalizedClient, err := config.NormalizeClientNameOrDefault(client)
	if err != nil {
		return fmt.Errorf("normalize client: %w", err)
	}

	if tok.CreatedAt.IsZero() {
		tok.CreatedAt = time.Now().UTC()
	}

	payload, err := json.Marshal(storedToken{
		RefreshToken: tok.RefreshToken,
		Scopes:       tok.Scopes,
		CreatedAt:    tok.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}

	if err := s.ring.Set(keyring.Item{
		Key:   tokenKey(normalizedClient, email),
		Data:  payload,
		Label: tokenLabel(email),
	}); err != nil {
		return wrapKeychainError(fmt.Errorf("store token: %w", err))
	}

	return nil
}

func (s *KeyringStore) GetToken(client, email string) (Token, error) {
	email = normalize(email)
	if email == "" {
		return Token{}, errMissingEmail
	}

	normalizedClient, err := config.NormalizeClientNameOrDefault(client)
	if err != nil {
		return Token{}, fmt.Errorf("normalize client: %w", err)
	}

	item, err := s.ring.Get(tokenKey(normalizedClient, email))
	if err != nil {
		return Token{}, fmt.Errorf("read token: %w", err)
	}

	var st storedToken
	if err := json.Unmarshal(item.Data, &st); err != nil {
		return Token{}, fmt.Errorf("decode token: %w", err)
	}

	return Token{
		Client:       normalizedClient,
		Email:        email,
		Scopes:       st.Scopes,
		CreatedAt:    st.CreatedAt,
		RefreshToken: st.RefreshToken,
	}, nil
}

func (s *KeyringStore) DeleteToken(client, email string) error {
	email = normalize(email)
	if email == "" {
		return errMissingEmail
	}

	normalizedClient, err := config.NormalizeClientNameOrDefault(client)
	if err != nil {
		return fmt.Errorf("normalize client: %w", err)
	}

	if err := s.ring.Remove(tokenKey(normalizedClient, email)); err != nil && !errors.Is(err, keyring.ErrKeyNotFound) {
		return fmt.Errorf("delete token: %w", err)
	}

	return nil
}

func (s *KeyringStore) ListTokens() ([]Token, error) {
	keys, err := s.Keys()
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}

	out := make([]Token, 0)
	seen := make(map[string]struct{})

	for _, k := range keys {
		client, email, ok := ParseTokenKey(k)
		if !ok {
			continue
		}

		key := client + "\n" + email
		if _, exists := seen[key]; exists {
			continue
		}

		tok, err := s.GetToken(client, email)
		if err != nil {
			return nil, fmt.Errorf("read token for %s: %w", email, err)
		}

		seen[key] = struct{}{}

		out = append(out, tok)
	}

	return out, nil
}

func ParseTokenKey(k string) (client, email string, ok bool) {
	const prefix = "token:"
	if !strings.HasPrefix(k, prefix) {
		return "", "", false
	}

	rest := strings.TrimPrefix(k, prefix)
	if strings.TrimSpace(rest) == "" {
		return "", "", false
	}

	if !strings.Contains(rest, ":") {
		return config.DefaultClientName, rest, true
	}

	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}

	return parts[0], parts[1], true
}

func tokenKey(client, email string) string {
	return fmt.Sprintf("token:%s:%s", client, email)
}

func tokenLabel(email string) string {
	return fmt.Sprintf("%s refresh token (%s)", config.AppName, email)
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func wrapKeychainError(err error) error {
	if err == nil {
		return nil
	}

	if IsKeychainLockedError(err.Error()) {
		return fmt.Errorf("%w\n\nYour macOS keychain is locked. Run:\n  security unlock-keychain ~/Library/Keychains/login.keychain-db", err)
	}

	return err
}

func IsKeychainLockedError(msg string) bool {
	return strings.Contains(msg, "keychain is locked") ||
		strings.Contains(msg, "The user name or passphrase you entered is not correct")
}
