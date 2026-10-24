// SPDX-License-Identifier: GPL-3.0-only

package launch

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func offlineSession(name string) (core.Session, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return core.Session{}, fmt.Errorf("offline username must contain 1-16 letters, digits or underscores")
	}
	hash := md5.Sum([]byte("OfflinePlayer:" + name))
	hash[6] = (hash[6] & 0x0f) | 0x30
	hash[8] = (hash[8] & 0x3f) | 0x80
	return core.Session{Player: core.Player{Name: name, ID: hex.EncodeToString(hash[:])}, AccessToken: "0", UserType: "legacy"}, nil
}

func closePlan(plan Plan) error {
	if plan.nativeDir == "" {
		return nil
	}
	return os.RemoveAll(plan.nativeDir)
}

func redactedArgs(args []string) []string {
	result := append([]string(nil), args...)
	for i := range result {
		if i > 0 && (args[i-1] == "--accessToken" || args[i-1] == "--clientId" || args[i-1] == "--xuid") {
			result[i] = "<redacted>"
		}
	}
	return result
}

func requireFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing installed file %s; run install again: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("installed path is not a regular file: %s", path)
	}
	return nil
}

func prepare(ctx context.Context, options Options) (plan Plan, err error) {
	if err := ctx.Err(); err != nil {
		return plan, err
	}
	version := options.Version
	if err := config.ValidID(version.ID); err != nil {
		return plan, err
	}
	if options.MemoryMB < 512 || options.MemoryMB > 65536 {
		return plan, fmt.Errorf("memory must be between 512 and 65536 MiB")
	}
	if options.Java.Major < version.JavaVersion.Major {
		return plan, fmt.Errorf("Minecraft %s requires Java %d; selected Java is %d", version.ID, version.JavaVersion.Major, options.Java.Major)
	}
	versionDir := filepath.Join(options.Paths.Versions, version.ID)
	clientJar := filepath.Join(versionDir, version.ID+".jar")
	if err := requireFile(clientJar); err != nil {
		return plan, err
	}
	if err := config.ValidID(version.AssetIndex.ID); err != nil {
		return plan, err
	}
	if err := requireFile(filepath.Join(options.Paths.Assets, "indexes", version.AssetIndex.ID+".json")); err != nil {
		return plan, err
	}
	var classpath []string
	for _, library := range version.Libraries {
		allowed, ruleErr := metadata.Allowed(library.Rules, options.Platform)
		if ruleErr != nil {
			return plan, ruleErr
		}
		if !allowed || library.Downloads.Artifact == nil {
			continue
		}
		path, pathErr := config.Join(options.Paths.Libraries, library.Downloads.Artifact.Path)
		if pathErr != nil {
			return plan, pathErr
		}
		if err := requireFile(path); err != nil {
			return plan, err
		}
		classpath = append(classpath, path)
	}
	classpath = append(classpath, clientJar)
	gameDir := filepath.Join(options.Paths.Instances, version.ID)
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		return plan, err
	}
	nativeDir, err := os.MkdirTemp(versionDir, "natives-")
	if err != nil {
		return plan, err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(nativeDir)
		}
	}()
	if err = extractNatives(ctx, options, nativeDir); err != nil {
		return plan, err
	}
	values := map[string]string{
		"natives_directory": nativeDir, "launcher_name": "CubicStart", "launcher_version": "0.1.0",
		"classpath": strings.Join(classpath, string(os.PathListSeparator)), "classpath_separator": string(os.PathListSeparator),
		"library_directory": options.Paths.Libraries, "version_name": version.ID, "version_type": version.Type,
		"game_directory": gameDir, "assets_root": options.Paths.Assets, "assets_index_name": version.AssetIndex.ID,
		"auth_player_name": options.Session.Player.Name, "auth_uuid": options.Session.Player.ID, "auth_access_token": options.Session.AccessToken,
		"user_type": options.Session.UserType, "clientid": options.Session.ClientID, "auth_xuid": options.Session.XUID,
		"user_properties": "{}",
	}
	args, err := expandArguments(version.Arguments.JVM, options.Platform, values)
	if err != nil {
		return plan, err
	}
	args = append(args, "-Xmx"+strconv.Itoa(options.MemoryMB)+"M")
	if logging, ok := version.Logging["client"]; ok {
		logPath, pathErr := config.Join(filepath.Join(options.Paths.Assets, "log_configs"), logging.File.ID)
		if pathErr != nil {
			return plan, pathErr
		}
		if err = requireFile(logPath); err != nil {
			return plan, err
		}
		argument, expandErr := expand(logging.Argument, map[string]string{"path": logPath})
		if expandErr != nil {
			return plan, expandErr
		}
		args = append(args, argument)
	}
	args = append(args, version.MainClass)
	gameArgs, err := expandArguments(version.Arguments.Game, options.Platform, values)
	if err != nil {
		return plan, err
	}
	args = append(args, gameArgs...)
	return Plan{Executable: options.Java.Path, Directory: gameDir, Args: args, nativeDir: nativeDir}, nil
}

func expandArguments(arguments []core.Argument, platform core.Platform, values map[string]string) ([]string, error) {
	var result []string
	for _, arg := range arguments {
		allowed, err := metadata.Allowed(arg.Rules, platform)
		if err != nil {
			return nil, err
		}
		if !allowed {
			continue
		}
		for _, text := range arg.Values {
			value, err := expand(text, values)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}
	}
	return result, nil
}

func expand(text string, values map[string]string) (string, error) {
	var missing string
	pattern := regexp.MustCompile(`\$\{([^}]+)\}`)
	result := pattern.ReplaceAllStringFunc(text, func(token string) string {
		name := token[2 : len(token)-1]
		value, ok := values[name]
		if !ok {
			missing = name
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("unsupported launch placeholder %q", missing)
	}
	return result, nil
}

func run(ctx context.Context, plan Plan, stdin io.Reader, stdout, stderr io.Writer) error {
	defer plan.Close()
	command := exec.CommandContext(ctx, plan.Executable, plan.Args...)
	command.Dir, command.Stdin, command.Stdout, command.Stderr = plan.Directory, stdin, stdout, stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("game process exited: %w", err)
	}
	return nil
}
