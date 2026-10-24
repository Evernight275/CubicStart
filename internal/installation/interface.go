// SPDX-License-Identifier: GPL-3.0-only

// Package installation reads local version records and resolves and verifies
// their required files. It does not store saves or start game processes.
// All public declarations belong in this file.
package installation

import (
	"context"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/download"
)

// Read loads an installation record for the requested target platform.
// The platform is a requirement, not a claim about a previous installation.
// Call Verify to check that the required files are actually available.
func Read(paths config.Paths, id string, platform core.Platform) (core.Installation, error) {
	return read(paths, id, platform)
}

// Files resolves required downloads using the local, verified asset index.
// Missing or damaged payloads are still returned so they can be repaired.
func Files(ctx context.Context, paths config.Paths, installed core.Installation) ([]download.File, error) {
	return requiredFiles(ctx, paths, installed)
}

// Verify checks all required local files without network requests or repairs.
// A successful check describes the files at this instant, not a lasting status.
func Verify(ctx context.Context, paths config.Paths, installed core.Installation) error {
	return verify(ctx, paths, installed)
}

// Publish atomically publishes version metadata after all downloads succeed.
func Publish(installed core.Installation, metadata []byte) error { return publish(installed, metadata) }
