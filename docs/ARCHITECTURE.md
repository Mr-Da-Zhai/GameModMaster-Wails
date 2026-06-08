# GameModMaster-Wails 架构设计

> 版本: v3.0.0 | 框架: Wails v3 + Go + Vue 3 | 基于 Tauri v2.1.0 重构

## 1. 项目定位

游戏修改器管理工具（风灵月影修改器），核心能力是**数据管理 + 查询**：
- 从 flingtrainer.com 爬取修改器数据
- 本地 SQLite 持久化
- 高效的中文搜索、分类浏览、状态管理

## 2. 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| 桌面框架 | Wails v3 (alpha.93+) | Go 后端 + WebView 前端 |
| 后端语言 | Go 1.22+ | 业务逻辑、爬虫、数据层 |
| 前端框架 | Vue 3 + TypeScript | Composition API |
| UI 组件库 | Naive UI | 与原项目保持一致 |
| 构建工具 | Vite | 前端构建 |
| 数据库 | SQLite (WAL mode) | 本地持久化 |
| HTML 解析 | goquery | 爬虫页面解析 |

## 3. 项目结构

```
GameModMaster-Wails/
├── main.go                     # 入口
├── app.go                      # Wails 绑定层（暴露给前端的方法）
├── internal/
│   ├── model/                  # 数据模型（纯结构体）
│   │   ├── game.go
│   │   ├── trainer.go
│   │   └── state.go
│   ├── repo/                   # 数据访问层（SQLite 操作）
│   │   ├── db.go               # 初始化、迁移
│   │   ├── game_repo.go
│   │   ├── trainer_repo.go
│   │   └── state_repo.go
│   ├── service/                # 业务逻辑层
│   │   ├── trainer_service.go  # 修改器 CRUD + 爬虫调度
│   │   ├── download_service.go # 下载管理（进度、取消）
│   │   └── mapping_service.go  # 中文名映射（内存索引）
│   ├── scraper/                # 爬虫模块
│   │   ├── scraper.go          # flingtrainer.com 爬取
│   │   └── parser.go           # HTML 解析
│   └── index/                  # 内存索引
│       └── index.go            # 热数据索引（启动时构建）
├── frontend/
│   ├── src/
│   │   ├── views/              # 页面组件
│   │   ├── components/         # 通用组件
│   │   ├── composables/        # 组合式函数
│   │   ├── stores/             # Pinia 状态管理
│   │   ├── types/              # TypeScript 类型
│   │   └── router/
│   ├── index.html
│   └── vite.config.ts
├── data/
│   └── name_mapping.json       # 外部中英映射表
├── docs/                       # 文档
└── build/                      # 构建配置
```

## 4. 数据模型

### 4.1 SQLite Schema

```sql
-- 游戏：每个游戏一行
CREATE TABLE games (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id   TEXT    NOT NULL UNIQUE,
    name_en     TEXT    NOT NULL,
    name_local  TEXT    DEFAULT '',
    cover_url   TEXT    DEFAULT '',
    source_url  TEXT    DEFAULT '',
    options_num INTEGER DEFAULT 0,
    updated_at  INTEGER NOT NULL
);
CREATE INDEX idx_games_name_en    ON games(name_en);
CREATE INDEX idx_games_name_local ON games(name_local);
CREATE INDEX idx_games_updated    ON games(updated_at DESC);

-- 修改器：每个版本一行
CREATE TABLE trainers (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id        INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    version        TEXT    DEFAULT '',
    game_version   TEXT    DEFAULT '',
    download_url   TEXT    DEFAULT '',
    file_size      INTEGER DEFAULT 0,
    file_name      TEXT    DEFAULT '',
    download_count INTEGER DEFAULT 0,
    source_hash    TEXT    NOT NULL UNIQUE,
    updated_at     INTEGER NOT NULL
);
CREATE INDEX idx_trainers_game_id ON trainers(game_id);

-- 状态：每个修改器最多一行
CREATE TABLE trainer_states (
    trainer_id   INTEGER PRIMARY KEY REFERENCES trainers(id) ON DELETE CASCADE,
    status       INTEGER NOT NULL DEFAULT 0,
    local_path   TEXT    DEFAULT '',
    installed_at INTEGER DEFAULT 0,
    launched_at  INTEGER DEFAULT 0
);

-- 中文名映射（从 JSON 加载到内存，仅爬取时查询）
CREATE TABLE name_mapping (
    name_en  TEXT PRIMARY KEY,
    name_zh  TEXT NOT NULL,
    aliases  TEXT DEFAULT '[]'
);

-- 键值配置
CREATE TABLE kv_store (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
```

### 4.2 Go 结构体

```go
type Game struct {
    ID         int32
    SourceID   string
    NameEN     string
    NameLocal  string  // 写入时已解析
    CoverURL   string
    SourceURL  string
    OptionsNum int16
    UpdatedAt  int64
}

type Trainer struct {
    ID            int32
    GameID        int32
    Version       string
    GameVersion   string
    DownloadURL   string
    FileSize      int32
    FileName      string
    DownloadCount int32
    SourceHash    string
    UpdatedAt     int64
}

type TrainerStatus uint8  // 0=可用 1=已下载 2=已安装

type TrainerState struct {
    TrainerID   int32
    Status      TrainerStatus
    LocalPath   string
    InstalledAt int64
    LaunchedAt  int64
}
```

### 4.3 内存索引

```go
type Index struct {
    gamesByID        map[int32]*Game
    trainersByID     map[int32]*Trainer
    statesByID       map[int32]*TrainerState
    gamesByNameEN    map[string]*Game    // lowercase key
    gamesByNameLocal map[string]*Game
    trainersByGame   map[int32][]*Trainer
    gamesByUpdated   []*Game             // 按更新时间排序
    nameMapping      map[string]string   // en→zh
    aliasIndex       map[string]string   // alias→zh
}
```

## 5. 核心设计决策

### 5.1 离线优先

```
启动 → SQLite 加载全部 games/trainers/states 到内存 (<100ms)
     → 立即渲染首页
     → 后台异步爬取最新数据
     → 增量更新 → 刷新 UI
```

- 本地 SQLite 是主数据源，网络是增量来源
- 首次安装才需要网络，之后秒开
- 爬取结果持久化到 DB + 内存索引

### 5.2 写入时翻译

```
旧: 爬取 → 存英文名 → 每次读取时查 DB 翻译 (N+1 查询)
新: 爬取 → 查内存映射表 → 中文名和英文名一起存入 DB → 读取时直出
```

- name_mapping.json 启动时加载到 Go map
- 爬取新游戏时一次性查找映射
- 显示时零 DB 查询

### 5.3 Game/Trainer 分离

```
旧: 每个 trainer 是独立条目，同一游戏的多个版本无关联
新: Game (1) → Trainers (N)，一个游戏的所有版本归组
```

- 同游戏多版本修改器在 UI 上归到一组
- 查询更高效：先找游戏，再取该游戏下所有修改器

### 5.4 状态归一

```
旧: installed_trainers 表 + downloaded_trainers 表 = 完整复制 trainer 全部字段
新: trainer_states 一张表，只存 trainer_id + status + 路径 + 时间
```

- 不重复存储 trainer 数据
- 状态变更只更新一行

## 6. UI 设计方向

### 6.1 默认视图：紧凑列表/表格

```
| 缩略图 | 游戏名称        | 修改器版本 | 游戏版本 | 选项数 | 状态   | 操作      |
|--------|----------------|-----------|---------|-------|--------|----------|
| [img]  | 艾尔登法环      | v1.0      | 1.09    | 15    | 已安装  | 启动 删除 |
| [img]  | 赛博朋克2077   | v2.3      | 2.1     | 20    | 已下载  | 安装 删除 |
| [img]  | 荒野大镖客2    | v1.5      | 1.0.12  | 12    | 可用    | 下载      |
```

- 默认表格视图，信息密度高
- 可选切换到卡片视图
- 虚拟滚动支持 1000+ 条目
- 支持排序（名称/更新时间/状态）和筛选

### 6.2 页面结构

- **首页**: 全部修改器表格 + 搜索 + 排序筛选
- **已下载**: 已下载/已安装的修改器，按状态分组
- **详情页**: 点击游戏展开，显示该游戏所有版本的修改器
- **设置页**: 下载路径、语言、映射表管理

## 7. 相比 Tauri 版本的改进

| 维度 | Tauri v2.1.0 (旧) | Wails v3.0.0 (新) |
|------|-------------------|-------------------|
| 后端语言 | Rust | Go（更易维护） |
| 启动速度 | 1-3s (网络依赖) | <200ms (离线优先) |
| 中文名映射 | 300条硬编码 + N+1查询 | 外部JSON + 内存索引 + 批量 |
| 数据模型 | 平面冗余 | Game/Trainer/State 归一化 |
| 缓存 | 三套独立缓存 + JSON blob | 内存索引，无需额外缓存 |
| 下载管理 | 假取消 | context.Cancel 真取消 |
| UI | 卡片为主，低密度 | 表格为主，高密度 |
| 虚拟滚动 | 有组件但首页没用 | 默认启用 |
