# Codex Profile Manager

Codex Profile Manager 是一个基于 **Go 1.25.5 + Wails v2.11.0** 的桌面应用，用来管理多个 Codex `auth.json` 配置，并一键切换当前生效账号。主平台为 Windows，同时通过 GitHub Actions 为 macOS 与 Linux 提供构建产物。

## 功能特性

- 多个供应商 / Profile 管理
- 支持两种供应商模式：
  - 官方账号
  - API Key
- 从当前 `auth.json`、本地文件、粘贴 JSON 导入官方账号配置
- API Key 模式支持填写 `API Key` 与 `Base URL`
- 明文保存 `auth.json`，支持查看与编辑原始 JSON
- 一键切换目标 `auth.json`，使用原子替换写入
- API Key 模式切换时会同步维护 `.codex/config.toml`
- 跨 provider 切换时自动同步 Codex 历史会话：
  - 官方账号 `openai` ↔ API Key `custom`
  - 通过复制 `~/.codex/sessions/.../rollout-*.jsonl` 并重写目标 `model_provider`
  - 自动避免重复克隆同一条根会话
- 当前启用项识别与托管状态检测
- 单实例运行：重复启动时唤醒已有窗口
- 系统托盘与快速切换
- 拖拽调整供应商顺序
- 审计日志与基础诊断
- 浅色 / 深色 / 跟随系统主题

> 当前版本按需求 **不做加密存储**，也 **不保留备份功能**。

## 历史会话同步说明

Codex 的可见历史不只取决于 `auth.json`，还和 `~/.codex/sessions/` 下的 rollout 会话文件有关。  
如果只切换账号而不处理 rollout 元数据，那么同类型 provider 间切换通常还能看到历史，但在以下场景里历史可能“消失”：

- 官方账号 → API Key
- API Key → 官方账号

当前版本在检测到这类**跨 provider**切换时，会自动：

1. 扫描目标 `auth.json` 同目录下的 `sessions/`
2. 找到当前 provider 下的 rollout 会话
3. 为目标 provider 克隆一份 rollout
4. 重写克隆会话中的关键字段：
   - 新的 session id
   - `model_provider`
   - `cloned_from`
   - `original_provider`
   - `root_session_id`
   - `clone_timestamp`

这样做的目的，是让 Codex 在切到另一个 provider 后仍能识别并显示原来的历史上下文。

## 技术栈

- Go 1.25.5
- Wails v2.11.0
- React 18 + TypeScript + Vite

## 本地开发

```bash
wails dev
```

前端单独运行：

```bash
cd frontend
npm install
npm run build
```

运行测试：

```bash
go test ./...
```

构建生产版本：

```bash
wails build
```

构建产物：

- Windows：`build/bin/codex-profile-manager.exe`
- macOS：`build/bin/codex-profile-manager.app`（universal，arm64 + amd64）
- Linux：`build/bin/codex-profile-manager`

每次 push 到 `main` 分支会通过 GitHub Actions 自动跨平台编译，产物可在仓库的 **Actions → 最近一次 Build run → Artifacts** 中下载（`codex-profile-manager-windows` / `-macos` / `-linux`）。

## 项目结构

- `main.go` / `app.go`：Wails 应用入口、窗口与后端绑定
- `tray.go`：系统托盘、快速切换、退出处理
- `api_types.go`：前后端共享 DTO
- `internal/codexsession`：Codex rollout 会话扫描、跨 provider 克隆与去重
- `internal/codexcfg`：`.codex/config.toml` 受管块维护
- `internal/config`：设置读写
- `internal/profile`：Profile 导入、编辑、排序
- `internal/switcher`：切换流程与目标文件写入
- `internal/detector`：当前状态识别、诊断
- `internal/audit`：审计日志
- `frontend/src`：React UI、主题、编辑器、列表页
- `build/`：图标与打包产物

## 数据与安全说明

应用配置位于当前 Windows 用户的 AppData 目录下，`auth.json` 内容会以**明文**形式存储在应用配置文件中。API Key 模式下，切换时还会修改 `auth.json` 同目录的 `config.toml`。请勿提交真实凭据，也不要在不受信任的机器上使用生产账号数据。

## 当前定位

这是一个偏本地工具化的账号切换器，重点是：

- 快速切换
- 可视化管理
- 最小打扰的桌面体验
