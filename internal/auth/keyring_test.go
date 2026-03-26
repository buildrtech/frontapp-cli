package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/99designs/keyring"
)

func TestSetTokenSetsHumanReadableLabel(t *testing.T) {
	fake := &fakeKeyring{}
	store := &KeyringStore{ring: fake}

	err := store.SetToken("default", "alice@example.com", Token{
		Email:        "alice@example.com",
		RefreshToken: "refresh-token",
		CreatedAt:    time.Unix(1700000000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("SetToken: %v", err)
	}

	if fake.lastSet.Key != "token:default:alice@example.com" {
		t.Fatalf("unexpected key: %#v", fake.lastSet)
	}

	wantLabel := "frontcli refresh token (alice@example.com)"
	if fake.lastSet.Label != wantLabel {
		t.Fatalf("expected label %q, got %q", wantLabel, fake.lastSet.Label)
	}
}

func TestGetAuthenticatedEmailUsesKeyMetadataOnly(t *testing.T) {
	fake := &fakeKeyring{
		keys: []string{
			"token:default:alice@example.com",
			"token:other:bob@example.com",
		},
		items: map[string]keyring.Item{
			"token:default:alice@example.com": storedTokenItem(t, "refresh-a"),
			"token:other:bob@example.com":     storedTokenItem(t, "refresh-b"),
		},
	}

	oldOpen := openKeyringFunc
	openKeyringFunc = func() (keyring.Keyring, error) { return fake, nil }

	t.Cleanup(func() { openKeyringFunc = oldOpen })

	ResetDefaultStore()
	t.Cleanup(ResetDefaultStore)

	email, err := GetAuthenticatedEmail("default")
	if err != nil {
		t.Fatalf("GetAuthenticatedEmail: %v", err)
	}

	if email != "alice@example.com" {
		t.Fatalf("expected alice@example.com, got %q", email)
	}

	if fake.getCalls != 0 {
		t.Fatalf("expected no secret reads while resolving email, got %d", fake.getCalls)
	}
}

type fakeKeyring struct {
	keys     []string
	items    map[string]keyring.Item
	lastSet  keyring.Item
	getCalls int
}

func (f *fakeKeyring) Get(key string) (keyring.Item, error) {
	f.getCalls++

	item, ok := f.items[key]
	if !ok {
		return keyring.Item{}, keyring.ErrKeyNotFound
	}

	return item, nil
}

func (f *fakeKeyring) GetMetadata(key string) (keyring.Metadata, error) {
	return keyring.Metadata{}, nil
}

func (f *fakeKeyring) Set(item keyring.Item) error {
	f.lastSet = item
	if f.items == nil {
		f.items = map[string]keyring.Item{}
	}
	f.items[item.Key] = item

	return nil
}

func (f *fakeKeyring) Remove(key string) error {
	delete(f.items, key)
	return nil
}

func (f *fakeKeyring) Keys() ([]string, error) {
	return f.keys, nil
}

func storedTokenItem(t *testing.T, refreshToken string) keyring.Item {
	t.Helper()

	data, err := json.Marshal(storedToken{
		RefreshToken: refreshToken,
		CreatedAt:    time.Unix(1700000000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("marshal stored token: %v", err)
	}

	return keyring.Item{
		Data: data,
	}
}
