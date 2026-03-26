package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/dedene/frontapp-cli/internal/api"
)

func TestConvSearchEncodesQuery(t *testing.T) {
	var gotPath string
	var gotQuery string
	var gotLimit string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotQuery = r.URL.Query().Get("q")
		gotLimit = r.URL.Query().Get("limit")
		_, _ = io.WriteString(w, `{"_results":[]}`)
	}))
	defer srv.Close()

	old := newClientFromAuth
	newClientFromAuth = func(_, _ string) (*api.Client, error) {
		return api.NewClientWithBaseURL(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}), srv.URL), nil
	}
	t.Cleanup(func() { newClientFromAuth = old })

	cmd := ConvSearchCmd{Query: "from:me project update", Limit: 10}
	flags := &RootFlags{JSON: true, Account: "test@example.com"}

	if err := cmd.Run(flags); err != nil {
		t.Fatalf("Run: %v", err)
	}

	wantPath := "/conversations/search/" + url.PathEscape("from:me project update")
	if gotPath != wantPath {
		t.Fatalf("expected path %s, got %s", wantPath, gotPath)
	}

	if gotQuery != "" {
		t.Fatalf("expected no q query parameter, got %q", gotQuery)
	}

	if gotLimit != "10" {
		t.Fatalf("expected limit=10, got %q", gotLimit)
	}
}

func TestBuildConvSearchQuery_NormalizesDateOnly(t *testing.T) {
	before := time.Date(2026, 3, 3, 0, 0, 0, 0, time.Local).Unix()
	after := time.Date(2026, 2, 1, 0, 0, 0, 0, time.Local).Unix()

	q, err := buildConvSearchQuery(&ConvSearchCmd{
		Before: "2026-03-03",
		After:  "2026-02-01",
		Query:  "google ads",
	})
	if err != nil {
		t.Fatalf("buildConvSearchQuery: %v", err)
	}

	want := "before:" + strconv.FormatInt(before, 10) + " after:" + strconv.FormatInt(after, 10) + " google ads"
	if q != want {
		t.Fatalf("unexpected query:\nwant: %s\ngot:  %s", want, q)
	}
}

func TestBuildConvSearchQuery_NormalizesRFC3339(t *testing.T) {
	raw := "2026-02-01T15:04:05+01:00"
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	q, err := buildConvSearchQuery(&ConvSearchCmd{
		After: raw,
		Query: "google ads",
	})
	if err != nil {
		t.Fatalf("buildConvSearchQuery: %v", err)
	}

	want := "after:" + strconv.FormatInt(parsed.Unix(), 10) + " google ads"
	if q != want {
		t.Fatalf("unexpected query:\nwant: %s\ngot:  %s", want, q)
	}
}

func TestBuildConvSearchQuery_PassesUnixTimestamp(t *testing.T) {
	q, err := buildConvSearchQuery(&ConvSearchCmd{
		After: "1704067200",
		Query: "google ads",
	})
	if err != nil {
		t.Fatalf("buildConvSearchQuery: %v", err)
	}

	want := "after:1704067200 google ads"
	if q != want {
		t.Fatalf("unexpected query:\nwant: %s\ngot:  %s", want, q)
	}
}

func TestBuildConvSearchQuery_InvalidTimestamp(t *testing.T) {
	_, err := buildConvSearchQuery(&ConvSearchCmd{
		After: "2026/02/01",
		Query: "google ads",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "invalid after value") {
		t.Fatalf("expected invalid after value error, got: %v", err)
	}
}

func TestConvTagSendsTagIDs(t *testing.T) {
	var gotBody map[string][]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/conversations/cnv_123/tags" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := newClientFromAuth
	newClientFromAuth = func(_, _ string) (*api.Client, error) {
		return api.NewClientWithBaseURL(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}), srv.URL), nil
	}
	t.Cleanup(func() { newClientFromAuth = old })

	cmd := ConvTagCmd{ID: "cnv_123", TagID: "tag_abc"}
	flags := &RootFlags{Account: "test@example.com"}

	if err := cmd.Run(flags); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if gotBody == nil || len(gotBody["tag_ids"]) != 1 || gotBody["tag_ids"][0] != "tag_abc" {
		t.Fatalf("unexpected body: %#v", gotBody)
	}
}

func TestConvArchiveIDsFromStdin(t *testing.T) {
	var seen []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}

		seen = append(seen, strings.TrimPrefix(r.URL.Path, "/conversations/"))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	old := newClientFromAuth
	newClientFromAuth = func(_, _ string) (*api.Client, error) {
		return api.NewClientWithBaseURL(oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "token"}), srv.URL), nil
	}
	t.Cleanup(func() { newClientFromAuth = old })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()

	_, _ = w.WriteString("cnv_1\ncnv_2\n")
	_ = w.Close()

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	cmd := ConvArchiveCmd{IDsFrom: "-"}
	flags := &RootFlags{Account: "test@example.com"}

	if err := cmd.Run(flags); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(seen) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(seen))
	}
}
