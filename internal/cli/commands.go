// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/Evernight275/cubicstart/internal/app"
	"github.com/Evernight275/cubicstart/internal/auth"
	"github.com/Evernight275/cubicstart/internal/download"
)

const help = `CubicStart — Minecraft Java Edition CLI launcher

Usage:
  cubicstart versions [--type release|snapshot|all] [--limit 20]
  cubicstart install <version|latest> [--dir PATH] [--workers 8]
  cubicstart java check [--java PATH]
  cubicstart login [--client-id ID]
  cubicstart whoami
  cubicstart logout
  cubicstart launch [version] [--instance NAME] [--java PATH] [--memory 2048] [--dir PATH]
                    [--offline --username Player] [--dry-run]
  cubicstart paths [--dir PATH]
  cubicstart version

Flags may appear before or after the version argument.
Data directory: --dir, CUBICSTART_HOME, or the user's config directory/CubicStart.
Login client ID: --client-id, CUBICSTART_CLIENT_ID, or the project's default.
Offline mode is explicit and cannot authenticate to online-mode servers.
`

type usageError struct{ error }

func (e *usageError) Unwrap() error            { return e.error }
func usage(format string, values ...any) error { return &usageError{fmt.Errorf(format, values...)} }

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 130
	}
	var badUsage *usageError
	if errors.As(err, &badUsage) {
		return 2
	}
	var process *exec.ExitError
	if errors.As(err, &process) && process.ExitCode() > 0 {
		return process.ExitCode()
	}
	return 1
}

func run(ctx context.Context, args []string, streams Streams) error {
	if streams.Out == nil {
		streams.Out = io.Discard
	}
	if streams.Err == nil {
		streams.Err = io.Discard
	}
	if len(args) == 0 {
		_, err := io.WriteString(streams.Out, help)
		return err
	}
	command := args[0]
	if command == "help" || command == "--help" || command == "-h" {
		_, err := io.WriteString(streams.Out, help)
		return err
	}
	if command == "version" || command == "--version" {
		if len(args) != 1 {
			return usage("version takes no arguments")
		}
		_, err := fmt.Fprintln(streams.Out, "CubicStart "+app.Version)
		return err
	}
	known := map[string]bool{"versions": true, "install": true, "java": true, "login": true, "whoami": true, "logout": true, "launch": true, "paths": true}
	if !known[command] {
		return usage("unknown command %q; run cubicstart --help", command)
	}
	args = args[1:]
	if command == "java" {
		if len(args) == 0 || args[0] != "check" {
			return usage("usage: cubicstart java check [--java PATH]")
		}
		args = args[1:]
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() { fmt.Fprint(streams.Out, help) }
	root := flags.String("dir", os.Getenv("CUBICSTART_HOME"), "launcher data directory")
	var kind, javaPath, clientID, username, instanceID string
	var limit, workers, memory int
	var offline, dryRun bool
	switch command {
	case "versions":
		flags.StringVar(&kind, "type", "release", "version type")
		flags.IntVar(&limit, "limit", 20, "number of results (0 = all)")
	case "install":
		flags.IntVar(&workers, "workers", 8, "concurrent downloads (1-32)")
	case "java":
		flags.StringVar(&javaPath, "java", "", "Java executable")
	case "login":
		defaultID := os.Getenv("CUBICSTART_CLIENT_ID")
		if defaultID == "" {
			defaultID = app.DefaultClientID
		}
		flags.StringVar(&clientID, "client-id", defaultID, "Microsoft application client ID")
	case "launch":
		flags.StringVar(&instanceID, "instance", "", "independent game instance (defaults to the version ID)")
		flags.StringVar(&javaPath, "java", "", "Java executable")
		flags.IntVar(&memory, "memory", 2048, "maximum heap in MiB")
		flags.BoolVar(&offline, "offline", false, "explicit local offline identity")
		flags.StringVar(&username, "username", "Player", "offline player name")
		flags.BoolVar(&dryRun, "dry-run", false, "print a redacted launch plan without starting Java")
	}
	positionals, err := parseFlags(flags, args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return usage("%s: %v", command, err)
	}
	expected := 0
	if command == "install" {
		expected = 1
	}
	if command == "launch" {
		if len(positionals) > 1 || (len(positionals) == 0 && instanceID == "") {
			return usage("launch requires a version or --instance NAME; run cubicstart --help")
		}
	} else if len(positionals) != expected {
		return usage("%s expects %d version argument(s); run cubicstart --help", command, expected)
	}
	if command == "versions" && (limit < 0 || (kind != "release" && kind != "snapshot" && kind != "all")) {
		return usage("invalid --type or --limit")
	}
	if command == "install" && (workers < 1 || workers > 32) {
		return usage("--workers must be between 1 and 32")
	}
	if command == "launch" && (memory < 512 || memory > 65536) {
		return usage("--memory must be between 512 and 65536 MiB")
	}
	options := app.Options{Root: *root, Workers: workers}
	switch command {
	case "versions":
		versions, err := app.Versions(ctx, options, kind)
		if err != nil {
			return err
		}
		if limit > 0 && len(versions) > limit {
			versions = versions[:limit]
		}
		for _, version := range versions {
			if _, err := fmt.Fprintf(streams.Out, "%s\t%s\t%s\n", version.ID, version.Type, version.ReleaseTime.Format("2006-01-02")); err != nil {
				return err
			}
		}
	case "install":
		installed, err := app.Install(ctx, options, positionals[0], func(progress download.Progress) {
			if progress.Completed == 1 || progress.Completed%100 == 0 || progress.Completed == progress.Total {
				fmt.Fprintf(streams.Err, "Files ready: %d/%d\n", progress.Completed, progress.Total)
			}
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(streams.Out, "Installed Minecraft %s (Java %d required)\n", installed.Version.ID, installed.Version.JavaVersion.Major)
	case "java":
		runtime, err := app.CheckJava(ctx, javaPath)
		if err != nil {
			return err
		}
		fmt.Fprintf(streams.Out, "Java %s (%s)\n%s\n", runtime.Version, runtime.Arch, runtime.Path)
	case "login":
		profile, err := app.Login(ctx, clientID, func(code auth.DeviceCode) {
			fmt.Fprintf(streams.Out, "Open %s\nEnter code: %s\nExpires: %s\n", code.URL, code.Code, code.ExpiresAt.Local().Format("15:04:05"))
		})
		if err != nil {
			return err
		}
		fmt.Fprintf(streams.Out, "Logged in as %s (%s)\n", profile.Name, profile.ID)
	case "whoami":
		profile, err := app.Whoami(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(streams.Out, "%s\t%s\n", profile.Name, profile.ID)
	case "logout":
		if err := app.Logout(); err != nil {
			return err
		}
		fmt.Fprintln(streams.Out, "Stored login removed.")
	case "paths":
		paths, err := app.DataPaths(*root)
		if err != nil {
			return err
		}
		return json.NewEncoder(streams.Out).Encode(paths)
	case "launch":
		var version string
		if len(positionals) == 1 {
			version = positionals[0]
		}
		plan, err := app.PrepareLaunch(ctx, *root, app.LaunchOptions{Version: version, Instance: instanceID, JavaPath: javaPath, Username: username, Offline: offline, Preview: dryRun, MemoryMB: memory})
		if err != nil {
			return err
		}
		defer plan.Close()
		if dryRun {
			return json.NewEncoder(streams.Out).Encode(struct {
				Executable, Directory string
				Args                  []string
			}{plan.Executable, plan.Directory, plan.RedactedArgs()})
		}
		return app.RunGame(ctx, plan, streams.In, streams.Out, streams.Err)
	}
	return nil
}

func parseFlags(flags *flag.FlagSet, args []string) ([]string, error) {
	var options, positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		options = append(options, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if name == "h" || name == "help" {
			return nil, flags.Parse([]string{"-h"})
		}
		option := flags.Lookup(name)
		if option == nil {
			return nil, fmt.Errorf("unknown flag %q", arg)
		}
		boolean, isBool := option.Value.(interface{ IsBoolFlag() bool })
		if !hasValue && !(isBool && boolean.IsBoolFlag()) {
			i++
			if i >= len(args) {
				return nil, fmt.Errorf("flag %s requires a value", arg)
			}
			options = append(options, args[i])
		}
	}
	if err := flags.Parse(options); err != nil {
		return nil, err
	}
	return positionals, nil
}
