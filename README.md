# CubicStart

使用 Go 编写的 Minecraft Java 版命令行启动器，当前版本为 `0.1.0`，优先验证 Windows。

## 第一版功能

- 查询官方版本列表，按正式版或快照筛选。
- 安装现代原版客户端、依赖、资源和日志配置，支持并发下载、重试、SHA-1 校验与缓存修复。
- 检查本机 Java 路径、版本和架构，启动时按游戏元数据检查 Java 要求。
- 微软设备代码登录、会话刷新、账号查询和退出登录；使用系统凭据库保存刷新令牌。
- 按版本隔离游戏目录，构建启动参数并转发游戏日志和退出状态。
- 显式离线启动，以及隐藏访问令牌的 `--dry-run` 参数预览。

当前支持使用 `arguments.jvm` / `arguments.game` 的现代原版版本，已验证 1.21.1 的安装和参数生成。旧版启动参数、旧资源目录、模组加载器、整合包和自动安装 Java 暂不支持。

## 构建与使用

需要项目 `go.mod` 指定的 Go 版本，以及目标游戏所需的 64 位 Java。以下示例使用 Windows PowerShell：

```powershell
go build -o bin/cubicstart.exe ./cmd/cubicstart

.\bin\cubicstart.exe --help
.\bin\cubicstart.exe versions --limit 10
.\bin\cubicstart.exe java check
.\bin\cubicstart.exe install 1.21.1
.\bin\cubicstart.exe login
.\bin\cubicstart.exe whoami
.\bin\cubicstart.exe launch 1.21.1 --memory 2048
```

1.21.1 的元数据要求 Java 21；其他版本的 Java 要求由该版本元数据决定。可用 `--java` 指定 Java **可执行文件**，路径包含空格时加引号：

```powershell
.\bin\cubicstart.exe launch 1.21.1 --java "C:\Program Files\Java\jdk-21\bin\java.exe"
```

查看启动参数而不运行游戏：

```powershell
.\bin\cubicstart.exe launch 1.21.1 --offline --username Player --dry-run
```

离线身份不会提供正版服务器认证；登录失败也不会自动切换到离线模式。`--dry-run` 会清理预览期间创建的临时原生库目录，因此输出用于检查，不应直接复制为启动脚本。

Linux / macOS 可使用 `go build -o bin/cubicstart ./cmd/cubicstart` 构建；这些平台尚未完成实际游戏验收。凭据存储依赖对应系统的 Keychain / Secret Service，服务不可用时会明确报错，不回退到明文文件。

## 命令

| 命令 | 用途 |
| --- | --- |
| `versions --type release --limit 20` | 查询版本；`--type` 支持 `release`、`snapshot`、`all`，`--limit 0` 显示全部 |
| `install <version\|latest> --workers 8` | 安装指定版本；`latest` 指最新正式版，工作线程数为 1–32 |
| `java check --java PATH` | 检查 Java；不指定时使用 PATH 中的 `java` |
| `login --client-id ID` | 使用个人微软账号登录，终端显示授权网址和验证码 |
| `whoami` | 刷新会话并显示当前玩家名称、UUID |
| `logout` | 删除本启动器在系统凭据库中保存的登录状态 |
| `launch <version> --memory 2048` | 启动已安装版本，内存单位为 MiB |
| `paths` | 显示数据目录布局 |
| `version` | 显示启动器版本 |

参数可以放在版本号之前或之后。所有子命令均接受 `--dir PATH`；账号凭据独立保存在系统凭据库中，当前仅支持一个账号。

## 数据目录

优先级为 `--dir` → `CUBICSTART_HOME` → 用户配置目录下的 `CubicStart`；Windows 默认是 `%APPDATA%\CubicStart`。

```text
versions/<version>/       客户端 JAR、完成安装后的版本 JSON
libraries/                公共 Java 依赖缓存
assets/indexes/           资源索引
assets/objects/           按哈希存储的资源
assets/log_configs/       官方日志配置
instances/<version>/      各版本的存档、设置和游戏日志
```

中断或失败后重新执行相同的 `install` 命令，会校验并复用已经完整下载的文件，重新下载缺失或损坏文件。安装文件与其他启动器的数据目录分开管理。

## 微软应用配置

Client ID 的优先级为 `login --client-id` → `CUBICSTART_CLIENT_ID` → 项目默认值。Client ID 是公开应用标识；本项目不使用 Client Secret。

在 Entra **应用注册**中，需要使用 **应用程序（客户端）ID**，并配置：

1. 受支持的账号类型包含个人 Microsoft 帐户。清单的 `signInAudience` 可以是 `PersonalMicrosoftAccount` 或 `AzureADandPersonalMicrosoftAccount`。
2. 清单中 `api.requestedAccessTokenVersion` 设为 `2`；旧版清单对应 `accessTokenAcceptedVersion`。这是支持个人账号的配套要求。
3. 允许公共客户端流，以支持设备代码授权。新版入口为“身份验证（预览）→ 设置”；也可以在清单中将 `isFallbackPublicClient` 设为 `true`。
4. 应用具备访问 Xbox / Minecraft 服务所需的配置和服务端许可。仅成功注册应用，并不能保证 Minecraft 服务一定接受该应用。

设备授权使用 `XboxLive.SignIn XboxLive.offline_access`。浏览器授权完成后，启动器交换 Xbox / Minecraft 令牌并检查游戏权限和玩家档案。刷新令牌保存在系统凭据库中，访问令牌只在内存和游戏进程参数中使用；普通配置和 dry-run 输出不包含这些令牌。

如果出现 `unauthorized_client`，先核对 Client ID 和账号类型；如果出现 `invalid_client`，检查公共客户端流设置；Xbox 错误会显示 `XErr` 编号，便于检查 Xbox 档案或家庭权限。

参考：[微软设备代码授权](https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-device-code)、[受支持的账号类型](https://learn.microsoft.com/en-us/entra/identity-platform/supported-accounts-validation)。

## 开发与验证

```powershell
go test ./...
go vet ./...
```

测试使用本地 HTTP 服务和内存凭据存储，不会调用真实账号登录或修改系统中的账号凭据。覆盖下载重试、完整性校验、缓存修复、失败安装、平台规则、令牌刷新、原生库路径检查和进程退出状态。

`internal` 下每个包的唯一对外入口必须是 `interface.go`。详见 [分层说明](internal/README.md) 和 [强制约束](internal/AGENTS.md)，根目录架构测试会检查这项规则。

跨包共享的版本、资源、规则、玩家、会话和 Java 环境模型集中定义在 [internal/core/interface.go](internal/core/interface.go)。`core` 只依赖标准库，其他职责包直接使用这些模型。

## License

CubicStart is licensed under the GNU General Public License, version 3 only
(`GPL-3.0-only`). See [LICENSE](LICENSE) for the full license text.

This program is distributed without any warranty, including the implied warranties
of merchantability or fitness for a particular purpose.

第三方组件保留各自许可证，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。Minecraft 游戏本体和资源不包含在本仓库的 GPL 授权范围内。
