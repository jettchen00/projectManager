# projectManager

项目管理后台（Go + MongoDB）+ 配套 Web 前端。

## 目录结构

```
projectManager/
├── cmd/server/                     程序入口
├── internal/
│   ├── config/                     配置加载（`etc/config.yaml` + 环境变量覆盖）
│   ├── errcode/                    错误码 + 统一响应
│   ├── log/                        日志封装
│   ├── router/                     极简 HTTP 路由
│   ├── middleware/                 鉴权 / CORS / panic 恢复
│   ├── store/mongo/                MongoDB 客户端
│   ├── http/                       HTTP 控制器
│   └── modules/
│       ├── owner/                  业主单位
│       ├── formtemplate/           表格模板
│       ├── formvalue/              表格值（含 ChangeLog 编排）
│       ├── changelog/              修改记录 ★
│       ├── snapshot/               提交定稿快照
│       ├── approval/               审核 / 审批
│       └── project/                项目主体（状态机）
├── docs/
│   ├── workflow.md
│   ├── requirement.md
│   ├── design.md
│   ├── http_api.md
│   └── rule.md
└── web/                            原生 HTML/JS 前端
    ├── index.html
    ├── styles.css
    └── app.js
```

## 配置

核心运行参数集中在 `etc/config.yaml`，进程启动时优先加载该文件；可通过环境变量
`CONFIG_FILE` 指定其它路径。敏感/部署相关字段允许通过环境变量覆盖（优先级最高）。

```yaml
# etc/config.yaml 示例
http_addr: ":8080"
web_dir: "web"
shutdown_timeout_seconds: 5
mongo:
  uri: "mongodb://127.0.0.1:27017"
  db: "project_manager"
  connect_timeout_seconds: 10
  ping_timeout_seconds: 5
```

### 环境变量覆盖

| Key           | 覆盖项                       | 说明 |
|---------------|------------------------------|------|
| `CONFIG_FILE` | 配置文件路径                 | 默认 `etc/config.yaml` |
| `HTTP_ADDR`   | `http_addr`                  | 监听地址 |
| `MONGO_URI`   | `mongo.uri`                  | MongoDB 连接串 |
| `MONGO_DB`    | `mongo.db`                   | 数据库名 |
| `WEB_DIR`     | `web_dir`                    | 前端静态资源目录 |

## 启动

```bash
# 1) 依赖（需要联网）
go mod tidy

# 2) 启动 MongoDB（本地或远端均可）
#    Windows 上推荐 docker：
#    docker run -d -p 27017:27017 --name mongo mongo:7

# 3) 运行服务
go run ./cmd/server

# 4) 浏览器访问
http://127.0.0.1:8080/
```

页面右上角填写 `用户ID / 角色` 即可作为该身份调用接口（演示用，实际请接入 SSO）。

## 测试

```bash
go test ./...
```

测试用例覆盖：立项最小信息、状态机、共享 revision、值未变化跳过、必填校验、驳回返工 phase 等核心流程，使用 in-memory repo 不依赖真实 MongoDB。

## 文档

- 需求：`docs/requirement.md`
- 设计：`docs/design.md`
- 接口：`docs/http_api.md`
- 工作流：`docs/workflow.md`
- 规则：`docs/rule.md`
