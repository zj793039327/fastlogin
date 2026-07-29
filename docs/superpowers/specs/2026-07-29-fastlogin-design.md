# fastlogin — 交互式命令启动器

## 概述

`fastlogin` 是一个基于终端的交互式命令启动器。启动后展示一个可分组、可折叠、可搜索的服务器/命令列表，用户选中条目后，工具退出 TUI 并把当前终端移交给对应的执行器 (Runner) 建立交互会话。会话结束后程序退出。

SSH 是第一个内置类型，但工具定位为 **通用启动器**：通过 Runner 接口可插拔地扩展新的会话类型（如数据库连接、任意命令等）。

## 目标与非目标

**目标:**
- 交互式选择服务器/命令并启动会话
- 配置来自 YAML 文件，同时支持合并读取 `~/.ssh/config`
- 支持 SSH 密码登录与 PEM 私钥登录
- 支持按分组组织、可折叠、可搜索过滤
- 架构上为未来扩展（新的会话类型）留好插拔口子

**非目标 (MVP 范围外):**
- 配置加密 / keyring（密码 MVP 明文存 YAML）
- 使用记录 / 常用置顶
- 远程批量执行
- 云端配置同步
- 文件监听热加载（改了配置重启即生效）
- 会话结束后返回 TUI（会话结束 = 程序结束）

## 数据模型

### Entry

每个可启动的条目是一个 `Entry`。所有类型共享一组公共字段，`type` 区分行为：

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | 是 | 列表显示名 |
| `type` | 否 | 默认 `ssh`。可选 `ssh` / `command` |
| `host` | ssh 必填 | 主机地址 |
| `user` | ssh 必填 | 登录用户 |
| `port` | ssh 否 | 端口，默认 22 |
| `auth` | ssh 必填 | 认证：`password` 或 `pem`（+可选 `passphrase`） |
| `tags` | 否 | 用于搜索过滤 |
| `command` | command 必填 | 要 exec 的命令（经 shell 解析） |

### Group

一组 Entry，带名字和可选描述，UI 上可折叠。

### 配置文件

默认路径 `~/.config/fastlogin/config.yaml`。

```yaml
# 顶层分组
groups:
  - name: 生产环境
    description: 线上机器，谨慎操作
    entries:
      - name: web-01
        type: ssh
        host: 10.0.1.10
        user: root
        port: 22
        auth:
          password: "secret"
        tags: [web, prod]

      - name: db-01
        type: ssh
        host: 10.0.1.20
        user: admin
        auth:
          pem: ~/.ssh/db01.pem

  - name: 开发环境
    entries:
      - name: local-dev
        type: ssh
        host: 127.0.0.1
        user: dev
        auth:
          password: "dev123"

      - name: mysql-repl
        type: command
        command: mysql -h 10.0.1.20 -u admin -p

# 未分组的散装条目
entries:
  - name: quick-ssh
    type: ssh
    host: 1.2.3.4
    user: root
    auth:
      pem: ~/.ssh/id_rsa

# 是否合并读取 ~/.ssh/config 中的 Host
include_ssh_config: true
```

### `.ssh/config` 合并

当 `include_ssh_config: true` 时，读取 `~/.ssh/config` 中的 `Host` 块（解析 `HostName`/`User`/`Port`/`IdentityFile`），转换成等效的 `type: ssh` + `auth.pem` 条目，归入一个名为 `ssh-config` 的虚拟分组。与 YAML 定义的条目去重（以 `user@host:port` 为键）。

## 架构

### 三层 + Runner 插拔

```
┌─────────────────────────────────────────┐
│  TUI 层 (bubbletea)                      │
│  渲染列表、键盘交互、选中后调用 Runner    │
└──────────────┬──────────────────────────┘
               │ Entry
┌──────────────▼──────────────────────────┐
│  Runner 层 (接口 + 实现)                 │
│  Runner interface { Run(Entry) error }   │
│  ├─ SSHRunner   (golang.org/x/crypto/ssh)│
│  └─ CommandRunner (os/exec + PTY)        │
└──────────────┬──────────────────────────┘
               │
┌──────────────▼──────────────────────────┐
│  Config 层                               │
│  ├─ YAML reader                          │
│  ├─ SSHConfig reader (~/.ssh/config)     │
│  └─ Merge (合并两组，去重)               │
└─────────────────────────────────────────┘
```

### Runner 接口

```go
// Runner 启动一个交互式会话。实现方负责接管终端。
type Runner interface {
    Run(ctx context.Context, e Entry) error
}

// Registry 按 type 名称注册 Runner
type Registry struct{ runners map[string]Runner }
func (r *Registry) Register(typeName string, runner Runner)
func (r *Registry) Get(e Entry) (Runner, error) // 按 e.Type 查找
```

启动时在 `main.go` 注册内置 Runner：
```go
reg.Register("ssh", &SSHRunner{})
reg.Register("command", &CommandRunner{})
```

未来加新类型只需实现 `Runner` 接口 + `Register` 一行，TUI 和 Config 无需改动。

### SSHRunner 实现

使用 `golang.org/x/crypto/ssh` 库自建连接，**不 fork 外部 ssh 进程**，也**不用 `syscall.Exec` 替换进程**。

流程：
1. 按 `auth` 构造 `ssh.ClientConfig`（`password` 或 `pem`+可选 `passphrase`）。
2. `ssh.Dial` 建立连接。
3. `client.NewSession()` 建会话。
4. 请求 PTY（`session.RequestPty`），设置当前终端尺寸。
5. 把 session 的 stdin/stdout/stderr 接到当前终端。
6. `session.Shell()` 启动远程 shell。
7. `session.Wait()` 阻塞直到会话结束，返回。

不依赖 `sshpass`，密码不出现在进程列表。

### CommandRunner 实现

使用 `os/exec`，统一走 PTY（保证交互式命令正常）：
1. `exec.Command("sh", "-c", command)`（经 shell 解析，支持管道/重定向）。
2. `pty.Start(cmd)` 启动，PTY 自动作为子进程的 stdin/stdout/stderr。
3. `io.Copy` 在子进程 stdin/stdout 与当前终端 `os.Stdin/os.Stdout` 间双向转发。
4. `cmd.Wait()`。

### 终端接管契约（重要）

会话期间 Runner 暂时持有终端控制权。会话结束后：
- Runner.Run 正常返回 → `main` 退出进程（exit 0），**不返回 TUI**。
- Runner.Run 返回 error → `main` 打印到 stderr，退出进程（exit 非 0）。

不做 "TUI 重新接管终端" 的复杂逻辑。

### 终端尺寸同步

SSHRunner 请求 PTY 时读取当前终端尺寸（`golang.org/x/term` 的 `GetSize`）。CommandRunner 通过 PTY 继承。MVP 不处理会话中的窗口 resize 事件（后续可加 SIGWINCH 监听）。

## TUI 交互

### 布局

```
 fastlogin
 ─────────────────────────────────────────────────
 ▶ 生产环境                       (3)
 ▼ 开发环境                       (2)
     web-01   root@10.0.1.10  [ssh]
     db-01    admin@10.0.1.20 [ssh]
   ssh-config                       (5)
 quick-ssh   root@1.2.3.4    [ssh]
 ────────────────────────────────────────────────
 ↑↓ navigate · →/← expand-collapse · ⏎ connect · / search · q quit
```

- 组标题行显示条目数；`▶` 折叠 / `▼` 展开。
- 条目行显示 `name`、`user@host`、`[type]`。
- 光标高亮当前行。

### 按键

| 键 | 作用 |
|----|------|
| `↑` / `↓` | 上下移动光标 |
| `→` / `←` / `Tab` | 组上展开/折叠；条目上无操作 |
| `⏎` Enter | 执行选中条目（调 Runner） |
| `/` | 进入搜索模式，输入即过滤 name/host/user/tags |
| `Esc` | 退出搜索 / 清除过滤 |
| `q` / `Ctrl+C` | 退出 |

### 搜索过滤

输入关键字后只显示匹配条目（name/host/user/tags 任一命中）。组内无匹配则整组隐藏。

### 配置加载时机

每次启动 TUI 重新读配置。不监听文件变更。

## 目录结构

```
20260729_fastlogin/
├── go.mod
├── main.go                       # 装配：加载配置 → 注册 Runner → 启动 TUI
├── internal/
│   ├── config/
│   │   ├── config.go             # Entry/Group/Auth/Config 结构体 + 加载入口
│   │   ├── yaml.go               # YAML 文件解析
│   │   └── sshconfig.go          # ~/.ssh/config 解析 + 合并去重
│   ├── runner/
│   │   ├── runner.go             # Runner 接口 + Registry
│   │   ├── ssh.go                # SSHRunner
│   │   ├── command.go            # CommandRunner
│   │   └── pty.go                # 共用 PTY 辅助
│   └── tui/
│       ├── model.go              # bubbletea Model（状态）
│       ├── view.go               # 渲染（分组/折叠/高亮）
│       └── update.go             # 键盘事件处理
└── config.example.yaml           # 示例配置随仓库分发
```

每个文件单一职责，包之间只通过接口和数据结构通信。

## 依赖

- `github.com/charmbracelet/bubbletea` — TUI 框架
- `github.com/charmbracelet/lipgloss` — TUI 样式
- `golang.org/x/crypto/ssh` — SSH 客户端
- `golang.org/x/term` — 终端尺寸 / raw mode
- `golang.org/x/sys/unix` — PTY（非 Windows）
- `gopkg.in/yaml.v3` — YAML 解析

## 错误处理

- 配置文件不存在：打印提示 + 给出生成示例配置的命令后退出。
- 配置文件解析失败：打印行号+错误后退出。
- `.ssh/config` 读取失败（不存在/无权限）：非致命，跳过该数据源，其余条目照常。
- Runner 找不到（未知 type）：打印 "未知类型: X" 后退出。
- SSH 连接失败：打印错误（含 host）后退出。
- 命令执行失败：打印错误（含 command）后退出。

## 测试策略

- `config` 包：YAML/SSHConfig 解析与合并的单元测试，覆盖去重逻辑。
- `runner` 包：
  - Registry 注册/查找测试。
  - Runner 的连接逻辑较难单测（依赖真实服务器），通过接口抽象保证可替换，端到端靠手动验证。
- `tui` 包：Model 状态转换的单元测试（光标移动、折叠、搜索过滤），不依赖真实渲染。

## 未来扩展（明确不在 MVP）

按优先级，后续可加：
- `type: mysql` / `type: postgres` 等 Runner（无需改 TUI/Config）。
- 密码 keyring 加密存储。
- 使用记录 + 常用置顶排序。
- 会话中 SIGWINCH 窗口 resize 同步。
- 配置热加载（文件监听）。
- 会话结束后返回 TUI（改回方案 A）。
