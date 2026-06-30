# terminal-mcp 需求规格 v0.1

## 1. 项目目标

`terminal-mcp` 是一个使用 Go 编写的本地 MCP Server。

它的目标是把本机已经存在的终端会话暴露给 MCP Host，使 Agent 可以完成以下核心动作：

1. 查询当前可用的终端会话列表。
2. 读取某个终端会话的输出内容。
3. 向某个终端会话输入文本。
4. 在某个终端会话中执行命令，并返回命令输出。
5. 中断某个正在运行的终端会话。
6. 记录所有操作审计日志。

本项目只做本地终端会话控制，不做远程主机管理、不做权限系统、不做安全拦截。

---

## 2. 技术约束

1. 开发语言：Go。
2. 运行形态：本地 MCP Server。
3. MCP 通信方式：stdio。
4. 终端后端：仅支持 tmux。
5. 操作对象：tmux pane。
6. 一个 tmux pane 在本项目中视为一个 terminal session。
7. 不要求直接控制 Warp、Tabby、iTerm2、WezTerm 等 GUI 终端。
8. 用户可以继续使用 Warp、Tabby 等 GUI 终端，只要里面运行的是 tmux 即可。

---

## 3. 核心概念

### 3.1 terminal session

terminal session 表示一个可操作的终端会话。

在 v0.1 中，一个 terminal session 等价于一个 tmux pane。

每个 session 必须有唯一 ID。

示例：

```text
tmux:%1
tmux:%2
tmux:%3
```

### 3.2 command execution

command execution 表示向某个 session 输入一条 shell 命令，并等待命令结束，然后返回输出。

该能力只保证在普通 shell prompt 中可靠。

如果目标 pane 当前处于 vim、less、top、mysql、python repl、ssh 交互程序等状态，执行结果不保证可靠。

### 3.3 raw input

raw input 表示向目标 session 直接发送文本。

该能力不判断命令是否完成，也不解析返回结果。

---

## 4. 功能范围

v0.1 只包含以下 MCP tools：

1. `terminal_list_sessions`
2. `terminal_read_session`
3. `terminal_send_text`
4. `terminal_run_command`
5. `terminal_interrupt`

除此之外，不增加其他功能。

---

## 5. 非目标

v0.1 明确不做以下功能：

1. 不创建 tmux session。
2. 不关闭 tmux session。
3. 不创建 tmux window。
4. 不关闭 tmux window。
5. 不创建 tmux pane。
6. 不关闭 tmux pane。
7. 不调整 pane 布局。
8. 不切换 GUI 终端焦点。
9. 不控制 Warp 原生窗口。
10. 不控制 Tabby 原生窗口。
11. 不做 SSH profile 管理。
12. 不做命令白名单。
13. 不做危险命令拦截。
14. 不做用户确认机制。
15. 不做鉴权。
16. 不做多用户隔离。
17. 不做数据库存储。
18. 不做 Web UI。
19. 不做 HTTP API。
20. 不做后台 daemon 管理。
21. 不做 shell history 分析。
22. 不做智能 prompt 识别。
23. 不做复杂交互式程序适配。
24. 不做跨机器控制。

---

## 6. MCP Tool 需求

## 6.1 terminal_list_sessions

### 目的

返回当前可操作的 terminal session 列表。

### 输入

无参数。

```json
{}
```

### 输出

返回 session 数组。

```json
{
  "sessions": [
    {
      "id": "tmux:%1",
      "backend": "tmux",
      "name": "work:0.1",
      "session_name": "work",
      "window_index": 0,
      "pane_index": 1,
      "title": "agentgrid-dev",
      "cwd": "/Users/ycgao/code/agentgrid",
      "current_command": "zsh"
    }
  ]
}
```

### 字段要求

| 字段                | 必填 | 说明                     |
| ----------------- | -: | ---------------------- |
| `id`              |  是 | terminal session 唯一 ID |
| `backend`         |  是 | 固定为 `tmux`             |
| `name`            |  是 | 人类可读名称                 |
| `session_name`    |  是 | tmux session 名称        |
| `window_index`    |  是 | tmux window index      |
| `pane_index`      |  是 | tmux pane index        |
| `title`           |  否 | pane title             |
| `cwd`             |  否 | 当前目录                   |
| `current_command` |  否 | 当前前台命令                 |

### 行为要求

1. 只返回当前 tmux 中存在的 pane。
2. 如果没有任何 pane，返回空数组。
3. 不创建新的 pane。
4. 不修改任何终端状态。

---

## 6.2 terminal_read_session

### 目的

读取指定 terminal session 的最近输出内容。

### 输入

```json
{
  "session_id": "tmux:%1",
  "last_n_lines": 200
}
```

### 参数要求

| 参数             | 必填 | 默认值 | 说明                  |
| -------------- | -: | --: | ------------------- |
| `session_id`   |  是 |   无 | terminal session ID |
| `last_n_lines` |  否 | 200 | 读取最近多少行             |

### 输出

```json
{
  "session_id": "tmux:%1",
  "content": "latest terminal output...",
  "line_count": 128,
  "truncated": false
}
```

### 行为要求

1. 读取指定 session 的最近输出。
2. 不向终端输入任何内容。
3. 不改变终端状态。
4. 如果 session 不存在，返回错误。
5. 如果输出超过最大返回限制，允许截断。
6. 截断时必须返回 `truncated: true`。

---

## 6.3 terminal_send_text

### 目的

向指定 terminal session 发送原始文本。

### 输入

```json
{
  "session_id": "tmux:%1",
  "text": "ls -lah\n"
}
```

### 参数要求

| 参数           | 必填 | 说明                  |
| ------------ | -: | ------------------- |
| `session_id` |  是 | terminal session ID |
| `text`       |  是 | 要发送的原始文本            |

### 输出

```json
{
  "session_id": "tmux:%1",
  "accepted": true,
  "bytes_sent": 8
}
```

### 行为要求

1. 将 `text` 原样发送到目标 session。
2. 不自动追加换行。
3. 不等待命令结束。
4. 不解析输出。
5. 不返回命令执行结果。
6. 如果用户希望执行命令，应由调用方在 `text` 中包含 `\n`。
7. 如果 session 不存在，返回错误。

---

## 6.4 terminal_run_command

### 目的

在指定 terminal session 中执行一条 shell 命令，并返回执行结果。

### 输入

```json
{
  "session_id": "tmux:%1",
  "command": "pwd && ls -lah",
  "timeout_ms": 30000
}
```

### 参数要求

| 参数           | 必填 |   默认值 | 说明                  |
| ------------ | -: | ----: | ------------------- |
| `session_id` |  是 |     无 | terminal session ID |
| `command`    |  是 |     无 | 要执行的 shell 命令       |
| `timeout_ms` |  否 | 30000 | 等待命令结束的最长时间         |

### 输出：成功完成

```json
{
  "session_id": "tmux:%1",
  "command": "pwd && ls -lah",
  "status": "completed",
  "exit_code": 0,
  "output": "/Users/ycgao/code/agentgrid\n...",
  "duration_ms": 842,
  "truncated": false
}
```

### 输出：超时

```json
{
  "session_id": "tmux:%1",
  "command": "pytest -q",
  "status": "timeout",
  "exit_code": null,
  "output": "partial output...",
  "duration_ms": 30000,
  "truncated": false
}
```

### 输出：失败

```json
{
  "session_id": "tmux:%1",
  "command": "unknown-command",
  "status": "completed",
  "exit_code": 127,
  "output": "zsh: command not found: unknown-command",
  "duration_ms": 120,
  "truncated": false
}
```

### status 枚举

| status      | 说明                  |
| ----------- | ------------------- |
| `completed` | 命令已结束               |
| `timeout`   | 等待超时                |
| `error`     | terminal-mcp 自身执行失败 |

### 行为要求

1. 命令必须在目标 session 当前 shell 环境中执行。
2. 命令执行目录由目标 session 当前目录决定。
3. 不支持单独传入 working directory。
4. 不支持单独传入环境变量。
5. 不支持交互式输入。
6. 不支持 stdin 流式传输。
7. 命令结束后必须返回 exit code。
8. 命令超时时必须返回已捕获的部分输出。
9. 命令超时后不强制杀死命令。
10. 如果输出超过最大返回限制，允许截断。
11. 截断时必须返回 `truncated: true`。
12. 如果 session 不存在，返回错误。
13. 如果目标 session 当前不是普通 shell prompt，结果可以返回 `timeout` 或 `error`。

---

## 6.5 terminal_interrupt

### 目的

向指定 terminal session 发送中断信号。

### 输入

```json
{
  "session_id": "tmux:%1"
}
```

### 参数要求

| 参数           | 必填 | 说明                  |
| ------------ | -: | ------------------- |
| `session_id` |  是 | terminal session ID |

### 输出

```json
{
  "session_id": "tmux:%1",
  "interrupted": true
}
```

### 行为要求

1. 对目标 session 发送一次中断操作。
2. 不等待目标进程完全退出。
3. 不返回目标进程退出码。
4. 如果 session 不存在，返回错误。

---

## 7. 错误模型

所有 tool 在失败时返回结构化错误。

### 错误输出格式

```json
{
  "error": {
    "code": "SESSION_NOT_FOUND",
    "message": "session not found: tmux:%99"
  }
}
```

### 错误码

| 错误码                   | 说明             |
| --------------------- | -------------- |
| `SESSION_NOT_FOUND`   | 指定 session 不存在 |
| `INVALID_ARGUMENT`    | 参数非法           |
| `BACKEND_UNAVAILABLE` | tmux 不可用       |
| `BACKEND_ERROR`       | tmux 操作失败      |
| `COMMAND_TIMEOUT`     | 命令执行超时         |
| `OUTPUT_TOO_LARGE`    | 输出超过限制         |
| `INTERNAL_ERROR`      | 未分类内部错误        |

---

## 8. 输出限制

为避免 MCP 返回内容过大，v0.1 需要限制输出大小。

### 默认限制

| 项                            |    默认值 |
| ---------------------------- | -----: |
| `terminal_read_session` 最大输出 |  64 KB |
| `terminal_run_command` 最大输出  | 128 KB |
| 默认 `last_n_lines`            |    200 |
| 默认 `timeout_ms`              |  30000 |
| 最大 `timeout_ms`              | 300000 |

### 行为要求

1. 超过输出限制时，保留尾部输出。
2. 被截断的响应必须设置 `truncated: true`。
3. 未被截断的响应必须设置 `truncated: false`。

---

## 9. 审计日志

### 目的

记录所有 MCP tool 调用，方便后续排查 Agent 做过什么。

### 存储形式

审计日志使用 JSONL 文件。

每一行是一条 JSON 记录。

### 日志位置

默认路径：

```text
~/.terminal-mcp/audit.jsonl
```

允许通过环境变量覆盖：

```text
TERMINAL_MCP_AUDIT_LOG_PATH
```

### 必须记录的字段

```json
{
  "timestamp": "2026-06-30T11:30:00+08:00",
  "request_id": "req_abc123",
  "tool": "terminal_run_command",
  "session_id": "tmux:%1",
  "command": "pwd && ls -lah",
  "text_bytes": 0,
  "status": "completed",
  "exit_code": 0,
  "duration_ms": 842,
  "output_bytes": 4096,
  "truncated": false,
  "error_code": "",
  "error_message": ""
}
```

### 字段要求

| 字段              | 必填 | 说明            |
| --------------- | -: | ------------- |
| `timestamp`     |  是 | 调用时间          |
| `request_id`    |  是 | 单次 tool 调用 ID |
| `tool`          |  是 | MCP tool 名称   |
| `session_id`    |  否 | 目标 session ID |
| `command`       |  否 | 执行的命令         |
| `text_bytes`    |  否 | 发送文本字节数       |
| `status`        |  是 | 调用结果          |
| `exit_code`     |  否 | 命令退出码         |
| `duration_ms`   |  是 | 调用耗时          |
| `output_bytes`  |  否 | 返回输出字节数       |
| `truncated`     |  否 | 输出是否截断        |
| `error_code`    |  否 | 错误码           |
| `error_message` |  否 | 错误信息          |

### 行为要求

1. 每次 tool 调用必须写一条审计日志。
2. 成功、失败、超时都必须记录。
3. `terminal_send_text` 必须记录 `text_bytes`。
4. `terminal_run_command` 必须记录完整 `command`。
5. 不要求记录完整输出内容。
6. 审计日志写入失败时，不阻塞主功能。
7. 审计日志目录不存在时，应自动创建。

---

## 10. 配置项

v0.1 只支持以下配置项。

| 环境变量                              | 默认值                           | 说明                   |
| --------------------------------- | ----------------------------- | -------------------- |
| `TERMINAL_MCP_AUDIT_LOG_PATH`     | `~/.terminal-mcp/audit.jsonl` | 审计日志路径               |
| `TERMINAL_MCP_DEFAULT_TIMEOUT_MS` | `30000`                       | 默认命令超时时间             |
| `TERMINAL_MCP_MAX_TIMEOUT_MS`     | `300000`                      | 最大命令超时时间             |
| `TERMINAL_MCP_READ_MAX_BYTES`     | `65536`                       | read_session 最大返回字节数 |
| `TERMINAL_MCP_RUN_MAX_BYTES`      | `131072`                      | run_command 最大返回字节数  |

不增加配置文件。

---

## 11. 启动方式

### 命令

```bash
terminal-mcp
```

### 行为要求

1. 进程启动后作为 MCP stdio server 运行。
2. 通过 stdin/stdout 与 MCP Host 通信。
3. stderr 可以输出本地诊断日志。
4. 不启动 HTTP 服务。
5. 不监听端口。
6. 不创建后台进程。

---

## 12. MCP Host 配置示例

MCP Host 可以按如下方式配置：

```json
{
  "mcpServers": {
    "terminal": {
      "command": "/usr/local/bin/terminal-mcp",
      "args": [],
      "env": {
        "TERMINAL_MCP_AUDIT_LOG_PATH": "/Users/ycgao/.terminal-mcp/audit.jsonl"
      }
    }
  }
}
```

---

## 13. 使用场景

### 场景 1：查询当前终端会话

用户已经在 Warp 中打开 tmux。

Agent 调用：

```json
{
  "tool": "terminal_list_sessions",
  "arguments": {}
}
```

期望返回当前所有 tmux pane。

---

### 场景 2：读取某个终端输出

Agent 调用：

```json
{
  "tool": "terminal_read_session",
  "arguments": {
    "session_id": "tmux:%1",
    "last_n_lines": 100
  }
}
```

期望返回该 pane 最近 100 行输出。

---

### 场景 3：执行命令

Agent 调用：

```json
{
  "tool": "terminal_run_command",
  "arguments": {
    "session_id": "tmux:%1",
    "command": "go test ./...",
    "timeout_ms": 60000
  }
}
```

期望返回：

1. 命令输出。
2. exit code。
3. 执行耗时。
4. 是否超时。
5. 是否截断。

---

### 场景 4：发送原始输入

Agent 调用：

```json
{
  "tool": "terminal_send_text",
  "arguments": {
    "session_id": "tmux:%1",
    "text": "clear\n"
  }
}
```

期望只完成输入，不等待结果。

---

### 场景 5：中断当前任务

Agent 调用：

```json
{
  "tool": "terminal_interrupt",
  "arguments": {
    "session_id": "tmux:%1"
  }
}
```

期望向目标 pane 发送中断。

---

## 14. 验收标准

### 14.1 基础启动

1. 安装后可以通过 `terminal-mcp` 启动。
2. 启动后可以被 MCP Host 识别。
3. 启动时不要求额外配置文件。

### 14.2 list sessions

1. 本机存在 tmux pane 时，`terminal_list_sessions` 能返回 pane 列表。
2. 返回结果中每个 session 都有唯一 ID。
3. 返回结果中包含 session 名称、window index、pane index。
4. 无 tmux pane 时返回空数组或明确错误，不得 panic。

### 14.3 read session

1. 对有效 session 调用 `terminal_read_session` 可以返回最近输出。
2. `last_n_lines` 生效。
3. session 不存在时返回 `SESSION_NOT_FOUND`。
4. 输出过大时返回截断结果，并设置 `truncated: true`。

### 14.4 send text

1. 对有效 session 调用 `terminal_send_text` 可以输入文本。
2. 不自动追加换行。
3. session 不存在时返回 `SESSION_NOT_FOUND`。
4. 调用后审计日志中有记录。

### 14.5 run command

1. 对有效 shell session 调用 `terminal_run_command` 可以执行命令。
2. 命令完成后返回 `status: completed`。
3. 命令完成后返回 exit code。
4. 命令完成后返回输出。
5. 命令超时时返回 `status: timeout`。
6. 命令超时时返回已捕获的部分输出。
7. session 不存在时返回 `SESSION_NOT_FOUND`。
8. 输出过大时返回截断结果，并设置 `truncated: true`。

### 14.6 interrupt

1. 对有效 session 调用 `terminal_interrupt` 可以发送中断。
2. session 不存在时返回 `SESSION_NOT_FOUND`。
3. 调用后审计日志中有记录。

### 14.7 audit log

1. 每次 MCP tool 调用都会写入一条 JSONL 审计日志。
2. 成功调用有日志。
3. 失败调用有日志。
4. 超时调用有日志。
5. 日志中包含 tool 名称、session ID、耗时、状态。
6. `terminal_run_command` 日志中包含 command。
7. 日志目录不存在时自动创建。

---

## 15. 版本边界

v0.1 只追求可用 MVP。

只要满足以下能力，即视为 v0.1 完成：

1. 能作为 MCP Server 启动。
2. 能列出 tmux pane。
3. 能读取 tmux pane 输出。
4. 能向 tmux pane 输入文本。
5. 能在 tmux pane 中执行命令并返回输出。
6. 能中断 tmux pane。
7. 能记录审计日志。

其他能力全部延后。

