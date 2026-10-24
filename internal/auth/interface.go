// SPDX-License-Identifier: GPL-3.0-only

// Package auth 负责微软账号登录、令牌刷新、Minecraft 玩家身份获取，
// 以及敏感令牌的安全存储。
// 本包的所有公开声明统一放在 interface.go 中。
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/Evernight275/cubicstart/internal/core"
)

const DefaultClientID = "809296ef-ee59-46ba-a7e0-1974d76f8e8e"

var ErrNotLoggedIn = errors.New("not logged in; run cubicstart login")

type DeviceCode struct {
	URL, Code string
	ExpiresAt time.Time
}

// Login authorizes a player and stores the refresh token in the system credential store.
func Login(ctx context.Context, clientID string, notify func(DeviceCode)) (core.Player, error) {
	return newAuthenticator(nil).Login(ctx, clientID, notify)
}

// RefreshSession exchanges the stored refresh token for a playable session.
func RefreshSession(ctx context.Context) (core.Session, error) {
	return newAuthenticator(nil).Session(ctx)
}

// Logout removes the stored account credentials.
func Logout() error { return newAuthenticator(nil).Logout() }
