# internal 分层

按职责组织包，通过 `app` 协调版本查询、安装、登录和启动流程。每个包的公共契约统一定义在 `interface.go` 中，具体实现放在其他文件中。

```text
cmd/cubicstart/       程序入口：建立取消信号，调用 cli，返回退出码
internal/
  cli/               命令层：命令解析、参数校验、进度和结果展示
  app/               应用层：组织登录、安装、启动等完整用例
  core/              核心概念：版本、资源、规则、玩家、游戏会话、Java 环境
  auth/              微软登录、令牌刷新、Minecraft 玩家身份
  metadata/          版本列表、版本描述、资源索引、平台条件规则
  download/          文件下载、重试、完整性校验、下载缓存
  java/              Java 路径查找、版本检查、运行环境选择
  launch/            启动参数构建、原生库准备、游戏进程管理
  config/            配置读写、数据目录和实例路径管理
```

## 依赖方向

```text
cli → app → auth / metadata / download / java / launch / config
       app / 职责包 → core → 标准库
```

- `cli` 负责命令行交互，通过 `app` 执行业务，不直接下载文件或启动游戏。
- 共享的游戏数据直接使用 `core` 类型；设备授权码、下载进度等操作专用数据使用所属包的类型，不经 `app` 再次定义类型别名。
- `app` 协调各包，向调用方返回结果和错误，不直接输出终端内容。
- 职责包不依赖 `cli` 或 `app`；确有需要时可以单向依赖其他职责包，避免循环依赖。
- `launch` 接收已经解析的启动信息，负责构建参数和管理进程；登录、下载等步骤由 `app` 协调。
- `config` 保存普通配置；敏感令牌的存储由 `auth` 负责，避免写入普通配置文件。
- 跨包共享的核心模型统一放在 `core`；操作参数、下载任务、设备授权码、目录布局和带清理行为的启动计划仍留在对应职责包。
- `core` 只依赖标准库，不依赖其他 `internal` 包或第三方库，不承载网络、凭据存储和进程操作；根目录架构测试检查其导入边界。

## 核心概念

`core/interface.go` 是以下 15 个共享模型的唯一定义位置，不在原包保留转发别名：

- 玩家与会话：`Player`、`Session`。认证结果直接用于启动，离线身份也使用同一会话模型。
- 运行环境：`JavaRuntime`、`Platform`。
- 游戏版本：`VersionManifest`、`VersionSummary`、`GameVersion`。
- 文件、依赖与资源：`Artifact`、`Library`、`AssetIndex`、`AssetObject`、`LogConfig`。
- 条件与参数：`Rule`、`OSRule`、`Argument`。参数值的 JSON 编码规则由模型自身处理。

## 实现约定

- 每个包统一使用 `interface.go` 作为唯一对外入口，例如 `cli/interface.go`、`auth/interface.go`。后续新增包也遵守这一命名约定。
- Go 只能导入包，不能导入单个文件。这里通过限制公开声明的位置，实现统一对外接口；并不改变 Go 的导入和编译规则。
- 导出的接口、数据类型、函数、变量、常量，以及导出类型的公开方法，只能定义在入口文件中。需要实现对象时，具体实现类型保持不导出。
- 其他文件只定义内部类型和函数。未导出实现类型为满足接口而定义的公开方法，可以留在实现文件中。
- 入口文件是公共契约的集中位置，并不要求每个包都建立 Go 接口或构造函数。
- 根目录的 `architecture_test.go` 自动检查入口文件与公开声明位置，运行 `go test ./...` 即可验证；测试文件不受公开声明位置限制。
- 网络请求、下载和进程操作接收 `context.Context`，支持取消和超时。
- 各包返回带上下文的错误，由 `cli` 统一展示。
- 进度和日志通过回调或输出接口传递，避免底层包绑定终端界面。
- 跨包接口按实际用例定义在提供方的入口文件中；调用方内部的适配接口保持不导出，不为尚未实现的功能预设接口。

## 按职责公开行为

| 包 | 公开行为 |
| --- | --- |
| `app` | `Versions`、`Install`、`CheckJava`、`Login`、`Whoami`、`Logout`、`PrepareLaunch`、`RunGame`、`DataPaths`，各自对应明确用例 |
| `metadata` | `Catalog`：读取版本清单和版本描述 |
| `download` | `Downloader`：下载并校验单个文件或一批文件 |
| `auth` | `Login`、`RefreshSession`、`Logout`，分别负责授权、刷新会话和清除凭据 |
| `java` | `Check`：检查指定 Java 可执行文件 |
| `launch` | `Prepare`、`Run`：准备游戏启动参数、执行游戏进程 |

不建立统一的 `Service` 总接口。无状态操作使用函数；需要持有网络配置的目录读取器和下载器，按具体职责命名。`app` 的安装和启动编排分别实现在 `install.go`、`launch.go`，版本查询和玩家查询实现在 `catalog.go`、`account.go`。
