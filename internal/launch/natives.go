// SPDX-License-Identifier: GPL-3.0-only

package launch

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func extractNatives(ctx context.Context, options Options, dest string) error {
	for _, library := range options.Version.Libraries {
		if _, ok := library.Natives[options.Platform.OS]; !ok {
			continue
		}
		files, err := metadata.LibraryFiles(library, options.Platform)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			continue
		}
		artifact := files[len(files)-1]
		path, err := config.Join(options.Paths.Libraries, artifact.Path)
		if err != nil {
			return err
		}
		if err := extractArchive(ctx, path, dest, library.Extract.Exclude); err != nil {
			return err
		}
	}
	return nil
}

func extractArchive(ctx context.Context, path, dest string, excludes []string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open native archive: %w", err)
	}
	defer archive.Close()
	var total uint64
	for _, file := range archive.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		skip := strings.HasPrefix(file.Name, "META-INF/")
		for _, prefix := range excludes {
			skip = skip || strings.HasPrefix(file.Name, prefix)
		}
		if skip {
			continue
		}
		name := strings.TrimSuffix(file.Name, "/")
		output, err := config.Join(dest, name)
		if err != nil {
			return fmt.Errorf("unsafe native archive: %w", err)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("native archive contains a symlink")
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(output, 0755); err != nil {
				return err
			}
			continue
		}
		total += file.UncompressedSize64
		if file.UncompressedSize64 > 128<<20 || total > 512<<20 {
			return fmt.Errorf("native archive exceeds extraction limit")
		}
		if err := extractFile(file, output); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(file *zip.File, output string) error {
	input, err := file.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	writer, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(writer, io.LimitReader(input, (128<<20)+1))
	closeErr := writer.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
