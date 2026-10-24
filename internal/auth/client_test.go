// SPDX-License-Identifier: GPL-3.0-only

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/zalando/go-keyring"
)

type memoryStore struct{ value string }

func (m *memoryStore) Get() (string, error) {
	if m.value == "" {
		return "", keyring.ErrNotFound
	}
	return m.value, nil
}
func (m *memoryStore) Set(value string) error { m.value = value; return nil }
func (m *memoryStore) Delete() error          { m.value = ""; return nil }

func authFixture(t *testing.T, noEntitlement bool) (*authenticator, *memoryStore) {
	t.Helper()
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("client_id") != "test-client" {
				t.Error("wrong client ID")
			}
			io.WriteString(w, `{"device_code":"device-secret","user_code":"TEST-CODE","verification_uri":"https://www.microsoft.com/link","expires_in":30,"interval":1}`)
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("grant_type") == "refresh_token" && r.Form.Get("refresh_token") != "old-refresh" {
				t.Error("wrong stored refresh token")
			}
			if r.Form.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" && polls.Add(1) == 1 {
				w.WriteHeader(400)
				io.WriteString(w, `{"error":"authorization_pending"}`)
				return
			}
			io.WriteString(w, `{"access_token":"ms-secret","refresh_token":"new-refresh"}`)
		case "/xbox":
			if r.Header.Get("x-xbl-contract-version") != "1" {
				t.Error("missing Xbox contract header")
			}
			io.WriteString(w, `{"Token":"xbox-secret"}`)
		case "/xsts":
			io.WriteString(w, `{"Token":"xsts-secret","DisplayClaims":{"xui":[{"uhs":"hash","xid":"xuid"}]}}`)
		case "/minecraft":
			io.WriteString(w, `{"access_token":"minecraft-secret","expires_in":3600}`)
		case "/entitlements":
			if r.Header.Get("Authorization") != "Bearer minecraft-secret" {
				t.Error("missing Minecraft bearer token")
			}
			if noEntitlement {
				io.WriteString(w, `{"items":[]}`)
			} else {
				io.WriteString(w, `{"items":[{"name":"game_minecraft"}]}`)
			}
		case "/profile":
			io.WriteString(w, `{"id":"0123456789abcdef0123456789abcdef","name":"TestPlayer"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	store := &memoryStore{}
	a := newAuthenticator(server.Client())
	a.store = store
	a.urls = endpoints{server.URL + "/device", server.URL + "/token", server.URL + "/xbox", server.URL + "/xsts", server.URL + "/minecraft", server.URL + "/entitlements", server.URL + "/profile"}
	return a, store
}

func TestDeviceLoginStoresOnlyRefreshToken(t *testing.T) {
	a, store := authFixture(t, false)
	var code DeviceCode
	profile, err := a.Login(context.Background(), "test-client", func(value DeviceCode) { code = value })
	if err != nil {
		t.Fatal(err)
	}
	if code.Code != "TEST-CODE" || profile.Name != "TestPlayer" {
		t.Fatalf("login result: %+v, %+v", code, profile)
	}
	if !strings.Contains(store.value, "new-refresh") {
		t.Fatal("refresh token not stored")
	}
	for _, secret := range []string{"ms-secret", "xbox-secret", "xsts-secret", "minecraft-secret", "device-secret"} {
		if strings.Contains(store.value, secret) {
			t.Errorf("persisted unnecessary token %s", secret)
		}
	}
}

func TestRefreshRotationAndEntitlementFailure(t *testing.T) {
	for _, noEntitlement := range []bool{false, true} {
		a, store := authFixture(t, noEntitlement)
		data, _ := json.Marshal(accountRecord{"test-client", "old-refresh"})
		store.value = string(data)
		session, err := a.Session(context.Background())
		if noEntitlement {
			if err == nil || !strings.Contains(err.Error(), "entitlement") {
				t.Fatalf("missing entitlement: %v", err)
			}
		} else if err != nil || session.Player.Name != "TestPlayer" || session.AccessToken != "minecraft-secret" || session.UserType != "msa" {
			t.Fatalf("session: %+v, %v", session, err)
		}
		if !strings.Contains(store.value, "new-refresh") {
			t.Fatal("rotated refresh token lost on a later-stage failure")
		}
		if err := a.Logout(); err != nil {
			t.Fatal(err)
		}
		if _, err := a.Session(context.Background()); !errors.Is(err, ErrNotLoggedIn) {
			t.Fatalf("logout did not clear session: %v", err)
		}
	}
}

func TestAuthErrorsNeverExposeResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		io.WriteString(w, `{"error":"invalid_grant","error_description":"SECRET-MUST-NOT-LEAK","access_token":"ALSO-SECRET"}`)
	}))
	defer server.Close()
	a := newAuthenticator(server.Client())
	a.urls.device = server.URL
	_, err := a.Login(context.Background(), "test-client", nil)
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestDeviceLoginHonorsCancellation(t *testing.T) {
	a, _ := authFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	_, err := a.Login(ctx, "test-client", func(DeviceCode) { cancel() })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled login: %v", err)
	}
}
