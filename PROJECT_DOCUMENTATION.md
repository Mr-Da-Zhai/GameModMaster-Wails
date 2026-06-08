# GameModMaster 项目文档

## 项目概述

**GameModMaster** 是一个基于 **Tauri 2.x + Vue 3 + TypeScript** 开发的桌面 GUI 应用，用于管理和使用"风灵月影"游戏修改器。

- **类型**: Windows 桌面应用
- **前端**: Vue 3 + TypeScript + Naive UI
- **后端**: Rust + Tauri 2.x
- **数据库**: SQLite (bundled)
- **功能**: 下载、管理、启动游戏修改器

## 项目结构

```
GameModMaster/
├── src/                          # 前端源码 (Vue 3 + TypeScript)
│   ├── App.vue                   # 根组件（窗口框架、导航、主题）
│   ├── main.ts                   # 前端入口
│   ├── views/                    # 页面视图
│   │   ├── HomeView.vue          # 首页（修改器列表）
│   │   ├── DownloadsView.vue     # 已下载修改器页面
│   │   ├── DetailView.vue        # 修改器详情页
│   │   └── SettingsView.vue      # 设置页
│   ├── stores/                   # Pinia 状态管理
│   │   └── trainer.ts            # 核心状态存储
│   ├── services/                 # 前端服务层
│   │   ├── storageService.ts     # 存储服务（调用后端 API）
│   │   └── updaterService.ts     # 更新服务
│   ├── components/               # 组件
│   │   ├── common/               # 通用组件（GameCard, TrainerCard 等）
│   │   ├── layouts/              # 布局组件
│   │   └── update/               # 更新相关组件
│   ├── router/index.ts           # Vue Router 路由配置
│   ├── types/index.ts            # TypeScript 类型定义
│   └── i18n/                     # 国际化（中/英/西/法/日）
│
├── src-tauri/                    # Rust 后端源码
│   ├── Cargo.toml                # Rust 依赖配置
│   ├── tauri.conf.json           # Tauri 配置
│   └── src/
│       ├── main.rs               # 后端入口（注册命令、初始化）
│       ├── api/                  # Tauri 命令 API 层
│       │   ├── trainer.rs        # 修改器 API（fetch/download/launch/delete）
│       │   ├── storage.rs        # 存储 API（暴露给前端）
│       │   ├── settings.rs       # 设置 API
│       │   └── ...
│       ├── services/             # 业务逻辑层
│       │   ├── trainer.rs        # 修改器核心逻辑
│       │   ├── storage.rs        # SQLite 数据库存储
│       │   ├── scraper.rs        # 网页爬虫（解析 flingtrainer.com）
│       │   ├── download_manager.rs # 下载管理
│       │   └── settings.rs       # 设置管理
│       ├── models/trainer.rs     # 数据模型
│       └── utils/                # 工具函数
│           ├── path.rs           # 路径处理
│           └── zip.rs            # ZIP 解压
│
├── package.json                  # 前端依赖
└── pnpm-lock.yaml
```

## 技术栈

### 前端
| 技术 | 版本 | 用途 |
|------|------|------|
| Vue 3 | ^3.5.13 | 前端框架 |
| TypeScript | ~5.7.3 | 类型安全 |
| Pinia | ^3.0.1 | 状态管理 |
| Vue Router | ^4.5.0 | 路由 |
| Naive UI | ^2.38.1 | UI 组件库 |
| vue-i18n | ^9.14.5 | 国际化 |
| Vite | ^6.1.0 | 构建工具 |
| axios | ^1.6.7 | HTTP 客户端 |
| cheerio | 1.0.0-rc.12 | HTML 解析 |

### 后端
| 技术 | 版本 | 用途 |
|------|------|------|
| Tauri | 2.2.4 | 跨平台桌面应用框架 |
| tokio | 1.36 | 异步运行时 |
| reqwest | 0.12 | HTTP 客户端 |
| rusqlite | 0.31 | SQLite 数据库（bundled） |
| scraper | 0.18 | HTML 解析 |
| serde/serde_json | 1.0 | JSON 序列化 |
| zip | 0.6 | ZIP 解压 |
| chrono | 0.4 | 时间处理 |
| windows-sys | 0.52 | Windows API（ShellExecute） |

## 核心功能模块

### 1. 修改器管理

**关键文件**:
- 前端: `src/stores/trainer.ts`, `src/services/storageService.ts`
- 后端: `src-tauri/src/services/trainer.rs`, `src-tauri/src/services/storage.rs`

**功能**:
- 从 flingtrainer.com 爬取修改器列表
- 下载修改器（支持 ZIP/EXE 格式）
- 安装管理（解压、存储、记录）
- 启动修改器（ShellExecute）
- 删除修改器

### 2. 数据存储

**数据库**: SQLite (`app.db`)

**表结构**:
```sql
-- 已安装修改器表
CREATE TABLE installed_trainers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    game_version TEXT NOT NULL,
    download_url TEXT NOT NULL,
    description TEXT NOT NULL,
    thumbnail TEXT NOT NULL,
    download_count INTEGER NOT NULL,
    last_update TEXT NOT NULL,
    installed_path TEXT NOT NULL,
    install_time TEXT NOT NULL,
    last_launch_time TEXT
);

-- 已下载修改器表
CREATE TABLE downloaded_trainers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    -- ... 其他字段
);

-- 修改器列表缓存
CREATE TABLE trainer_cache (
    page INTEGER PRIMARY KEY,
    data TEXT NOT NULL,
    expiration INTEGER NOT NULL
);
```

### 3. 网页爬虫

**关键文件**: `src-tauri/src/services/scraper.rs`

**功能**:
- 解析 flingtrainer.com 网页
- 提取修改器信息（名称、版本、下载链接等）
- 支持分页加载
- 提取下载链接（可能需要解密）

### 4. 下载管理

**关键文件**: `src-tauri/src/services/download_manager.rs`

**功能**:
- 异步下载文件
- 进度回调
- 重试机制
- 临时文件管理

### 5. 国际化

**关键文件**: `src/i18n/`

**支持语言**:
- 中文 (zh-CN)
- 英文 (en-US)
- 西班牙语 (es-ES)
- 法语 (fr-FR)
- 日语 (ja-JP)

## 核心流程

### 应用初始化流程

```
App.vue (onMounted)
    │
    ├─> 加载设置 (get_settings)
    │
    ├─> 初始化 Store (store.initialize)
    │   │
    │   ├─> 检查存储迁移 (migrateFromLocalStorage)
    │   │
    │   ├─> 并行加载本地数据
    │   │   ├─> getInstalledTrainers (从 SQLite)
    │   │   └─> getDownloadedTrainers (从 SQLite)
    │   │
    │   ├─> 清理过期缓存 (cleanExpiredCache)
    │   │
    │   └─> 获取远程数据 (fetchTrainers)
    │       └─> 从 flingtrainer.com 爬取数据
    │
    └─> 初始化更新监听器 (initUpdateListener)
```

### 下载修改器流程

```
download_trainer()
    │
    ├─> 获取下载目录
    │
    ├─> 创建临时目录 (staging_dir)
    │
    ├─> 下载文件（带进度）
    │   └─> download_manager::download_file_with_progress
    │
    ├─> 检测文件类型
    │   ├─> ZIP: 解压到临时目录
    │   └─> EXE: 直接使用
    │
    ├─> 原子性切换: staging_dir -> final_dir
    │
    └─> 保存到数据库
        ├─> upsert_installed_trainer
        └─> upsert_downloaded_trainer
```

### 启动修改器流程

```
launch_trainer(trainer_id)
    │
    ├─> 从数据库读取安装路径
    │   └─> get_installed_trainer_by_id
    │
    ├─> 如果数据库没有，扫描文件系统（兼容旧数据）
    │
    ├─> 查找 EXE 文件
    │   └─> 遍历目录，查找 .exe 文件
    │
    ├─> 使用 Windows ShellExecuteW 启动
    │
    └─> 更新最后启动时间
        └─> update_last_launch_time
```

## 数据流向图

```
用户操作
    │
    ▼
┌─────────────────────────────────────────────────────────┐
│                    前端 (Vue 3)                          │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │   Views     │───▶│   Stores    │───▶│  Services   │  │
│  │ (HomeView,  │    │  (trainer.ts)│   │ (storage    │  │
│  │ Downloads)  │    │             │    │  Service.ts)│  │
│  └─────────────┘    └─────────────┘    └─────────────┘  │
└─────────────────────────────────────────────────────────┘
                              │
                              │ invoke() / Tauri IPC
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    后端 (Rust/Tauri)                     │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │  API 层     │───▶│  Services   │───▶│  Storage    │  │
│  │ (storage.rs,│    │ (trainer.rs,│    │ (SQLite     │  │
│  │  trainer.rs)│    │  scraper.rs)│    │  app.db)    │  │
│  └─────────────┘    └─────────────┘    └─────────────┘  │
└─────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────┐
│                    外部资源                              │
│  • flingtrainer.com (网页爬取)                          │
│  • 下载文件服务器                                        │
│  • 本地文件系统（下载目录、trainer.json）                │
└─────────────────────────────────────────────────────────┘
```

## 开发指南

### 环境要求

- Node.js >= 18
- pnpm >= 8
- Rust >= 1.70
- Tauri CLI >= 2.0

### 安装依赖

```bash
# 安装前端依赖
pnpm install

# Rust 依赖会在首次构建时自动安装
```

### 开发模式

```bash
pnpm tauri dev
```

### 构建发布

```bash
pnpm tauri build
```

### 关键配置文件

- `tauri.conf.json`: Tauri 应用配置（窗口、权限、打包等）
- `Cargo.toml`: Rust 依赖和构建配置
- `package.json`: 前端依赖和脚本

## 性能优化建议

### 已识别的性能瓶颈

1. **网络请求阻塞本地数据显示**
   - 问题: 初始化时必须等待远程数据加载完成
   - 优化: 先显示本地数据，后台异步更新远程数据

2. **数据库连接管理**
   - 问题: 每次操作都创建新的 SQLite 连接
   - 优化: 使用数据库连接池

3. **串行初始化流程**
   - 问题: 多个操作串行执行
   - 优化: 尽可能并行化独立操作

### 优化方向

1. **分离本地和远程数据加载**
   - 本地数据立即加载显示
   - 远程数据异步更新
   - 提供加载状态反馈

2. **改进缓存策略**
   - 增加缓存时间
   - 实现增量更新
   - 后台预加载

3. **数据库优化**
   - 使用连接池
   - 添加索引
   - 批量操作

## 已知问题

1. 首次启动需要从 flingtrainer.com 爬取数据，可能较慢
2. 网站反爬机制可能导致请求失败
3. 部分下载链接可能需要解密

## 许可证

（根据项目实际情况填写）

## 贡献指南

（根据项目实际情况填写）

## 更新日志

（根据项目实际情况填写）
