# AGENTS.md

本仓库的 AI 编码代理（Claude Code、Codex 等）工作规则。项目的整体说明见
[README.md](README.md)。

## 仓库结构

- `src/sugar_pet/`：Python 包（`uv run pytest`、`uv run ruff check .`）
- `app/`：桌面 app（Go，MyGo 原生 UI），配置在 `app/mygo.json`
- `docs/`：VitePress 用户指南（中英文），截图在 `docs/public/screenshots/`

## 规则

### 1. 桌面 app 使用 MyGo 框架

- `app/` 下的桌面 app 用 [MyGo](https://mygo.run/docs) 开发，界面是 MyGo 的原生 UI（`github.com/egoist/mygo/ui`），不用 webview。
- 写或改界面代码前，先查 MyGo 文档，不要凭记忆猜 API：
  - 文档首页：https://mygo.run/docs
  - 原生 UI 组件：https://mygo.run/docs/ui
  - 命令行：https://mygo.run/docs/cli
  - 配置（`mygo.json`）：https://mygo.run/docs/configuration
  - 打包发布：https://mygo.run/docs/distribution
  - Go API：https://pkg.go.dev/github.com/egoist/mygo
- MyGo 的版本以 `app/go.mod` 为准；文档与该版本不一致时，以本地模块源码（`go doc github.com/egoist/mygo/ui`）为准。
- MyGo 命令都通过 `go tool mygo` 在 `app/` 目录下运行。

### 2. 修改代码后自动提交

- 每完成一项修改，确认测试通过后直接 `git commit`，不必先问我。
- 直接提交到 `main`，不要新建分支；不要 `git push`，除非我明确要求。
- 提交信息沿用仓库现有风格：英文、一句完整的祈使句，说明这次改动做了什么（例如 `Make the individual analysis three steps that hand their files on`）。

### 3. 桌面 app 改完后编译并运行，供我测试

涉及 `app/` 的修改，改好并提交后：

```bash
cd app
go vet ./... && go test ./...
go tool mygo build -platform darwin/arm64 -skip-dmg
osascript -e 'quit app "SUGAR-PET"' 2>/dev/null
open build/darwin-arm64/SUGAR-PET.app
```

- 编译失败或测试不通过时先修好，不要把跑不起来的版本交给我。
- 打开 app 后告诉我改了哪些地方、该去哪个页面看、怎么操作来验证。

### 4. 不要主动更新文档截图

- 修改桌面 app 后，**不要**急着重新生成截图（`docs/public/screenshots/`、`app/screenshots/`，即 `docs/scripts/screenshots.sh` 或 `--snapshot`），除非我主动提出。
- 界面改动可能还会反复调整；如果截图已明显过时，在回复里提一句即可。
