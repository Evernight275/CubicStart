// SPDX-License-Identifier: GPL-3.0-only

package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/Evernight275/cubicstart/internal/core"
)

const oauthScope = "XboxLive.SignIn XboxLive.offline_access"

type tokenStore interface {
	Get() (string, error)
	Set(string) error
	Delete() error
}
type vault struct{}

func (vault) Get() (string, error)   { return keyring.Get("CubicStart", "microsoft-account") }
func (vault) Set(value string) error { return keyring.Set("CubicStart", "microsoft-account", value) }
func (vault) Delete() error          { return keyring.Delete("CubicStart", "microsoft-account") }

type endpoints struct{ device, token, xbox, xsts, minecraft, entitlements, profile string }
type authenticator struct {
	http  *http.Client
	store tokenStore
	urls  endpoints
}
type accountRecord struct {
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}
type oauthToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
type apiError struct {
	status   int
	code     string
	xboxCode uint64
}

func (e *apiError) Error() string {
	if e.xboxCode != 0 {
		return fmt.Sprintf("Xbox authentication failed (HTTP %d, XErr %d); check Xbox profile and family permissions", e.status, e.xboxCode)
	}
	if e.code == "unauthorized_client" {
		return "Microsoft rejected this application (unauthorized_client); check Application (client) ID and enable personal Microsoft accounts"
	}
	if e.code == "invalid_client" {
		return "Microsoft rejected this public client (invalid_client); check the client ID and public client flow settings"
	}
	return fmt.Sprintf("authentication service returned HTTP %d (%s)", e.status, e.code)
}

func newAuthenticator(client *http.Client) *authenticator {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &authenticator{http: client, store: vault{}, urls: endpoints{
		device:       "https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode",
		token:        "https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		xbox:         "https://user.auth.xboxlive.com/user/authenticate",
		xsts:         "https://xsts.auth.xboxlive.com/xsts/authorize",
		minecraft:    "https://api.minecraftservices.com/launcher/login",
		entitlements: "https://api.minecraftservices.com/entitlements/license",
		profile:      "https://api.minecraftservices.com/minecraft/profile",
	}}
}

func (a *authenticator) request(ctx context.Context, method, endpoint, contentType, bearer string, body io.Reader, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "CubicStart/0.1.0")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if endpoint == a.urls.xbox || endpoint == a.urls.xsts {
		req.Header.Set("x-xbl-contract-version", "1")
	}
	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("contact authentication service: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return err
	}
	if len(data) > 4<<20 {
		return fmt.Errorf("authentication response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure struct {
			Error string `json:"error"`
			XErr  uint64 `json:"XErr"`
		}
		_ = json.Unmarshal(data, &failure)
		// Never include response bodies or OAuth error descriptions in logs.
		if !regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`).MatchString(failure.Error) {
			failure.Error = "request_failed"
		}
		return &apiError{resp.StatusCode, failure.Error, failure.XErr}
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid authentication response")
	}
	return nil
}

func (a *authenticator) form(ctx context.Context, endpoint string, values url.Values, target any) error {
	return a.request(ctx, http.MethodPost, endpoint, "application/x-www-form-urlencoded", "", strings.NewReader(values.Encode()), target)
}

func (a *authenticator) post(ctx context.Context, endpoint string, body any, target any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return a.request(ctx, http.MethodPost, endpoint, "application/json", "", bytes.NewReader(data), target)
}

func (a *authenticator) save(record accountRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := a.store.Set(string(data)); err != nil {
		return fmt.Errorf("save login in system credential store: %w", err)
	}
	return nil
}

func (a *authenticator) Login(ctx context.Context, clientID string, notify func(DeviceCode)) (core.Player, error) {
	if strings.TrimSpace(clientID) == "" {
		return core.Player{}, fmt.Errorf("Microsoft application client ID is required")
	}
	var device struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := a.form(ctx, a.urls.device, url.Values{"client_id": {clientID}, "scope": {oauthScope}}, &device); err != nil {
		return core.Player{}, err
	}
	if device.DeviceCode == "" || device.UserCode == "" || device.VerificationURI == "" || device.ExpiresIn <= 0 {
		return core.Player{}, fmt.Errorf("incomplete device authorization response")
	}
	deadline := time.Now().Add(time.Duration(device.ExpiresIn) * time.Second)
	if notify != nil {
		notify(DeviceCode{device.VerificationURI, device.UserCode, deadline})
	}
	pollCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	interval := time.Duration(device.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		timer := time.NewTimer(interval)
		select {
		case <-pollCtx.Done():
			timer.Stop()
			return core.Player{}, fmt.Errorf("device login ended: %w", pollCtx.Err())
		case <-timer.C:
		}
		var token oauthToken
		err := a.form(pollCtx, a.urls.token, url.Values{"client_id": {clientID}, "device_code": {device.DeviceCode}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, &token)
		var failure *apiError
		if errors.As(err, &failure) {
			switch failure.code {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			}
		}
		if err != nil {
			return core.Player{}, err
		}
		if token.AccessToken == "" || token.RefreshToken == "" {
			return core.Player{}, fmt.Errorf("Microsoft did not return access and refresh tokens")
		}
		session, err := a.exchange(pollCtx, token.AccessToken, clientID)
		if err != nil {
			return core.Player{}, err
		}
		if err := a.save(accountRecord{clientID, token.RefreshToken}); err != nil {
			return core.Player{}, err
		}
		return session.Player, nil
	}
}

func (a *authenticator) Session(ctx context.Context) (core.Session, error) {
	data, err := a.store.Get()
	if errors.Is(err, keyring.ErrNotFound) {
		return core.Session{}, ErrNotLoggedIn
	}
	if err != nil {
		return core.Session{}, fmt.Errorf("read system credential store: %w", err)
	}
	var record accountRecord
	if json.Unmarshal([]byte(data), &record) != nil || record.ClientID == "" || record.RefreshToken == "" {
		return core.Session{}, fmt.Errorf("stored login is invalid; run cubicstart login again")
	}
	var token oauthToken
	if err := a.form(ctx, a.urls.token, url.Values{"client_id": {record.ClientID}, "refresh_token": {record.RefreshToken}, "grant_type": {"refresh_token"}, "scope": {oauthScope}}, &token); err != nil {
		return core.Session{}, fmt.Errorf("refresh login; if authorization was revoked, run cubicstart login again: %w", err)
	}
	if token.AccessToken == "" {
		return core.Session{}, fmt.Errorf("Microsoft returned an empty access token")
	}
	if token.RefreshToken != "" {
		record.RefreshToken = token.RefreshToken
		if err := a.save(record); err != nil {
			return core.Session{}, err
		}
	}
	return a.exchange(ctx, token.AccessToken, record.ClientID)
}

func (a *authenticator) Logout() error {
	err := a.store.Delete()
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove stored login: %w", err)
	}
	return nil
}

type xboxResponse struct {
	Token         string `json:"Token"`
	DisplayClaims struct {
		Users []struct {
			Hash string `json:"uhs"`
			XUID string `json:"xid"`
		} `json:"xui"`
	} `json:"DisplayClaims"`
}

func (a *authenticator) exchange(ctx context.Context, msToken, clientID string) (core.Session, error) {
	var xbox xboxResponse
	if err := a.post(ctx, a.urls.xbox, map[string]any{
		"Properties":   map[string]string{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": "d=" + msToken},
		"RelyingParty": "http://auth.xboxlive.com", "TokenType": "JWT",
	}, &xbox); err != nil {
		return core.Session{}, err
	}
	if xbox.Token == "" {
		return core.Session{}, fmt.Errorf("Xbox returned an empty user token")
	}
	var xsts xboxResponse
	if err := a.post(ctx, a.urls.xsts, map[string]any{
		"Properties":   map[string]any{"SandboxId": "RETAIL", "UserTokens": []string{xbox.Token}},
		"RelyingParty": "rp://api.minecraftservices.com/", "TokenType": "JWT",
	}, &xsts); err != nil {
		return core.Session{}, err
	}
	if xsts.Token == "" || len(xsts.DisplayClaims.Users) == 0 || xsts.DisplayClaims.Users[0].Hash == "" {
		return core.Session{}, fmt.Errorf("Xbox returned incomplete authorization")
	}
	var minecraft struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	user := xsts.DisplayClaims.Users[0]
	if err := a.post(ctx, a.urls.minecraft, map[string]string{"xtoken": "XBL3.0 x=" + user.Hash + ";" + xsts.Token, "platform": "PC_LAUNCHER"}, &minecraft); err != nil {
		return core.Session{}, err
	}
	if minecraft.AccessToken == "" || minecraft.ExpiresIn <= 0 {
		return core.Session{}, fmt.Errorf("Minecraft returned an invalid session")
	}
	var entitlements struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	var requestID [16]byte
	if _, err := rand.Read(requestID[:]); err != nil {
		return core.Session{}, err
	}
	requestID[6], requestID[8] = (requestID[6]&15)|64, (requestID[8]&63)|128
	entitlementsURL := fmt.Sprintf("%s?requestId=%x-%x-%x-%x-%x", a.urls.entitlements, requestID[:4], requestID[4:6], requestID[6:8], requestID[8:10], requestID[10:])
	if err := a.request(ctx, http.MethodGet, entitlementsURL, "", minecraft.AccessToken, nil, &entitlements); err != nil {
		return core.Session{}, err
	}
	canPlay := false
	for _, item := range entitlements.Items {
		canPlay = canPlay || item.Name == "game_minecraft" || item.Name == "product_minecraft"
	}
	if !canPlay {
		return core.Session{}, fmt.Errorf("this account has no Minecraft Java entitlement")
	}
	var profile core.Player
	if err := a.request(ctx, http.MethodGet, a.urls.profile, "", minecraft.AccessToken, nil, &profile); err != nil {
		return core.Session{}, fmt.Errorf("load Minecraft profile (create a Java profile if needed): %w", err)
	}
	if !regexp.MustCompile(`^[0-9a-fA-F]{32}$`).MatchString(profile.ID) || profile.Name == "" {
		return core.Session{}, fmt.Errorf("Minecraft returned an invalid player profile")
	}
	return core.Session{Player: profile, AccessToken: minecraft.AccessToken, XUID: user.XUID, ClientID: clientID, UserType: "msa", ExpiresAt: time.Now().Add(time.Duration(minecraft.ExpiresIn) * time.Second)}, nil
}
