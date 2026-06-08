# v2.1.0 Release Notes

## 🎮 新功能：游戏中文名称映射系统

### 主要更新

#### 🌏 游戏名称中英文自动转换
- **225+ 款游戏**的中英文名称映射数据库
- 自动将英文游戏名转换为中文显示
- 支持多级智能匹配：
  - 精确匹配（Stellar Blade → 剑星）
  - 大小写不敏感匹配
  - 部分匹配（支持游戏名变体）
  - 别名匹配（支持常见缩写和昵称）

#### 🔍 中文搜索支持
- 现在可以使用中文搜索游戏
- 例如：搜索"悟空"可找到"黑神话：悟空"
- 搜索"帕鲁"可找到"幻兽帕鲁"
- 搜索"剑星"可找到"Stellar Blade"

#### 🎯 2024-2025 热门游戏支持
新增以下热门游戏的中文映射：
- **剑星** (Stellar Blade)
- **黑神话：悟空** (Black Myth: Wukong)
- **幻兽帕鲁** (Palworld)
- **龙之信条2** (Dragon's Dogma 2)
- **最终幻想7重生** (Final Fantasy VII Rebirth)
- **铁拳8** (Tekken 8)
- **绝地潜兵2** (Helldivers 2)
- **如龙8** (Like a Dragon: Infinite Wealth)
- **GTA6** (Grand Theft Auto VI)
- **艾尔登法环：黄金树之影** (Elden Ring: Shadow of the Erdtree)

以及 200+ 款经典游戏和独立游戏。

### 技术改进

#### 🗄️ 数据库优化
- 新增 `game_name_mapping` 数据库表
- 修复 UTF-8 编码问题，确保中文正确显示
- 使用 WAL 模式提升数据库性能

#### 🔧 后端优化
- 下载修改器时自动转换为中文名称
- 已安装和已下载列表自动翻译
- 优化名称匹配算法性能

### 修复问题

- ✅ 修复 2024-2025 热门游戏无法显示中文名称的问题
- ✅ 修复数据库编码导致的中文乱码问题
- ✅ 修复中文搜索无法找到游戏的问题

### 升级说明

**推荐完全卸载旧版本后重新安装**，以确保数据库使用正确的 UTF-8 编码。

### 下载

- Windows x64: `GameMod Master_2.1.0_x64-setup.exe`
- 便携版: `GameMod Master_2.1.0_x64_en-US.msi`

---

**完整更新日志**: https://gitee.com/x_z/GameModMaster/compare/v2.0.0...v2.1.0
