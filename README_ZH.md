# Xentry

自托管式错误监控与性能分析平台，类似 Sentry。专为集成 Crashpad 的桌面和移动端应用设计。

## 功能

- **崩溃监控** — 接收 Crashpad minidump 和 JSON 事件，自动按堆栈指纹分组聚合
- **符号化** — 支持所有平台（Linux/macOS/Windows）的 Breakpad .sym 符号文件，通过 dump_syms 生成 .sym 文件，使用 minidump-stackwalk 解析崩溃堆栈
- **性能监控 (APM)** — 接收 trace/span 数据，事务列表、瀑布图、延迟统计
- **日志管理** — 结构化日志采集，SQLite FTS5 全文搜索，日志级别过滤
- **Release 管理** — 版本跟踪，per-release 崩溃/事件统计
- **多项目支持** — 组织 → 项目 → 环境 多级管理，独立 DSN 和 API Token

## 技术栈

Go 1.22+ / Chi v5 / html/template + HTMX / SQLite (modernc.org/sqlite) / JWT / bcrypt / Tailwind CSS (CDN)

无 CGO 依赖，单二进制部署。

## 快速开始

### 二进制运行

```bash
# 编译
go build -o xentry ./cmd/xentry

# 启动 (默认监听 :8080)
./xentry

# 或指定配置
XENTRY_ADDR=:9090 XENTRY_JWT_SECRET=my-secret ./xentry
```

### Docker

```bash
docker compose up -d
# 访问 http://localhost:8080
```

## 环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `XENTRY_ADDR` | `:8080` | 监听地址 |
| `XENTRY_DB` | `./xentry.db` | SQLite 数据库路径 |
| `XENTRY_JWT_SECRET` | `change-me-in-production` | JWT 签名密钥 |
| `XENTRY_ENV` | `development` | 运行环境 |
| `XENTRY_DATA_DIR` | `./data` | 符号文件等数据存储目录 |

## 使用指南

### 1. 注册与登录

打开 `http://localhost:8080/login` 注册一个账户，然后用注册的邮箱和密码登录。

### 2. 创建组织和项目

登录后通过 API 创建组织（需要 JWT Token，从登录响应中获取）：

```bash
TOKEN="你的JWT_TOKEN"

# 创建组织
curl -X POST http://localhost:8080/api/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Team","slug":"my-team"}'

# 在组织下创建项目
curl -X POST http://localhost:8080/api/organizations/{orgID}/projects \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My App","slug":"my-app","platform":"windows"}'
```

响应中包含 `dsn_token`，客户端使用此 token 上报数据。

### 3. 上报崩溃事件

使用项目的 `dsn_token` 通过 `X-Xentry-DSN` 请求头上报：

```bash
DSN="项目的DSN_TOKEN"

curl -X POST http://localhost:8080/api/proj-123/events \
  -H "X-Xentry-DSN: $DSN" \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Null pointer dereference",
    "level": "fatal",
    "platform": "windows",
    "release": "1.0.0",
    "threads": [{
      "name": "main",
      "crashed": true,
      "frames": [
        {"function": "main", "file": "main.cpp", "line": 10},
        {"function": "crash_func", "file": "crash.cpp", "line": 42}
      ]
    }]
  }'
```

上传 minidump 文件：

```bash
curl -X POST http://localhost:8080/api/proj-123/crash \
  -H "X-Xentry-DSN: $DSN" \
  -F "upload_file_minidump=@crash.dmp" \
  -F "platform=windows" \
  -F "release=1.0.0"
```

相同的堆栈模式会自动聚合为同一个 Issue，重复上报只会增加计数。

### 4. 查看崩溃

```bash
# 获取项目所有 Issues
curl http://localhost:8080/api/proj-123/issues \
  -H "X-Xentry-DSN: $DSN"

# 获取单个 Issue 详情（含堆栈帧）
curl http://localhost:8080/api/proj-123/issues/{issueID} \
  -H "X-Xentry-DSN: $DSN"
```

Web 界面访问 `http://localhost:8080/projects/proj-123/issues`。

### 5. 上传符号文件

符号文件用于将内存地址解析为函数名、文件名和行号。

```bash
curl -X POST http://localhost:8080/api/proj-123/symbols \
  -H "X-Xentry-DSN: $DSN" \
  -F "file=@app.sym" \
  -F "debug_id=ABC123" \
  -F "type=breakpad" \
  -F "release=1.0.0"
```

支持的符号格式：

- `breakpad` — 由 dump_syms 生成的 Breakpad .sym 文件，通过 minidump-stackwalk 解析

上传后，新上报的崩溃事件会自动触发异步符号化。

### 6. 性能监控 (APM)

上报 trace/span 数据：

```bash
curl -X POST http://localhost:8080/api/proj-123/traces \
  -H "X-Xentry-DSN: $DSN" \
  -H "Content-Type: application/json" \
  -d '{
    "transactions": [{
      "name": "GET /api/users",
      "trace_id": "trace-abc",
      "span_id": "span-def",
      "op": "http.server",
      "status": "ok",
      "duration": 150.5
    }],
    "spans": [
      {"op": "db.query", "description": "SELECT * FROM users", "duration": 50.0},
      {"op": "http.client", "description": "GET /api/data", "duration": 80.0}
    ]
  }'
```

查询事务和统计：

```bash
# 事务列表
curl "http://localhost:8080/api/proj-123/transactions" -H "X-Xentry-DSN: $DSN"

# 统计信息（总数、平均耗时、错误数）
curl "http://localhost:8080/api/proj-123/transactions/stats" -H "X-Xentry-DSN: $DSN"

# 事务详情和 spans
curl "http://localhost:8080/api/proj-123/transactions/{txID}" -H "X-Xentry-DSN: $DSN"
curl "http://localhost:8080/api/proj-123/transactions/{txID}/spans" -H "X-Xentry-DSN: $DSN"
```

### 7. 日志管理

批量上报日志：

```bash
curl -X POST http://localhost:8080/api/proj-123/logs \
  -H "X-Xentry-DSN: $DSN" \
  -H "Content-Type: application/json" \
  -d '[{
    "level": "info",
    "message": "Server started on port 8080",
    "logger": "main",
    "trace_id": "trace-abc"
  }, {
    "level": "error",
    "message": "Connection refused",
    "logger": "db"
  }]'
```

查询和搜索：

```bash
# 查询日志（支持 level 过滤和分页）
curl "http://localhost:8080/api/proj-123/logs?level=error&limit=20" -H "X-Xentry-DSN: $DSN"

# 全文搜索
curl "http://localhost:8080/api/proj-123/logs/search?q=connection" -H "X-Xentry-DSN: $DSN"
```

### 8. Release 管理

```bash
# 创建 Release
curl -X POST http://localhost:8080/api/proj-123/releases \
  -H "X-Xentry-DSN: $DSN" \
  -H "Content-Type: application/json" \
  -d '{"version":"1.0.0","environment":"production"}'

# 列出 Releases
curl http://localhost:8080/api/proj-123/releases -H "X-Xentry-DSN: $DSN"
```

## API 参考

### 认证

```
POST /api/auth/register    注册
POST /api/auth/login       登录（返回 JWT token）
GET  /api/auth/me          当前用户信息（需 Bearer token）
```

### 组织与项目（需 Bearer token）

```
POST /api/organizations                创建组织
GET  /api/organizations                组织列表
GET  /api/organizations/{id}           组织详情
GET  /api/organizations/{id}/members   成员列表
POST /api/organizations/{orgID}/projects    创建项目
GET  /api/organizations/{orgID}/projects    项目列表
GET  /api/projects/{id}                   项目详情
POST /api/projects/{id}/tokens            创建 API Token
```

### 数据上报（需 DSN token）

```
POST /api/{projectID}/events         上报崩溃事件 (JSON)
POST /api/{projectID}/crash          上报 minidump 文件
POST /api/{projectID}/traces         上报 trace/span
POST /api/{projectID}/logs           上报日志
POST /api/{projectID}/symbols        上传符号文件
POST /api/{projectID}/releases       创建 Release
```

### 查询（需 DSN token）

```
GET /api/{projectID}/issues              Issue 列表
GET /api/{projectID}/issues/{issueID}    Issue 详情（含堆栈帧）
GET /api/{projectID}/transactions        事务列表
GET /api/{projectID}/transactions/{id}    事务详情
GET /api/{projectID}/transactions/{id}/spans  事务 spans
GET /api/{projectID}/transactions/stats  统计信息
GET /api/{projectID}/logs                日志查询
GET /api/{projectID}/logs/search         日志搜索
GET /api/{projectID}/releases            Release 列表
```

### Web 页面

```
GET /login                                    登录页
GET /register                                 注册页
GET /                                        仪表板
GET /projects/{projectID}/issues            Issue 列表
GET /projects/{projectID}/issues/{id}       Issue 详情
GET /projects/{projectID}/transactions      事务列表
GET /projects/{projectID}/transactions/{id}  事务详情（瀑布图）
GET /projects/{projectID}/logs              日志浏览器
GET /projects/{projectID}/releases          Release 列表
```

## Crashpad 客户端集成

在客户端代码中配置 Crashpad 使用 Xentry 的 DSN：

```cpp
#include "client/crashpad_client.h"

// 初始化 Crashpad，指向 Xentry 服务器
crashpad::CrashpadClient client;
client.StartCrashpad("http://your-xentry-server:8080/api/{projectID}/crash",
                    /* annotations */);
```

客户端会自动捕获崩溃并上传 minidump 到 Xentry 服务器。

## 项目结构

```
xentry/
├── cmd/xentry/main.go              # 入口
├── internal/
│   ├── config/                     # 配置加载
│   ├── db/                          # SQLite + 嵌入式 migration
│   │   └── migrations/             # SQL migration 文件 (001-006)
│   ├── auth/                        # 用户认证 (JWT + bcrypt)
│   ├── org/                         # 组织管理
│   ├── project/                     # 项目管理
│   ├── crash/                       # 崩溃上报、分组、查询
│   ├── symbol/                      # 符号文件上传、符号化引擎
│   ├── apm/                         # 性能监控 (trace/span)
│   ├── log/                         # 日志管理 + FTS5 搜索
│   ├── release/                     # Release 管理
│   ├── web/                         # HTML 模板 + HTMX
│   │   └── templates/              # 页面模板
│   └── router/router.go            # 统一路由
├── pkg/util/                        # UUID、Token 生成
├── static/                          # 静态资源
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## 开发

```bash
# 运行测试
go test ./... -v

# 代码检查
go vet ./...

# 编译
go build -o xentry ./cmd/xentry
```

## License

MIT
