// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"

	"github.com/Evernight275/cubicstart/internal/auth"
	"github.com/Evernight275/cubicstart/internal/core"
)

func whoami(ctx context.Context) (core.Player, error) {
	session, err := auth.RefreshSession(ctx)
	return session.Player, err
}
