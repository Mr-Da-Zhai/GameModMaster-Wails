# 开发计划

> GameModMaster-Wails v3.0.0 开发任务拆分

## 阶段概览

```
Phase 0: 项目初始化        ── 清理旧代码，创建 Wails v3 项目骨架
Phase 1: 数据层            ── 模型、SQLite、Repository
Phase 2: 后端服务          ── 爬虫、映射、下载管理
Phase 3: Wails 绑定层      ── app.go，前后端桥接
Phase 4: 前端页面          ── 布局、首页、下载页、详情页、设置页
Phase 5: 集成联调          ── 端到端打通，修 bug
```

---

## Phase 0: 项目初始化

**任务 0.1: 清理旧 Tauri 代码，初始化 Wails 项目**
- 删除 src-tauri/、src/（旧前端）、旧配置文件
- 用 `wails3 init` 创建新的 Wails v3 + Vue + TS 项目
- 保留 .git 历史、LICENSE、docs/
- 更新 README.md（致谢原作者 + 新项目说明）
- 更新 CLAUDE.md（新项目开发指南）
- 提交: `chore: initialize Wails v3 project`

**任务 0.2: 创建 Go 项目结构**
- 创建 internal/ 目录结构（model/、repo/、service/、scraper/、index/）
- 创建 data/ 目录，放入 name_mapping.json
- 确保 `go mod tidy` 通过
- 提交: `chore: setup Go project structure`

---

## Phase 1: 数据层

**任务 1.1: 数据模型 (model/)**
- game.go、trainer.go、state.go
- 纯 Go 结构体，含 JSON 和 DB tag
- 提交: `feat: add data models`

**任务 1.2: SQLite 初始化 + Schema (repo/db.go)**
- 数据库初始化、Schema 创建（全部 5 张表）
- WAL mode、连接池配置
- 提交: `feat: add SQLite schema and initialization`

**任务 1.3: Game Repository (repo/game_repo.go)**
- CRUD: Insert/GetByID/GetAll/Search/BatchUpsert
- 批量插入用事务
- 提交: `feat: add game repository`

**任务 1.4: Trainer Repository (repo/trainer_repo.go)**
- CRUD + GetByGameID + BatchUpsert
- 提交: `feat: add trainer repository`

**任务 1.5: State Repository (repo/state_repo.go)**
- Upsert/GetByTrainerID/GetByStatus/ListAll
- 提交: `feat: add trainer state repository`

**任务 1.6: 内存索引 (index/index.go)**
- 启动时从 DB 加载全部数据构建索引
- 全文搜索方法（英文名/中文名/别名）
- 热重载方法（增量更新后刷新索引）
- 提交: `feat: add in-memory index`

---

## Phase 2: 后端服务

**任务 2.1: 中文名映射服务 (service/mapping_service.go)**
- 从 data/name_mapping.json 加载到内存
- 启动时构建 nameMapping + aliasIndex
- 查询方法: GetChineseName(en) / TranslateBatch(names)
- 提交: `feat: add name mapping service`

**任务 2.2: HTML 解析器 (scraper/parser.go)**
- 从旧 Rust scraper 移植解析逻辑到 Go + goquery
- parseTrainerList(html) → []Game
- parseTrainerDetail(html) → Trainer 详情
- 提交: `feat: add HTML parser`

**任务 2.3: 爬虫服务 (scraper/scraper.go)**
- HTTP 客户端（连接池、超时、User-Agent）
- 分页爬取 flingtrainer.com
- 搜索爬取
- 结果写入 DB（调 repo 层）
- 增量更新（只处理新增/变更的）
- 提交: `feat: add scraper service`

**任务 2.4: 下载服务 (service/download_service.go)**
- 流式下载 + 进度回调
- context.Cancel 真取消
- ZIP/EXE 自动识别处理
- 安装到本地目录
- 状态同步到 trainer_states 表
- 提交: `feat: add download service`

---

## Phase 3: Wails 绑定层

**任务 3.1: App 绑定 (app.go)**
- Wails 绑定方法，暴露给前端:
  - GetTrainers(page, pageSize) — 首页列表
  - SearchTrainers(query) — 搜索
  - GetTrainerDetail(gameID) — 详情
  - DownloadTrainer(trainerID) — 下载
  - InstallTrainer(trainerID) — 安装
  - LaunchTrainer(trainerID) — 启动
  - DeleteTrainer(trainerID) — 删除
  - GetDownloadedTrainers() — 已下载列表
  - GetInstalledTrainers() — 已安装列表
  - RefreshData() — 强制刷新
  - GetSettings() / SaveSettings()
- 提交: `feat: add Wails bindings`

**任务 3.2: 前端绑定生成**
- 运行 `wails3 generate module` 生成 TS 绑定
- 确保类型安全的前端调用
- 提交: `chore: generate frontend bindings`

---

## Phase 4: 前端页面

**任务 4.1: 基础布局 + 路由**
- MainLayout（侧边导航 + 内容区）
- Vue Router 配置（/、/downloads、/detail/:id、/settings）
- 全局样式（主题变量、紧凑模式基础）
- 提交: `feat: add layout and router`

**任务 4.2: 首页 — 修改器表格**
- NDataTable 虚拟滚动表格
- 列: 缩略图 | 游戏名 | 修改器版本 | 游戏版本 | 选项数 | 状态 | 操作
- 搜索框（防抖）
- 排序（更新时间/名称/状态）
- 分页或无限滚动
- 提交: `feat: add home page with table view`

**任务 4.3: 已下载页面**
- 按状态分组（已安装 / 已下载未安装）
- 操作: 启动、安装、删除、打开文件夹
- 提交: `feat: add downloads page`

**任务 4.4: 详情页**
- 游戏信息 + 该游戏所有版本的修改器列表
- 每个版本可独立下载/安装
- 提交: `feat: add detail page`

**任务 4.5: 设置页面**
- 下载路径配置
- 语言切换
- 映射表管理（查看/编辑/导入）
- 关于信息
- 提交: `feat: add settings page`

---

## Phase 5: 集成联调

**任务 5.1: 启动流程联调**
- 验证: 启动 → 数据加载 → 首页渲染全流程
- 确保 <500ms 显示内容
- 提交: `fix: startup flow integration`

**任务 5.2: 核心功能端到端**
- 搜索 → 下载 → 安装 → 启动 → 删除 全链路
- 提交: `fix: e2e functionality`

**任务 5.3: README + 发布准备**
- README 完善（功能介绍、截图、安装说明）
- 致谢原作者
- 提交: `docs: update README for v3.0.0`

---

## 提交规范

- `feat:` 新功能
- `fix:` 修复
- `chore:` 构建/配置/清理
- `docs:` 文档
- `refactor:` 重构
- 每个任务一个提交，最小化原子提交
