# GameModMaster-Wails — 开发指南

> 游戏修改器管理工具（FLiNG 修改器），Wails v3 + Go + Vue 3

## 项目定位

从 flingtrainer.com 爬取修改器数据，本地 SQLite 持久化，提供高效的中英文搜索、多版本管理、下载/安装/启动。**离线优先**：本地 DB 是主数据源，网络仅作增量。

## 架构概览

```
flingtrainer.com ──scraper──▶ SQLite (games/trainers/states) ──▶ 内存 Index ──▶ Wails 绑定 ──▶ Vue UI
```

- **离线优先**：启动从 DB 加载全部数据到内存索引（<100ms），立即渲染，后台异步刷新。
- **写入时翻译**：爬取时查内存 name_mapping.json 一次性翻译中文名存入 DB，读取时零查询。
- **Game(1) → Trainer(N)**：一个游戏的多个修改器版本归到一组，详情页展示多版本。
- **状态归一**：trainer_states 单表只存 trainer_id + status + 路径 + 时间。

详细设计见 `docs/ARCHITECTURE.md`，任务拆分见 `docs/DEVELOPMENT_PLAN.md`。

## 目录结构

```
main.go                  入口，embed 前端 dist + name_mapping.json，创建窗口并注入 AppService
app.go                   Wails 绑定层（所有暴露给前端的方法在此）
internal/
  model/                 纯数据结构（game/trainer/state）
  repo/                  SQLite 数据访问（db 初始化 + 3 个 repo）
  service/               mapping_service（中英映射）、download_service（下载/解压/状态）
  scraper/               parser.go（HTML 解析）、scraper.go（HTTP 爬取 + 入库）
  index/                 内存索引（启动加载、搜索、刷新）
frontend/
  src/
    views/               HomeView / DownloadsView / DetailView / SettingsView
    stores/trainer.ts    Pinia store，含 refresh/download 进度事件监听
    components/          MainLayout
    router/              路由配置
data/name_mapping.json  中英文名称映射表（embed 进二进制）
```

## 开发命令

```bash
# 前端
cd frontend && pnpm install
npx vue-tsc --noEmit          # 类型检查
npx vite build                # 前端生产构建（输出 frontend/dist）

# 后端
go build ./...                # 编译全部 Go 包
go test ./internal/...        # 运行测试（scraper parser 有单测）
go build -o gamm.exe .        # 构建完整应用（embed 前端 + 数据）

# Wails 开发/构建
wails3 dev                    # 开发模式（热重载）
wails3 generate bindings      # app.go 改动后重新生成前端 bindings（frontend/bindings/）
wails3 build                  # 生产构建

# 端到端爬取验证（临时脚本，勿提交）
go run tmp_e2e/main.go        # 真实抓取 1 页，打印入库结果
```

## 关键约定

### 后端绑定方法（app.go）

所有暴露给前端的方法返回 `(value, error)` 或 `value`。新增方法后必须 `wails3 generate bindings` 重新生成 `frontend/bindings/`。

**异步任务模式**：长耗时操作（RefreshData、DownloadTrainer）通过事件推送进度：
- 后端：`appService.SetWindow(window)` 注入窗口，`window.EmitEvent(name, data...)` 广播。
- 前端：`import { Events } from '@wailsio/runtime'`，`Events.On(name, cb)` 监听。
- 事件名：`refresh:progress`（{page,total,current,games,trainers,done,summary}）、`download:progress`（{trainer_id,downloaded,total,speed,done}）。

### 爬虫（scraper）

**解析器必须匹配真实 HTML**。FLiNG 页面结构：
- 列表页：`article.post-standard` → `h2.post-title a`（标题+链接）；封面取 `img.wp-post-image`（`.post-details-thumb img` 常为空 src）。
- 详情页：`.entry > p` 首段摘要 `"<N> Options · Game Version: <X> · Last Updated: <YYYY.MM.DD>"`；多版本表格 `.download-attachments tr`，每行 td：`[icon, 文件名+下载链接, 日期, 大小, 下载数]`。
- 下载链接形如 `https://flingtrainer.com/downloads/<TOKEN>,,`。

修改解析器后跑 `go test ./internal/scraper/`。改动选择器前建议先抓真实 HTML 验证（参考历史上的 tmp_e2e 脚本）。

### 数据模型

- `Game`：source_id（slug，唯一）、name_en、name_local（中文，写入时翻译）、cover_url、options_num。
- `Trainer`：source_hash（下载 URL 的 FNV 哈希，唯一去重键）、game_id、version、game_version、download_url、file_size、download_count。
- `TrainerState`：trainer_id（主键）、status（0=可用 1=已下载 2=已安装）、local_path、时间戳。
- `kv_store`：键值配置（download_dir、mapping_count 等）。

### 前端

- Naive UI 组件库，暗色主题（背景 #1b2636，侧边栏 #162130，强调色 #63e2b7）。
- 表格视图为主，`NDataTable` 开启 `virtual-scroll` 支持 1000+ 条。
- store 的 `downloadProgress[trainerId]` 和 `refreshProgress` 驱动进度 UI。
- 数据下载路径由 `SetDownloadDir` 配置，默认 `<dataDir>/downloads`。

## 提交规范

`feat:` 新功能、`fix:` 修复、`chore:` 构建/配置、`docs:` 文档、`refactor:` 重构。最小原子提交。

## 已知未完成项（按优先级）

- 卡片视图切换（架构文档承诺，当前仅表格）
- 多语言 i18n（README 宣传但未实现）
- 映射表管理 UI（查看/编辑/导入，当前只读显示数量）
- 下载安装的二次确认对话框
- 全局错误 toast（当前部分页面仅 console.error）
