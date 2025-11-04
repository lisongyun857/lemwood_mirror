# 启动器自动镜像程序

本项目实现自动从 GitHub 获取指定启动器（fcl、zl、zl2）的最新 release，并将资产文件下载到本地存储结构，同时提供一个简单的黑白风格前端页面展示版本信息与下载链接，并具备基本文件浏览功能。

## 功能概述
- 通过浏览器模拟（colly）获取启动器的 GitHub 仓库地址。
- 使用 GitHub API（go-github v50）获取最新 release（仅最新，不取历史）。
- 每 10 分钟自动检查更新（可通过配置调整）。
- 下载 release 资产到 `download/启动器名/版本号/`，并生成 `info.json`。
- 提供 HTTP 服务：
  - `GET /` 前端页面。
  - `GET /api/status` 返回各启动器版本信息。
  - `POST /api/scan` 触发一次手动扫描。
  - `GET /api/files?path=...` 列出存储目录树。
  - `GET /download/...` 提供下载静态文件。

## 目录结构
- `cmd/mirror`：主程序入口。
- `internal/...`：配置、浏览器模拟、GitHub 交互、下载、存储、HTTP 服务。
- `web/static`：前端 HTML/CSS/JS。
- `download`：下载文件根目录（默认）。
- `.trae/rules/project_rules.md`：项目规则与需求汇总。

## 配置

通过修改 `config.json` 文件来自定义程序的行为。

- `github_token`: 你的 GitHub Personal Access Token，用于提高 API 请求速率限制。
- `storage_path`: 下载文件的存储目录，默认为 `download`。
- `check_cron`: 自动检查更新的 cron 表达式，默认为每 10 分钟检查一次 (`*/10 * * * *`)。
- `proxy_url`: 用于网络请求的 HTTP/HTTPS 代理地址，例如 `http://127.0.0.1:7890`。
- `asset_proxy_url`: 用于加速 GitHub Release 资源下载的代理地址，会作为前缀拼接到下载链接前。
- `xget_domain`: Xget 服务域名，用于加速 GitHub 仓库的访问和下载。
- `xget_enabled`: 是否启用 Xget 加速，`true` 或 `false`。
- `download_timeout_minutes`: 下载单个文件的超时时间（分钟），默认为 40。
- `launchers`: 要镜像的启动器列表。
  - `name`: 启动器名称。
  - `source_url`: 包含 GitHub 仓库链接的官方页面地址。
  - `repo_selector`: 用于从页面中提取 GitHub 仓库链接的 CSS 选择器。

## 构建与运行

由于当前环境未安装 Go 工具链，请先安装 Go（>=1.22）：
- Windows：访问 https://go.dev/dl/ 下载并安装，确保 `go version` 可用。
- Linux：使用系统包管理器或从官方 tarball 安装。

安装依赖并构建：
```powershell
# 在项目根目录执行
$env:GOPROXY = "https://proxy.golang.org,direct"
go mod download
go build -o .\mirror.exe .\cmd\mirror
```
运行（Windows）：
```powershell
# 可选：设置 GitHub Token
$env:GITHUB_TOKEN = "<your token>"
# 启动服务
./mirror.exe
# 访问 http://localhost:8080
```
运行（Linux）：
```bash
export GITHUB_TOKEN="<your token>"
go build -o ./mirror ./cmd/mirror
./mirror
# 访问 http://localhost:8080
```

## 使用说明
- 前端首页显示各启动器最新版本信息与下载链接。
- 点击“手动刷新”将触发一次扫描更新。
- 文件浏览可输入相对路径（例如 `.`、`fcl/`、`fcl/v1.2.3/`）查看结构。

## 认证与限流
- 建议在配置或环境变量中提供 `GITHUB_TOKEN`，提升 API 配额。
- 代码在遇到 403/配额耗尽时会按照响应的重置时间进行退避等待（有限）。

## 并发安全与资源清理
- 下载采用原子写入（.partial -> rename）。
- 使用上下文超时控制网络请求。
- 在内存状态更新和索引维护处使用锁保证并发安全。

## 注意事项
- 请将下载目录配置到 E盘（默认 `download` 在项目根），避免占用 C盘空间。
- 如果 `source_url` 不是 GitHub 仓库 URL，请提供可解析到目标仓库链接的来源页面以及相应的 `repo_selector` 或正则匹配。
