package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"github.com/dedene/frontapp-cli/internal/api"
)

func TestSignatureListPlain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/teammates/tea_123/signatures" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"_results":[
				{"id":"sig_123","name":"Support","is_default":true,"body":"Best regards"}
			]
		}`)
	}))
	defer srv.Close()

	old := newClientFromAuth
	newClientFromAuth = func(_, _ string) (*api.Client, error) {
		return api.NewClientWithBaseURL(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}), srv.URL), nil
	}
	t.Cleanup(func() { newClientFromAuth = old })

	restoreStdout := captureFile(t, &os.Stdout)

	cmd := SignatureListCmd{Teammate: "tea_123"}
	flags := &RootFlags{Plain: true, Account: "test@example.com"}

	if err := cmd.Run(flags); err != nil {
		t.Fatalf("Run: %v", err)
	}

	stdout := restoreStdout()

	if gotPath != "/teammates/tea_123/signatures" {
		t.Fatalf("expected teammate signatures path, got %s", gotPath)
	}

	if !strings.Contains(stdout, "sig_123") || !strings.Contains(stdout, "Support") || !strings.Contains(stdout, "true") {
		t.Fatalf("expected signature table output, got %q", stdout)
	}
}

func TestSignatureGetPlain(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/signatures/sig_123" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"sig_123",
			"name":"Support",
			"is_default":true,
			"body":"Best regards"
		}`)
	}))
	defer srv.Close()

	old := newClientFromAuth
	newClientFromAuth = func(_, _ string) (*api.Client, error) {
		return api.NewClientWithBaseURL(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}), srv.URL), nil
	}
	t.Cleanup(func() { newClientFromAuth = old })

	restoreStdout := captureFile(t, &os.Stdout)

	cmd := SignatureGetCmd{ID: "sig_123"}
	flags := &RootFlags{Plain: true, Account: "test@example.com"}

	if err := cmd.Run(flags); err != nil {
		t.Fatalf("Run: %v", err)
	}

	stdout := restoreStdout()

	if gotPath != "/signatures/sig_123" {
		t.Fatalf("expected signature path, got %s", gotPath)
	}

	if !strings.Contains(stdout, "ID:") || !strings.Contains(stdout, "sig_123") || !strings.Contains(stdout, "Default: true") || !strings.Contains(stdout, "Best regards") {
		t.Fatalf("expected signature detail output, got %q", stdout)
	}
}
