# terminal-mcp

`terminal-mcp` 是一个本地 MCP Server，用于把已有的 tmux pane 暴露给 MCP Host。一个 tmux pane 会被视为一个 terminal session，session ID 形如 `tmux:%1`。

## 功能

- `terminal_list_sessions`：列出当前 tmux pane。
- `terminal_read_session`：读取指定 pane 的最近输出。
- `terminal_send_text`：向指定 pane 发送原始文本，不自动追加换行。
- `terminal_run_command`：在指定 pane 当前 shell 中执行命令，返回输出、退出码、耗时和截断状态。
- `terminal_interrupt`：向指定 pane 发送一次 `Ctrl-C`。

## 安装

```bash
go build -o terminal-mcp ./cmd/terminal-mcp
```

运行前需要本机可执行 `tmux`。本项目不会创建、关闭或调整 tmux session/window/pane，只操作已经存在的 pane。

## MCP Host 配置

```json
{
  "mcpServers": {
    "terminal": {
      "command": "/path/to/terminal-mcp",
      "args": [],
      "env": {
        "TERMINAL_MCP_AUDIT_LOG_PATH": "/Users/ycgao/.terminal-mcp/audit.jsonl"
      }
    }
  }
}
```

## Codex 配置

先构建一个固定路径的二进制：

```bash
go build -o /usr/local/bin/terminal-mcp ./cmd/terminal-mcp
```

然后在 `~/.codex/config.toml` 中添加：

```toml
[mcp_servers.terminal]
command = "/usr/local/bin/terminal-mcp"
args = []
enabled = true
startup_timeout_sec = 10.0
tool_timeout_sec = 300.0

[mcp_servers.terminal.env]
TERMINAL_MCP_AUDIT_LOG_PATH = "/Users/ycgao/.terminal-mcp/audit.jsonl"
```

如果不想安装到 `/usr/local/bin`，也可以把 `command` 改成本仓库构建出的二进制绝对路径。

## 配置项

| 环境变量 | 默认值 | 说明 |
| --- | ---: | --- |
| `TERMINAL_MCP_AUDIT_LOG_PATH` | `~/.terminal-mcp/audit.jsonl` | 审计日志 JSONL 路径 |
| `TERMINAL_MCP_DEFAULT_TIMEOUT_MS` | `30000` | 默认命令超时时间 |
| `TERMINAL_MCP_MAX_TIMEOUT_MS` | `300000` | 最大命令超时时间 |
| `TERMINAL_MCP_READ_MAX_BYTES` | `65536` | `terminal_read_session` 最大返回字节数 |
| `TERMINAL_MCP_RUN_MAX_BYTES` | `131072` | `terminal_run_command` 最大返回字节数 |

## 安全边界

`terminal-mcp` 不做命令白名单、危险命令拦截、用户确认、鉴权或多用户隔离。MCP Host 调用该 server 等价于获得向本机 tmux pane 输入文本和执行命令的能力。

`terminal_run_command` 只保证在普通 shell prompt 中可靠。如果目标 pane 当前处于 vim、less、top、mysql、python repl、ssh 等交互程序，结果可能超时或失败。

## 开发验证

```bash
make test
make test-e2e
go test ./...
go build ./cmd/terminal-mcp
```

`make test-e2e` 会运行 `test/e2e` 下的真实端到端测试，要求：

- 本机已安装 `tmux`。
- 当前机器可以免交互执行 `ssh root@vm hostname`。

普通 `go test ./...` 不会运行 `test/e2e`，因为端到端测试使用 `e2e` build tag。tmux 内部集成测试仍会在本机未安装 `tmux` 时自动跳过。
