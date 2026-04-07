package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/dedene/frontapp-cli/internal/config"
)

// Front OAuth2 endpoints
var frontEndpoint = oauth2.Endpoint{
	AuthURL:  "https://app.frontapp.com/oauth/authorize",
	TokenURL: "https://app.frontapp.com/oauth/token",
}

const defaultCallbackPort = 8484

type AuthorizeOptions struct {
	Manual       bool
	ForceConsent bool
	Timeout      time.Duration
	Client       string
}

type RemoteAuthorizeOptions struct {
	Email        string
	Client       string
	ForceConsent bool
}

type RemoteAuthorization struct {
	AuthURL    string `json:"auth_url"`
	State      string `json:"state"`
	Email      string `json:"email"`
	ClientName string `json:"client_name"`
}

type RemoteCompleteOptions struct {
	Client      string
	State       string
	RedirectURL string
}

var (
	errAuthorization       = errors.New("authorization error")
	errHTTPSRequired       = errors.New("redirect uri must use https")
	errMissingCode         = errors.New("missing code")
	errNoCodeInURL         = errors.New("no code found in URL")
	errNoRefreshToken      = errors.New("no refresh token received; try with --force-consent")
	errStateMismatch       = errors.New("state mismatch")
	errUnsupportedPlatform = errors.New("unsupported platform")
	openBrowserFn          = openBrowser
	randomStateFn          = randomState
	exchangeAuthCodeFn     = exchangeAuthCode
)

func Authorize(ctx context.Context, opts AuthorizeOptions) (string, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 2 * time.Minute
	}

	cfg, err := oauthConfigForClient(opts.Client)
	if err != nil {
		return "", err
	}

	state, err := randomStateFn()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	authOpts := buildAuthCodeOptions(opts.ForceConsent)

	if opts.Manual {
		return authorizeManual(ctx, cfg, state, authOpts)
	}

	return authorizeWithServer(ctx, cfg, state, authOpts)
}

func BeginRemoteAuthorization(_ context.Context, opts RemoteAuthorizeOptions) (RemoteAuthorization, error) {
	cfg, err := oauthConfigForClient(opts.Client)
	if err != nil {
		return RemoteAuthorization{}, err
	}

	state, err := randomStateFn()
	if err != nil {
		return RemoteAuthorization{}, err
	}

	clientName, err := config.NormalizeClientNameOrDefault(opts.Client)
	if err != nil {
		return RemoteAuthorization{}, err
	}

	return RemoteAuthorization{
		AuthURL:    cfg.AuthCodeURL(state, buildAuthCodeOptions(opts.ForceConsent)...),
		State:      state,
		Email:      opts.Email,
		ClientName: clientName,
	}, nil
}

func CompleteRemoteAuthorization(ctx context.Context, opts RemoteCompleteOptions) (string, error) {
	cfg, err := oauthConfigForClient(opts.Client)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(opts.State) == "" {
		return "", errStateMismatch
	}

	code, gotState, err := extractCodeAndState(opts.RedirectURL)
	if err != nil {
		return "", err
	}

	if gotState != opts.State {
		return "", errStateMismatch
	}

	tok, err := exchangeAuthCodeFn(ctx, cfg, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}

	if tok.RefreshToken == "" {
		return "", errNoRefreshToken
	}

	return tok.RefreshToken, nil
}

func oauthConfigForClient(client string) (oauth2.Config, error) {
	creds, err := config.ReadClientCredentials(client)
	if err != nil {
		return oauth2.Config{}, fmt.Errorf("read credentials: %w", err)
	}

	redirectURI := creds.RedirectURI
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("https://localhost:%d/callback", defaultCallbackPort)
	}

	cfg := oauth2.Config{
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     frontEndpoint,
		RedirectURL:  redirectURI,
	}

	parsed, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return oauth2.Config{}, fmt.Errorf("parse redirect uri: %w", err)
	}

	if !strings.EqualFold(parsed.Scheme, "https") {
		return oauth2.Config{}, errHTTPSRequired
	}

	return cfg, nil
}

func buildAuthCodeOptions(forceConsent bool) []oauth2.AuthCodeOption {
	authOpts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if forceConsent {
		authOpts = append(authOpts, oauth2.SetAuthURLParam("prompt", "consent"))
	}

	return authOpts
}

func exchangeAuthCode(ctx context.Context, cfg oauth2.Config, code string) (*oauth2.Token, error) {
	return cfg.Exchange(ctx, code)
}

func authorizeManual(ctx context.Context, cfg oauth2.Config, state string, authOpts []oauth2.AuthCodeOption) (string, error) {
	authURL := cfg.AuthCodeURL(state, authOpts...)

	fmt.Fprintln(os.Stderr, "Visit this URL to authorize:")
	fmt.Fprintln(os.Stderr, authURL)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "After authorizing, you'll be redirected to a URL.")
	fmt.Fprintln(os.Stderr, "Copy the URL from your browser and paste it here.")
	fmt.Fprintln(os.Stderr)
	fmt.Fprint(os.Stderr, "Paste redirect URL: ")

	var line string
	if _, err := fmt.Scanln(&line); err != nil {
		return "", fmt.Errorf("read redirect url: %w", err)
	}

	line = strings.TrimSpace(line)

	code, gotState, err := extractCodeAndState(line)
	if err != nil {
		return "", err
	}

	if gotState != "" && gotState != state {
		return "", errStateMismatch
	}

	tok, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", fmt.Errorf("exchange code: %w", err)
	}

	if tok.RefreshToken == "" {
		return "", errNoRefreshToken
	}

	return tok.RefreshToken, nil
}

func authorizeWithServer(ctx context.Context, cfg oauth2.Config, state string, authOpts []oauth2.AuthCodeOption) (string, error) {
	// Parse port from redirect URI
	parsed, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return "", fmt.Errorf("parse redirect uri: %w", err)
	}

	port := parsed.Port()
	if port == "" {
		port = "8484"
	}

	// Get TLS certificate for HTTPS
	certPath, keyPath, err := EnsureCertificate()
	if err != nil {
		return "", fmt.Errorf("setup TLS: %w", err)
	}

	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:"+port)
	if err != nil {
		return "", fmt.Errorf("listen for callback: %w", err)
	}

	defer func() { _ = ln.Close() }()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	srv := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0), //nolint:forbidigo // Suppress TLS handshake errors from self-signed cert
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/callback" {
				http.NotFound(w, r)

				return
			}

			q := r.URL.Query()

			w.Header().Set("Content-Type", "text/html; charset=utf-8")

			if q.Get("error") != "" {
				select {
				case errCh <- fmt.Errorf("%w: %s", errAuthorization, q.Get("error")):
				default:
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(cancelledHTML))

				return
			}

			if q.Get("state") != state {
				select {
				case errCh <- errStateMismatch:
				default:
				}

				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(errorHTML("State mismatch - please try again.")))

				return
			}

			code := q.Get("code")
			if code == "" {
				select {
				case errCh <- errMissingCode:
				default:
				}

				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(errorHTML("Missing authorization code.")))

				return
			}

			select {
			case codeCh <- code:
			default:
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(successHTML))
		}),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Close()
	}()

	go func() {
		if err := srv.ServeTLS(ln, certPath, keyPath); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	authURL := cfg.AuthCodeURL(state, authOpts...)

	fmt.Fprintln(os.Stderr, "Opening browser for authorization...")
	fmt.Fprintln(os.Stderr, "If the browser doesn't open, visit:")
	fmt.Fprintln(os.Stderr, authURL)
	_ = openBrowserFn(authURL)

	select {
	case code := <-codeCh:
		fmt.Fprintln(os.Stderr, "Authorization received. Finishing...")

		tok, err := cfg.Exchange(ctx, code)
		if err != nil {
			_ = srv.Close()

			return "", fmt.Errorf("exchange code: %w", err)
		}

		if tok.RefreshToken == "" {
			_ = srv.Close()

			return "", errNoRefreshToken
		}

		shutdownCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)

		return tok.RefreshToken, nil

	case err := <-errCh:
		_ = srv.Close()

		return "", err

	case <-ctx.Done():
		_ = srv.Close()

		return "", fmt.Errorf("authorization canceled: %w", ctx.Err())
	}
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func extractCodeAndState(rawURL string) (code, state string, err error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("parse redirect url: %w", err)
	}

	code = parsed.Query().Get("code")
	if code == "" {
		return "", "", errNoCodeInURL
	}

	return code, parsed.Query().Get("state"), nil
}

func openBrowser(targetURL string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL) //nolint:noctx // fire-and-forget browser open
	case "linux":
		cmd = exec.Command("xdg-open", targetURL) //nolint:noctx // fire-and-forget browser open
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", targetURL) //nolint:noctx // fire-and-forget browser open
	default:
		return fmt.Errorf("%w: %s", errUnsupportedPlatform, runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start browser: %w", err)
	}

	return nil
}

const successHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Authorization Successful</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
           display: flex; justify-content: center; align-items: center;
           min-height: 100vh; margin: 0; background: #f5f5f5; }
    .container { text-align: center; padding: 40px; background: white;
                 border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
    h1 { color: #22c55e; margin-bottom: 16px; }
    p { color: #666; }
  </style>
</head>
<body>
  <div class="container">
    <h1>&#10004; Authorization Successful</h1>
    <p>You can close this window and return to the terminal.</p>
  </div>
</body>
</html>`

const cancelledHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Authorization Cancelled</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
           display: flex; justify-content: center; align-items: center;
           min-height: 100vh; margin: 0; background: #f5f5f5; }
    .container { text-align: center; padding: 40px; background: white;
                 border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
    h1 { color: #f59e0b; margin-bottom: 16px; }
    p { color: #666; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Authorization Cancelled</h1>
    <p>You can close this window.</p>
  </div>
</body>
</html>`

func errorHTML(msg string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <title>Authorization Error</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
           display: flex; justify-content: center; align-items: center;
           min-height: 100vh; margin: 0; background: #f5f5f5; }
    .container { text-align: center; padding: 40px; background: white;
                 border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
    h1 { color: #ef4444; margin-bottom: 16px; }
    p { color: #666; }
  </style>
</head>
<body>
  <div class="container">
    <h1>Authorization Error</h1>
    <p>%s</p>
  </div>
</body>
</html>`, msg)
}
