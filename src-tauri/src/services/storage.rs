use std::collections::HashMap;
use std::path::PathBuf;

use anyhow::{Context, Result};
use chrono::Utc;
use once_cell::sync::Lazy;
use r2d2::{Pool, PooledConnection};
use r2d2_sqlite::SqliteConnectionManager;
use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::models::trainer::{InstalledTrainer, Trainer};
use crate::utils::path::get_app_dir;

// 存储键名常量（用于前后端约定）
pub const INSTALLED_TRAINERS_KEY: &str = "installedTrainers";
pub const DOWNLOADED_TRAINERS_KEY: &str = "downloadedTrainers";
pub const TRAINER_LIST_PREFIX: &str = "trainerList_";
pub const SEARCH_RESULTS_PREFIX: &str = "searchResults_";

// 缓存配置
const CACHE_EXPIRATION_TIME: i64 = 1000 * 60 * 15; // 15分钟
const DB_FILE_NAME: &str = "app.db";

// 数据库连接池类型
type SqlitePool = Pool<SqliteConnectionManager>;

/// 全局数据库连接池
static DB_POOL: Lazy<SqlitePool> = Lazy::new(|| {
    let db_path = get_db_path().expect("Failed to get database path");
    let manager = SqliteConnectionManager::file(db_path);
    Pool::builder()
        .max_size(10) // 连接池最大连接数
        .min_idle(Some(2)) // 最小空闲连接数
        .connection_timeout(std::time::Duration::from_secs(30))
        .build(manager)
        .expect("Failed to create database connection pool")
});

// 缓存项结构（兼容 localStorage 迁移数据）
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheItem<T> {
    pub data: T,
    pub timestamp: i64,
    pub expiration: i64,
}

fn get_db_path() -> Result<PathBuf> {
    Ok(get_app_dir()?.join(DB_FILE_NAME))
}

/// 从连接池获取数据库连接
pub fn get_conn() -> Result<PooledConnection<SqliteConnectionManager>> {
    DB_POOL
        .get()
        .context("Failed to get database connection from pool")
}

async fn with_conn<T, F>(f: F) -> Result<T>
where
    T: Send + 'static,
    F: FnOnce(&mut Connection) -> Result<T> + Send + 'static,
{
    tauri::async_runtime::spawn_blocking(move || {
        let mut conn = get_conn()?;
        f(&mut conn)
    })
    .await
    .context("数据库线程池执行失败")?
}

pub async fn init_db() -> Result<()> {
    let db_path = get_db_path()?;
    log::info!("数据库路径: {:?}", db_path);

    if let Some(parent) = db_path.parent() {
        std::fs::create_dir_all(parent)?;
    }

    // 初始化数据库结构（通过连接池）
    tauri::async_runtime::spawn_blocking(move || {
        let conn = get_conn()?;
        conn.execute_batch(
            "
            PRAGMA journal_mode=WAL;
            PRAGMA synchronous=NORMAL;
            PRAGMA encoding='UTF-8';
            CREATE TABLE IF NOT EXISTS installed_trainers (
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
            CREATE TABLE IF NOT EXISTS downloaded_trainers (
                id TEXT PRIMARY KEY,
                name TEXT NOT NULL,
                version TEXT NOT NULL,
                game_version TEXT NOT NULL,
                download_url TEXT NOT NULL,
                description TEXT NOT NULL,
                thumbnail TEXT NOT NULL,
                download_count INTEGER NOT NULL,
                last_update TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS trainer_cache (
                page INTEGER PRIMARY KEY,
                data TEXT NOT NULL,
                expiration INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS search_cache (
                query TEXT NOT NULL,
                page INTEGER NOT NULL,
                data TEXT NOT NULL,
                expiration INTEGER NOT NULL,
                PRIMARY KEY (query, page)
            );
            CREATE TABLE IF NOT EXISTS game_name_mapping (
                english_name TEXT PRIMARY KEY,
                chinese_name TEXT NOT NULL,
                aliases TEXT
            );
            ",
        )?;
        log::info!("数据库表结构创建完成");
        Ok::<(), anyhow::Error>(())
    })
    .await
    .context("初始化数据库失败")??;

    log::info!("开始初始化游戏名称映射");
    // 初始化游戏名称映射
    init_game_name_mapping().await?;
    log::info!("游戏名称映射初始化完成");

    Ok(())
}

pub async fn save_installed_trainers(trainers: Vec<InstalledTrainer>) -> Result<()> {
    with_conn(move |conn| {
        let tx = conn.transaction()?;
        tx.execute("DELETE FROM installed_trainers", [])?;

        {
            let mut stmt = tx.prepare(
                "
                INSERT INTO installed_trainers (
                    id, name, version, game_version, download_url,
                    description, thumbnail, download_count, last_update,
                    installed_path, install_time, last_launch_time
                ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
                ",
            )?;

            for t in trainers {
                stmt.execute(params![
                    t.id,
                    t.name,
                    t.version,
                    t.game_version,
                    t.download_url,
                    t.description,
                    t.thumbnail,
                    t.download_count,
                    t.last_update,
                    t.installed_path,
                    t.install_time,
                    t.last_launch_time,
                ])?;
            }
        }

        tx.commit()?;
        Ok(())
    })
    .await
}

pub async fn get_installed_trainers() -> Result<Vec<InstalledTrainer>> {
    log::info!("开始加载已安装的修改器");
    let trainers = with_conn(move |conn| {
        let mut stmt = conn.prepare(
            "
            SELECT id, name, version, game_version, download_url,
                   description, thumbnail, download_count, last_update,
                   installed_path, install_time, last_launch_time
            FROM installed_trainers
            ",
        )?;

        let rows = stmt.query_map([], |row| {
            Ok(InstalledTrainer {
                id: row.get(0)?,
                name: row.get(1)?,
                version: row.get(2)?,
                game_version: row.get(3)?,
                download_url: row.get(4)?,
                description: row.get(5)?,
                thumbnail: row.get(6)?,
                download_count: row.get(7)?,
                last_update: row.get(8)?,
                installed_path: row.get(9)?,
                install_time: row.get(10)?,
                last_launch_time: row.get::<_, Option<String>>(11)?.unwrap_or_default(),
            })
        })?;

        let mut result = Vec::new();
        for trainer in rows {
            result.push(trainer?);
        }
        Ok(result)
    })
    .await?;

    // 翻译名称为中文
    log::info!("翻译已安装修改器的名称，共 {} 个", trainers.len());
    let mut translated_trainers = Vec::new();
    for mut trainer in trainers {
        let chinese_name = get_chinese_game_name(trainer.name.clone()).await?;
        if chinese_name != trainer.name {
            log::info!("翻译已安装修改器: {} -> {}", trainer.name, chinese_name);
        }
        trainer.name = chinese_name;
        translated_trainers.push(trainer);
    }

    Ok(translated_trainers)
}

pub async fn save_downloaded_trainers(trainers: Vec<Trainer>) -> Result<()> {
    with_conn(move |conn| {
        let tx = conn.transaction()?;
        tx.execute("DELETE FROM downloaded_trainers", [])?;

        {
            let mut stmt = tx.prepare(
                "
                INSERT INTO downloaded_trainers (
                    id, name, version, game_version, download_url,
                    description, thumbnail, download_count, last_update
                ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
                ",
            )?;

            for t in trainers {
                stmt.execute(params![
                    t.id,
                    t.name,
                    t.version,
                    t.game_version,
                    t.download_url,
                    t.description,
                    t.thumbnail,
                    t.download_count,
                    t.last_update
                ])?;
            }
        }

        tx.commit()?;
        Ok(())
    })
    .await
}

pub async fn get_downloaded_trainers() -> Result<Vec<Trainer>> {
    println!("[DEBUG] get_downloaded_trainers() 开始执行");
    log::info!("开始加载已下载的修改器");
    let trainers = with_conn(move |conn| {
        let mut stmt = conn.prepare(
            "
            SELECT id, name, version, game_version, download_url,
                   description, thumbnail, download_count, last_update
            FROM downloaded_trainers
            ",
        )?;

        let rows = stmt.query_map([], |row| {
            Ok(Trainer {
                id: row.get(0)?,
                name: row.get(1)?,
                version: row.get(2)?,
                game_version: row.get(3)?,
                download_url: row.get(4)?,
                description: row.get(5)?,
                thumbnail: row.get(6)?,
                download_count: row.get(7)?,
                last_update: row.get(8)?,
            })
        })?;

        let mut result = Vec::new();
        for trainer in rows {
            result.push(trainer?);
        }
        println!("[DEBUG] 从数据库读取了 {} 个修改器", result.len());
        Ok(result)
    })
    .await?;

    // 翻译名称为中文
    println!("[DEBUG] 准备翻译 {} 个修改器名称", trainers.len());
    for trainer in &trainers {
        println!("[DEBUG] 原始名称: {}", trainer.name);
    }

    log::info!("翻译已下载修改器的名称，共 {} 个", trainers.len());
    let translated_trainers = translate_trainer_names(trainers).await?;

    println!("[DEBUG] 翻译完成:");
    for trainer in &translated_trainers {
        println!("[DEBUG] 翻译后: {}", trainer.name);
    }

    Ok(translated_trainers)
}

pub async fn upsert_downloaded_trainer(trainer: Trainer) -> Result<()> {
    with_conn(move |conn| {
        conn.execute(
            "
            INSERT INTO downloaded_trainers (
                id, name, version, game_version, download_url,
                description, thumbnail, download_count, last_update
            ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9)
            ON CONFLICT(id) DO UPDATE SET
                name = excluded.name,
                version = excluded.version,
                game_version = excluded.game_version,
                download_url = excluded.download_url,
                description = excluded.description,
                thumbnail = excluded.thumbnail,
                download_count = excluded.download_count,
                last_update = excluded.last_update
            ",
            params![
                trainer.id,
                trainer.name,
                trainer.version,
                trainer.game_version,
                trainer.download_url,
                trainer.description,
                trainer.thumbnail,
                trainer.download_count,
                trainer.last_update,
            ],
        )?;
        Ok(())
    })
    .await
}

pub async fn remove_downloaded_trainer(trainer_id: &str) -> Result<()> {
    let id = trainer_id.to_string();
    with_conn(move |conn| {
        conn.execute("DELETE FROM downloaded_trainers WHERE id = ?1", params![id])?;
        Ok(())
    })
    .await
}

pub async fn upsert_installed_trainer(trainer: InstalledTrainer) -> Result<()> {
    with_conn(move |conn| {
        conn.execute(
            "
            INSERT INTO installed_trainers (
                id, name, version, game_version, download_url,
                description, thumbnail, download_count, last_update,
                installed_path, install_time, last_launch_time
            ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12)
            ON CONFLICT(id) DO UPDATE SET
                name = excluded.name,
                version = excluded.version,
                game_version = excluded.game_version,
                download_url = excluded.download_url,
                description = excluded.description,
                thumbnail = excluded.thumbnail,
                download_count = excluded.download_count,
                last_update = excluded.last_update,
                installed_path = excluded.installed_path,
                install_time = excluded.install_time,
                last_launch_time = excluded.last_launch_time
            ",
            params![
                trainer.id,
                trainer.name,
                trainer.version,
                trainer.game_version,
                trainer.download_url,
                trainer.description,
                trainer.thumbnail,
                trainer.download_count,
                trainer.last_update,
                trainer.installed_path,
                trainer.install_time,
                trainer.last_launch_time
            ],
        )?;
        Ok(())
    })
    .await
}

pub async fn get_installed_trainer_by_id(id: &str) -> Result<Option<InstalledTrainer>> {
    let trainer_id = id.to_string();
    with_conn(move |conn| {
        let mut stmt = conn.prepare(
            "
            SELECT id, name, version, game_version, download_url,
                   description, thumbnail, download_count, last_update,
                   installed_path, install_time, last_launch_time
            FROM installed_trainers WHERE id = ?1
            ",
        )?;
        let mut rows = stmt.query(params![trainer_id])?;
        if let Some(row) = rows.next()? {
            let trainer = InstalledTrainer {
                id: row.get(0)?,
                name: row.get(1)?,
                version: row.get(2)?,
                game_version: row.get(3)?,
                download_url: row.get(4)?,
                description: row.get(5)?,
                thumbnail: row.get(6)?,
                download_count: row.get(7)?,
                last_update: row.get(8)?,
                installed_path: row.get(9)?,
                install_time: row.get(10)?,
                last_launch_time: row.get::<_, Option<String>>(11)?.unwrap_or_default(),
            };
            Ok(Some(trainer))
        } else {
            Ok(None)
        }
    })
    .await
}

pub async fn remove_installed_trainer(id: &str) -> Result<()> {
    let trainer_id = id.to_string();
    with_conn(move |conn| {
        conn.execute("DELETE FROM installed_trainers WHERE id = ?1", params![trainer_id])?;
        Ok(())
    })
    .await
}

pub async fn update_last_launch_time(id: &str, timestamp: &str) -> Result<()> {
    let trainer_id = id.to_string();
    let ts = timestamp.to_string();
    with_conn(move |conn| {
        conn.execute(
            "UPDATE installed_trainers SET last_launch_time = ?1 WHERE id = ?2",
            params![ts, trainer_id],
        )?;
        Ok(())
    })
    .await
}

pub async fn cache_trainer_list(page: u32, trainers: Vec<Trainer>) -> Result<()> {
    let expiration = Utc::now().timestamp_millis() + CACHE_EXPIRATION_TIME;
    let data = serde_json::to_string(&trainers)?;

    with_conn(move |conn| {
        conn.execute(
            "
            INSERT INTO trainer_cache (page, data, expiration)
            VALUES (?1, ?2, ?3)
            ON CONFLICT(page) DO UPDATE SET
                data = excluded.data,
                expiration = excluded.expiration
            ",
            params![page, data, expiration],
        )?;
        Ok(())
    })
    .await
}

pub async fn get_cached_trainer_list(page: u32) -> Result<Option<Vec<Trainer>>> {
    let now = Utc::now().timestamp_millis();
    with_conn(move |conn| {
        let mut stmt = conn.prepare(
            "
            SELECT data, expiration FROM trainer_cache WHERE page = ?1
            ",
        )?;
        let mut rows = stmt.query(params![page])?;
        if let Some(row) = rows.next()? {
            let expiration: i64 = row.get(1)?;
            if expiration < now {
                conn.execute("DELETE FROM trainer_cache WHERE page = ?1", params![page])?;
                return Ok(None);
            }
            let data: String = row.get(0)?;
            let trainers: Vec<Trainer> = serde_json::from_str(&data)?;
            Ok(Some(trainers))
        } else {
            Ok(None)
        }
    })
    .await
}

pub async fn cache_search_results(query: String, page: u32, trainers: Vec<Trainer>) -> Result<()> {
    let expiration = Utc::now().timestamp_millis() + CACHE_EXPIRATION_TIME;
    let data = serde_json::to_string(&trainers)?;

    with_conn(move |conn| {
        conn.execute(
            "
            INSERT INTO search_cache (query, page, data, expiration)
            VALUES (?1, ?2, ?3, ?4)
            ON CONFLICT(query, page) DO UPDATE SET
                data = excluded.data,
                expiration = excluded.expiration
            ",
            params![query, page, data, expiration],
        )?;
        Ok(())
    })
    .await
}

pub async fn get_cached_search_results(query: String, page: u32) -> Result<Option<Vec<Trainer>>> {
    let now = Utc::now().timestamp_millis();
    with_conn(move |conn| {
        let mut stmt = conn.prepare(
            "
            SELECT data, expiration FROM search_cache
            WHERE query = ?1 AND page = ?2
            ",
        )?;
        let mut rows = stmt.query(params![query, page])?;
        if let Some(row) = rows.next()? {
            let expiration: i64 = row.get(1)?;
            if expiration < now {
                conn.execute(
                    "DELETE FROM search_cache WHERE query = ?1 AND page = ?2",
                    params![query, page],
                )?;
                return Ok(None);
            }
            let data: String = row.get(0)?;
            let trainers: Vec<Trainer> = serde_json::from_str(&data)?;
            Ok(Some(trainers))
        } else {
            Ok(None)
        }
    })
    .await
}

pub async fn clean_expired_cache() -> Result<()> {
    let now = Utc::now().timestamp_millis();
    with_conn(move |conn| {
        conn.execute(
            "DELETE FROM trainer_cache WHERE expiration < ?1",
            params![now],
        )?;
        conn.execute(
            "DELETE FROM search_cache WHERE expiration < ?1",
            params![now],
        )?;
        Ok(())
    })
    .await
}

pub async fn get_all_keys() -> Result<Vec<String>> {
    with_conn(move |conn| {
        let mut keys = Vec::new();

        let installed_count: i64 =
            conn.query_row("SELECT COUNT(*) FROM installed_trainers", [], |row| row.get(0))?;
        if installed_count > 0 {
            keys.push(INSTALLED_TRAINERS_KEY.to_string());
        }

        let downloaded_count: i64 =
            conn.query_row("SELECT COUNT(*) FROM downloaded_trainers", [], |row| row.get(0))?;
        if downloaded_count > 0 {
            keys.push(DOWNLOADED_TRAINERS_KEY.to_string());
        }

        let mut trainer_stmt = conn.prepare("SELECT page FROM trainer_cache")?;
        let trainer_iter = trainer_stmt.query_map([], |row| row.get::<_, i64>(0))?;
        for page in trainer_iter {
            let page = page?;
            keys.push(format!("{}{}", TRAINER_LIST_PREFIX, page));
        }

        let mut search_stmt = conn.prepare("SELECT query, page FROM search_cache")?;
        let search_iter =
            search_stmt.query_map([], |row| Ok((row.get::<_, String>(0)?, row.get::<_, i64>(1)?)))?;
        for item in search_iter {
            let (query, page) = item?;
            keys.push(format!("{}{}_{}", SEARCH_RESULTS_PREFIX, query, page));
        }

        Ok(keys)
    })
    .await
}

pub async fn clear_all() -> Result<()> {
    with_conn(move |conn| {
        conn.execute("DELETE FROM installed_trainers", [])?;
        conn.execute("DELETE FROM downloaded_trainers", [])?;
        conn.execute("DELETE FROM trainer_cache", [])?;
        conn.execute("DELETE FROM search_cache", [])?;
        Ok(())
    })
    .await
}

pub async fn migrate_from_local_storage(local_data: HashMap<String, Value>) -> Result<()> {
    let mut installed: Option<Vec<InstalledTrainer>> = None;
    let mut downloaded: Option<Vec<Trainer>> = None;
    let mut trainer_cache_items: Vec<(u32, CacheItem<Vec<Trainer>>)> = Vec::new();
    let mut search_cache_items: Vec<(String, u32, CacheItem<Vec<Trainer>>)> = Vec::new();

    for (key, value) in local_data {
        if key == INSTALLED_TRAINERS_KEY {
            if let Ok(items) = serde_json::from_value::<Vec<InstalledTrainer>>(value.clone()) {
                installed = Some(items);
            }
        } else if key == DOWNLOADED_TRAINERS_KEY {
            if let Ok(items) = serde_json::from_value::<Vec<Trainer>>(value.clone()) {
                downloaded = Some(items);
            }
        } else if key.starts_with(TRAINER_LIST_PREFIX) {
            if let Some(page_str) = key.strip_prefix(TRAINER_LIST_PREFIX) {
                if let Ok(page) = page_str.parse::<u32>() {
                    if let Ok(cache_item) =
                        serde_json::from_value::<CacheItem<Vec<Trainer>>>(value.clone())
                    {
                        if cache_item.expiration > Utc::now().timestamp_millis() {
                            trainer_cache_items.push((page, cache_item));
                        }
                    }
                }
            }
        } else if key.starts_with(SEARCH_RESULTS_PREFIX) {
            if let Some(rest) = key.strip_prefix(SEARCH_RESULTS_PREFIX) {
                if let Some((query_part, page_part)) = rest.rsplit_once('_') {
                    if let Ok(page) = page_part.parse::<u32>() {
                        let query = query_part.trim_start_matches('_').to_string();
                        if let Ok(cache_item) =
                            serde_json::from_value::<CacheItem<Vec<Trainer>>>(value.clone())
                        {
                            if cache_item.expiration > Utc::now().timestamp_millis() {
                                search_cache_items.push((query, page, cache_item));
                            }
                        }
                    }
                }
            }
        }
    }

    if let Some(items) = installed {
        save_installed_trainers(items).await?;
    }
    if let Some(items) = downloaded {
        save_downloaded_trainers(items).await?;
    }

    for (page, cache_item) in trainer_cache_items {
        let expiration = cache_item.expiration;
        let data = serde_json::to_string(&cache_item.data)?;
        let _ = with_conn(move |conn| {
            conn.execute(
                "
                INSERT INTO trainer_cache (page, data, expiration)
                VALUES (?1, ?2, ?3)
                ON CONFLICT(page) DO UPDATE SET
                    data = excluded.data,
                    expiration = excluded.expiration
                ",
                params![page, data, expiration],
            )?;
            Ok(())
        })
        .await?;
    }

    for (query, page, cache_item) in search_cache_items {
        let expiration = cache_item.expiration;
        let data = serde_json::to_string(&cache_item.data)?;
        let _ = with_conn(move |conn| {
            conn.execute(
                "
                INSERT INTO search_cache (query, page, data, expiration)
                VALUES (?1, ?2, ?3, ?4)
                ON CONFLICT(query, page) DO UPDATE SET
                    data = excluded.data,
                    expiration = excluded.expiration
                ",
                params![query, page, data, expiration],
            )?;
            Ok(())
        })
        .await?;
    }

    Ok(())
}

/// 初始化游戏名称映射数据
pub async fn init_game_name_mapping() -> Result<()> {
    log::info!("init_game_name_mapping() 开始执行");
    with_conn(move |conn| {
        let count: i64 = conn.query_row(
            "SELECT COUNT(*) FROM game_name_mapping",
            [],
            |row| row.get(0),
        )?;

        log::info!("游戏名称映射表中已有 {} 条记录", count);

        log::info!("开始插入/更新游戏名称映射数据...");

        // 插入游戏名称映射数据
        let mappings = vec![
            ("Grand Theft Auto V", "侠盗猎车手5", r#"["GTA5", "GTA V", "GTA5", "侠盗猎车手V", "侠盗飞车5"]"#),
            ("Elden Ring", "艾尔登法环", r#"["老头环", "艾尔登", "法环"]"#),
            ("Cyberpunk 2077", "赛博朋克2077", r#"["赛博朋克", "2077", "CP2077"]"#),
            ("Red Dead Redemption 2", "荒野大镖客2", r#"["大镖客2", "RDR2", "荒野大镖客救赎2"]"#),
            ("The Witcher 3: Wild Hunt", "巫师3：狂猎", r#"["巫师3", "狂猎", "Witcher 3"]"#),
            ("Resident Evil 4 Remake", "生化危机4重制版", r#"["生化4重制版", "RE4R", "生化危机4"]"#),
            ("Hogwarts Legacy", "霍格沃茨之遗", r#"["霍格沃茨", "哈利波特游戏"]"#),
            ("Starfield", "星空", r#"["星空", "Starfield"]"#),
            ("Baldur's Gate 3", "博德之门3", r#"["博德之门", "BG3", "博德3"]"#),
            ("God of War", "战神", r#"["战神", "奎托斯", "God of War"]"#),
            ("Spider-Man Remastered", "蜘蛛侠重制版", r#"["蜘蛛侠", "漫威蜘蛛侠"]"#),
            ("Horizon Zero Dawn", "地平线：零之曙光", r#"["地平线零之曙光", "零之曙光"]"#),
            ("Horizon Forbidden West", "地平线：西之绝境", r#"["地平线西之绝境", "西之绝境"]"#),
            ("Days Gone", "往日不再", r#"["往日不再", "Days Gone"]"#),
            ("Death Stranding", "死亡搁浅", r#"["死亡搁浅", "Death Stranding"]"#),
            ("Monster Hunter: World", "怪物猎人：世界", r#"["怪物猎人世界", "怪猎世界", "MHW"]"#),
            ("Monster Hunter Rise", "怪物猎人：崛起", r#"["怪物猎人崛起", "怪猎崛起", "MHR"]"#),
            ("Sekiro: Shadows Die Twice", "只狼：影逝二度", r#"["只狼", "影逝二度", "Sekiro"]"#),
            ("Dark Souls III", "黑暗之魂3", r#"["黑魂3", "Dark Souls 3", "DS3"]"#),
            ("Dark Souls Remastered", "黑暗之魂重制版", r#"["黑魂重制版", "DSR"]"#),
            ("Dark Souls II: Scholar of the First Sin", "黑暗之魂2：原罪学者", r#"["黑魂2", "DS2"]"#),
            ("Assassin's Creed Valhalla", "刺客信条：英灵殿", r#"["刺客信条英灵殿", "英灵殿", "ACV"]"#),
            ("Assassin's Creed Odyssey", "刺客信条：奥德赛", r#"["刺客信条奥德赛", "奥德赛", "ACO"]"#),
            ("Assassin's Creed Origins", "刺客信条：起源", r#"["刺客信条起源", "起源", "ACO"]"#),
            ("Assassin's Creed Mirage", "刺客信条：幻景", r#"["刺客信条幻景", "幻景", "ACM"]"#),
            ("Far Cry 6", "孤岛惊魂6", r#"["孤岛惊魂6", "FC6", "远哭6"]"#),
            ("Far Cry 5", "孤岛惊魂5", r#"["孤岛惊魂5", "FC5", "远哭5"]"#),
            ("Watch Dogs: Legion", "看门狗：军团", r#"["看门狗军团", "军团", "WDL"]"#),
            ("Watch Dogs 2", "看门狗2", r#"["看门狗2", "WD2"]"#),
            ("Tom Clancy's Ghost Recon Wildlands", "幽灵行动：荒野", r#"["幽灵行动荒野", "荒野"]"#),
            ("Tom Clancy's Ghost Recon Breakpoint", "幽灵行动：断点", r#"["幽灵行动断点", "断点"]"#),
            ("Tom Clancy's Rainbow Six Siege", "彩虹六号：围攻", r#"["彩虹六号", "R6", "R6S"]"#),
            ("Forza Horizon 5", "极限竞速：地平线5", r#"["地平线5", "FH5", "极限竞速5"]"#),
            ("Forza Horizon 4", "极限竞速：地平线4", r#"["地平线4", "FH4", "极限竞速4"]"#),
            ("Forza Motorsport", "极限竞速", r#"["极限竞速", "FM", "Forza"]"#),
            ("Need for Speed Heat", "极品飞车：热度", r#"["极品飞车热度", "热度", "NFS Heat"]"#),
            ("Need for Speed Unbound", "极品飞车：不羁", r#"["极品飞车不羁", "不羁", "NFS Unbound"]"#),
            ("FIFA 23", "FIFA 23", r#"["FIFA23", "国际足联23"]"#),
            ("EA Sports FC 24", "EA Sports FC 24", r#"["FC24", "FIFA24"]"#),
            ("NBA 2K24", "NBA 2K24", r#"["NBA2K24", "2K24"]"#),
            ("Madden NFL 24", "麦登橄榄球24", r#"["Madden24", "橄榄球24"]"#),
            ("Call of Duty: Modern Warfare II", "使命召唤：现代战争2", r#"["使命召唤19", "COD19", "MW2"]"#),
            ("Call of Duty: Modern Warfare III", "使命召唤：现代战争3", r#"["使命召唤20", "COD20", "MW3"]"#),
            ("Call of Duty: Black Ops Cold War", "使命召唤：黑色行动冷战", r#"["使命召唤17", "COD17", "BOCW"]"#),
            ("Call of Duty: Vanguard", "使命召唤：先锋", r#"["使命召唤18", "COD18", "先锋"]"#),
            ("Battlefield 2042", "战地2042", r#"["战地2042", "BF2042", "BF4"]"#),
            ("Battlefield V", "战地5", r#"["战地5", "BF5", "BFV"]"#),
            ("Battlefield 1", "战地1", r#"["战地1", "BF1"]"#),
            ("Borderlands 3", "无主之地3", r#"["无主之地3", "BL3", "边境之地3"]"#),
            ("Borderlands 2", "无主之地2", r#"["无主之地2", "BL2", "边境之地2"]"#),
            ("Destiny 2", "命运2", r#"["命运2", "Destiny2", "天命2"]"#),
            ("The Division 2", "全境封锁2", r#"["全境封锁2", "Division2", "TD2"]"#),
            ("Outriders", "先驱者", r#"["先驱者", "Outriders"]"#),
            ("Remnant: From the Ashes", "遗迹：灰烬重生", r#"["遗迹灰烬重生", "Remnant"]"#),
            ("Remnant II", "遗迹2", r#"["遗迹2", "Remnant 2"]"#),
            ("Dying Light 2", "消逝的光芒2", r#"["消逝的光芒2", "DL2", " dying light 2"]"#),
            ("Dying Light", "消逝的光芒", r#"["消逝的光芒", "DL", "Dying Light"]"#),
            ("Dead Rising 4", "丧尸围城4", r#"["丧尸围城4", "DR4"]"#),
            ("State of Decay 2", "腐烂国度2", r#"["腐烂国度2", "SoD2", "腐烂2"]"#),
            ("Resident Evil Village", "生化危机8：村庄", r#"["生化危机8", "生化危机村庄", "RE8"]"#),
            ("Resident Evil 2 Remake", "生化危机2重制版", r#"["生化2重制版", "RE2R", "生化危机2"]"#),
            ("Resident Evil 3 Remake", "生化危机3重制版", r#"["生化3重制版", "RE3R", "生化危机3"]"#),
            ("Devil May Cry 5", "鬼泣5", r#"["鬼泣5", "DMC5", "Devil May Cry 5"]"#),
            ("Street Fighter 6", "街头霸王6", r#"["街霸6", "SF6", "街头霸王6"]"#),
            ("Tekken 8", "铁拳8", r#"["铁拳8", "Tekken8"]"#),
            ("Mortal Kombat 1", "真人快打1", r#"["真人快打1", "MK1"]"#),
            ("Control", "控制", r#"["控制", "Control"]"#),
            ("Alan Wake 2", "心灵杀手2", r#"["心灵杀手2", "Alan Wake 2"]"#),
            ("A Plague Tale: Innocence", "瘟疫传说：无罪", r#"["瘟疫传说无罪", "无罪"]"#),
            ("A Plague Tale: Requiem", "瘟疫传说：安魂曲", r#"["瘟疫传说安魂曲", "安魂曲"]"#),
            ("Shadow of the Tomb Raider", "古墓丽影：暗影", r#"["古墓丽影暗影", "暗影", "SOTTR"]"#),
            ("Rise of the Tomb Raider", "古墓丽影：崛起", r#"["古墓丽影崛起", "崛起", "ROTTR"]"#),
            ("Tomb Raider (2013)", "古墓丽影(2013)", r#"["古墓丽影9", "TR9", "古墓丽影2013"]"#),
            ("Final Fantasy XVI", "最终幻想16", r#"["最终幻想16", "FF16", "FFXVI"]"#),
            ("Final Fantasy XV", "最终幻想15", r#"["最终幻想15", "FF15", "FFXV"]"#),
            ("Final Fantasy VII Remake", "最终幻想7重制版", r#"["最终幻想7重制版", "FF7R", "FF7重制版"]"#),
            ("Kingdom Hearts III", "王国之心3", r#"["王国之心3", "KH3"]"#),
            ("Persona 5 Royal", "女神异闻录5皇家版", r#"["P5R", "女神异闻录5R", "P5皇家版"]"#),
            ("Persona 3 Reload", "女神异闻录3重制版", r#"["P3R", "女神异闻录3R", "P3重制版"]"#),
            ("Dragon's Dogma 2", "龙之信条2", r#"["龙之信条2", "DD2"]"#),
            ("Nier: Automata", "尼尔：机械纪元", r#"["尼尔机械纪元", "尼尔", "Nier"]"#),
            ("Nioh 2", "仁王2", r#"["仁王2", "Nioh 2"]"#),
            ("Wo Long: Fallen Dynasty", "卧龙：苍天陨落", r#"["卧龙", "苍天陨落", "Wo Long"]"#),
            ("Rise of the Ronin", "浪人崛起", r#"["浪人崛起", "Rise of Ronin"]"#),
            ("Lies of P", "匹诺曹的谎言", r#"["匹诺曹", "谎言", "Lies of P"]"#),
            ("Stray", "流浪", r#"["流浪", "猫咪游戏", "Stray"]"#),
            ("Kena: Bridge of Spirits", "柯娜：精神之桥", r#"["柯娜", "精神之桥", "Kena"]"#),
            ("Sifu", "师父", r#"["师父", "Sifu"]"#),
            ("It Takes Two", "双人成行", r#"["双人成行", "It Takes Two"]"#),
            ("Hades", "哈迪斯", r#"["哈迪斯", "Hades"]"#),
            ("Hollow Knight", "空洞骑士", r#"["空洞骑士", "Hollow Knight"]"#),
            ("Ori and the Will of the Wisps", "精灵与萤火意志", r#"["精灵与萤火意志", "Ori 2", "奥日2"]"#),
            ("Ori and the Blind Forest", "精灵与森林", r#"["精灵与森林", "Ori 1", "奥日1"]"#),
            ("Celeste", "蔚蓝", r#"["蔚蓝", "Celeste"]"#),
            ("Dead Cells", "死亡细胞", r#"["死亡细胞", "Dead Cells"]"#),
            ("Hades II", "哈迪斯2", r#"["哈迪斯2", "Hades 2"]"#),
            ("Disco Elysium", "极乐迪斯科", r#"["极乐迪斯科", "Disco Elysium"]"#),
            ("The Outer Worlds", "天外世界", r#"["天外世界", "Outer Worlds"]"#),
            ("Star Wars Jedi: Fallen Order", "星球大战绝地：陨落的武士团", r#"["绝地陨落", "JFO", "星球大战绝地"]"#),
            ("Star Wars Jedi: Survivor", "星球大战绝地：幸存者", r#"["绝地幸存者", "JS", "星球大战绝地2"]"#),
            ("Mass Effect Legendary Edition", "质量效应：传奇版", r#"["质量效应传奇版", "MELE"]"#),
            ("Dragon Age: Inquisition", "龙腾世纪：审判", r#"["龙腾世纪审判", "DAI"]"#),
            ("Cyberpunk 2077: Phantom Liberty", "赛博朋克2077：自由幻影", r#"["自由幻影", "Phantom Liberty"]"#),
            ("Shadow Warrior 3", "影武者3", r#"["影武者3", "SW3"]"#),
            ("Serious Sam 4", "英雄萨姆4", r#"["英雄萨姆4", "SS4"]"#),
            ("DOOM Eternal", "毁灭战士：永恒", r#"["毁灭战士永恒", "DOOM永恒", "永恒"]"#),
            // 2024-2025 热门游戏
            ("Stellar Blade", "剑星", r#"["剑星", "星刃", "Stellar Blade"]"#),
            ("Black Myth: Wukong", "黑神话：悟空", r#"["黑神话悟空", "悟空", "Wukong", "Black Myth"]"#),
            ("Palworld", "幻兽帕鲁", r#"["幻兽帕鲁", "帕鲁", "Palworld", "Pocketpair"]"#),
            ("Dragon's Dogma 2", "龙之信条2", r#"["龙之信条2", "DD2", "Dragon's Dogma II"]"#),
            ("Final Fantasy VII Rebirth", "最终幻想7重生", r#"["最终幻想7重生", "FF7重生", "FFVII Rebirth"]"#),
            ("Tekken 8", "铁拳8", r#"["铁拳8", "Tekken 8", "TK8"]"#),
            ("Like a Dragon: Infinite Wealth", "如龙8", r#"["如龙8", "无限财富", "LAD8", "Yakuza 8"]"#),
            ("Grand Theft Auto VI", "侠盗猎车手6", r#"["GTA6", "GTA VI", "侠盗猎车手VI", "侠盗飞车6"]"#),
            ("Suicide Squad: Kill the Justice League", "自杀小队：杀死正义联盟", r#"["自杀小队", "杀死正义联盟", "SSKTJL"]"#),
            ("Skull and Bones", "骷髅与骨头", r#"["骷髅与骨头", "海盗游戏", "Skull & Bones"]"#),
            ("Prince of Persia: The Lost Crown", "波斯王子：失落的王冠", r#"["波斯王子失落的王冠", "失落的王冠", "PoP"]"#),
            ("Helldivers 2", "绝地潜兵2", r#"["绝地潜兵2", "地狱潜者2", "Helldivers 2"]"#),
            ("Dragon's Dogma 2", "龙之信条2", r#"["龙之信条2", "DD2"]"#),
            ("V Rising", "吸血鬼崛起", r#"["吸血鬼崛起", "V Rising"]"#),
            ("Enshrouded", "雾锁王国", r#"["雾锁王国", "Enshrouded"]"#),
            ("Pacific Drive", "太平洋驾驶", r#"["太平洋驾驶", "Pacific Drive"]"#),
            ("Granblue Fantasy: Relink", "碧蓝幻想：Relink", r#"["碧蓝幻想Relink", "GBF Relink"]"#),
            ("Persona 3 Reload", "女神异闻录3重制版", r#"["P3R", "女神异闻录3R", "P3重制版"]"#),
            ("Foamstars", "泡沫之星", r#"["泡沫之星", "Foamstars"]"#),
            ("Banishers: Ghosts of New Eden", "放逐者：新伊甸园的幽灵", r#"["放逐者", "新伊甸园幽灵"]"#),
            ("Senua's Saga: Hellblade II", "地狱之刃2：塞娜的传奇", r#"["地狱之刃2", "塞娜传奇", "Hellblade 2"]"#),
            ("Avowed", "宣誓", r#"["宣誓", "Avowed"]"#),
            ("Indiana Jones and the Great Circle", "印第安纳琼斯：大圆", r#"["印第安纳琼斯", "大圆"]"#),
            ("S.T.A.L.K.E.R. 2: Heart of Chornobyl", "潜行者2：切尔诺贝利之心", r#"["潜行者2", "切尔诺贝利之心", "STALKER 2"]"#),
            ("Hollow Knight: Silksong", "空洞骑士：丝之歌", r#"["空洞骑士丝之歌", "丝之歌", "Silksong"]"#),
            ("Hades II", "哈迪斯2", r#"["哈迪斯2", "Hades 2"]"#),
            ("Clockwork Revolution", "发条革命", r#"["发条革命", "Clockwork Revolution"]"#),
            ("Fable", "神鬼寓言", r#"["神鬼寓言", "Fable", "寓言"]"#),
            ("Perfect Dark", "完美黑暗", r#"["完美黑暗", "Perfect Dark"]"#),
            // 更多经典3A大作
            ("Elden Ring: Shadow of the Erdtree", "艾尔登法环：黄金树之影", r#"["黄金树之影", "艾尔登法环DLC", "Shadow of the Erdtree"]"#),
            ("Diablo IV", "暗黑破坏神4", r#"["暗黑4", "Diablo 4", "D4"]"#),
            ("Diablo III", "暗黑破坏神3", r#"["暗黑3", "Diablo 3", "D3"]"#),
            ("Overwatch 2", "守望先锋2", r#"["守望先锋2", "OW2", "守望2"]"#),
            ("Valorant", "无畏契约", r#"["无畏契约", "Valorant", "瓦洛兰特"]"#),
            ("Apex Legends", "Apex英雄", r#"["Apex英雄", "APEX", "派派"]"#),
            ("Fallout 4", "辐射4", r#"["辐射4", "Fallout 4", "FO4"]"#),
            ("Fallout: New Vegas", "辐射：新维加斯", r#"["辐射新维加斯", "新维加斯", "FNV"]"#),
            ("The Elder Scrolls V: Skyrim", "上古卷轴5：天际", r#"["上古卷轴5", "天际", "Skyrim", "老滚5"]"#),
            ("The Elder Scrolls IV: Oblivion", "上古卷轴4：湮没", r#"["上古卷轴4", "湮没", "Oblivion"]"#),
            ("The Elder Scrolls Online", "上古卷轴OL", r#"["上古卷轴OL", "ESO", "老滚OL"]"#),
            ("Star Wars: Battlefront II", "星球大战：前线2", r#"["星球大战前线2", "前线2", "SWBF2"]"#),
            ("Anthem", "圣歌", r#"["圣歌", "Anthem"]"#),
            ("Marvel's Avengers", "漫威复仇者联盟", r#"["漫威复仇者", "复仇者联盟游戏"]"#),
            ("Marvel's Spider-Man 2", "漫威蜘蛛侠2", r#"["蜘蛛侠2", "漫威蜘蛛侠2", "Spider-Man 2"]"#),
            ("Marvel's Spider-Man: Miles Morales", "漫威蜘蛛侠：迈尔斯·莫拉莱斯", r#"["蜘蛛侠迈尔斯", "迈尔斯", "Miles Morales"]"#),
            ("God of War Ragnarök", "战神：诸神黄昏", r#"["战神诸神黄昏", "诸神黄昏", "Ragnarök"]"#),
            ("The Last of Us Part I", "最后生还者第一部", r#"["最后生还者1", "美国末日1", "TLOU1"]"#),
            ("The Last of Us Part II", "最后生还者第二部", r#"["最后生还者2", "美国末日2", "TLOU2"]"#),
            ("Uncharted 4: A Thief's End", "神秘海域4：盗贼末路", r#"["神秘海域4", "盗贼末路", "UC4"]"#),
            ("Uncharted: Legacy of Thieves Collection", "神秘海域：盗贼传奇合集", r#"["神秘海域合集", "盗贼传奇"]"#),
            ("Ghost of Tsushima", "对马岛之魂", r#"["对马岛之魂", "对马岛", "Tsushima"]"#),
            ("Death Stranding Director's Cut", "死亡搁浅导演剪辑版", r#"["死亡搁浅导演版", "导演剪辑版"]"#),
            ("Returnal", "死亡回归", r#"["死亡回归", "Returnal"]"#),
            ("Demon's Souls", "恶魔之魂", r#"["恶魔之魂", "Demon's Souls"]"#),
            ("Bloodborne", "血源诅咒", r#"["血源诅咒", "血源", "Bloodborne"]"#),
            ("Armored Core VI: Fires of Rubicon", "装甲核心6：境界天火", r#"["装甲核心6", "境界天火", "AC6"]"#),
            ("Metal Gear Solid V: The Phantom Pain", "合金装备5：幻痛", r#"["合金装备5", "幻痛", "MGS5", "MGSV"]"#),
            ("Metal Gear Solid Δ: Snake Eater", "合金装备Δ：食蛇者", r#"["合金装备3重制版", "食蛇者", "MGS3"]"#),
            ("Silent Hill 2 Remake", "寂静岭2重制版", r#"["寂静岭2重制版", "SH2 Remake", "寂静岭2"]"#),
            ("Alan Wake Remastered", "心灵杀手重制版", r#"["心灵杀手重制版", "Alan Wake Remastered"]"#),
            ("Quantum Break", "量子破碎", r#"["量子破碎", "Quantum Break"]"#),
            ("Max Payne 3", "马克思·佩恩3", r#"["马克思佩恩3", "Max Payne 3", "MP3"]"#),
            // 更多独立游戏和日本游戏
            ("Persona 5 Royal", "女神异闻录5皇家版", r#"["P5R", "女神异闻录5R", "P5皇家版"]"#),
            ("Persona 5 Strikers", "女神异闻录5：乱战", r#"["P5S", "女神异闻录5乱战", "P5乱战"]"#),
            ("Persona 4 Golden", "女神异闻录4黄金版", r#"["P4G", "女神异闻录4G", "P4黄金版"]"#),
            ("Shin Megami Tensei V: Vengeance", "真·女神转生5：复仇", r#"["真女神转生5", "SMT5", "复仇"]"#),
            ("Metaphor: ReFantazio", "暗喻幻想：ReFantazio", r#"["暗喻幻想", "ReFantazio", "隐喻"]"#),
            ("Final Fantasy VII Remake Intergrade", "最终幻想7重制版Intergrade", r#"["FF7RI", "FF7R Intergrade", "尤菲DLC"]"#),
            ("Final Fantasy XIV Online", "最终幻想14", r#"["最终幻想14", "FF14", "FFXIV"]"#),
            ("Final Fantasy Origin: Stranger of Paradise", "最终幻想起源：天堂的陌生人", r#"["天堂的陌生人", "FF起源", "Stranger of Paradise"]"#),
            ("Crisis Core: Final Fantasy VII Reunion", "核心危机：最终幻想7重聚", r#"["核心危机重聚", "CCFF7", "Crisis Core"]"#),
            ("Dragon Quest XI S: Echoes of an Elusive Age", "勇者斗恶龙11S", r#"["勇者斗恶龙11", "DQ11", "Dragon Quest 11"]"#),
            ("Dragon Quest Builders 2", "勇者斗恶龙：建造者2", r#"["勇者斗恶龙建造者2", "DQB2", "DQ建造者2"]"#),
            ("Tales of Arise", "破晓传奇", r#"["破晓传奇", "Tales of Arise", "传说系列"]"#),
            ("Scarlet Nexus", "绯红结系", r#"["绯红结系", "Scarlet Nexus"]"#),
            ("Code Vein", "噬血代码", r#"["噬血代码", "Code Vein"]"#),
            ("God Eater 3", "噬神者3", r#"["噬神者3", "God Eater 3", "GE3"]"#),
            ("Monster Hunter Stories 2: Wings of Ruin", "怪物猎人物语2", r#"["怪物猎人物语2", "MHS2", "物语2"]"#),
            ("Dauntless", "无畏", r#"["无畏", "Dauntless", "共斗游戏"]"#),
            ("Wild Hearts", "狂野之心", r#"["狂野之心", "Wild Hearts"]"#),
            ("Toukiden 2", "讨鬼传2", r#"["讨鬼传2", "Toukiden 2"]"#),
            ("NieR Replicant ver.1.22474487139...", "尼尔：人工生命", r#"["尼尔人工生命", "NieR Replicant", "尼尔1"]"#),
            ("Drakengard 3", "龙背上的骑兵3", r#"["龙背上的骑兵3", "Drakengard 3"]"#),
            ("Bayonetta 3", "猎天使魔女3", r#"["猎天使魔女3", "Bayonetta 3", "贝优妮塔3"]"#),
            ("Astral Chain", "异界锁链", r#"["异界锁链", "Astral Chain"]"#),
            ("Xenoblade Chronicles 3", "异度神剑3", r#"["异度神剑3", "Xenoblade 3", "XB3"]"#),
            ("Xenoblade Chronicles 2", "异度神剑2", r#"["异度神剑2", "Xenoblade 2", "XB2"]"#),
            ("Xenoblade Chronicles: Definitive Edition", "异度神剑：终极版", r#"["异度神剑终极版", "Xenoblade DE"]"#),
            ("Fire Emblem: Three Houses", "火焰纹章：风花雪月", r#"["火焰纹章风花雪月", "风花雪月", "FE Three Houses"]"#),
            ("Fire Emblem Engage", "火焰纹章：结合", r#"["火焰纹章结合", "Engage", "FE Engage"]"#),
            ("Shin Megami Tensei V", "真·女神转生5", r#"["真女神转生5", "SMT5"]"#),
            ("Bayonetta", "猎天使魔女", r#"["猎天使魔女", "Bayonetta", "贝优妮塔"]"#),
            ("Bayonetta 2", "猎天使魔女2", r#"["猎天使魔女2", "Bayonetta 2", "贝优妮塔2"]"#),
            ("The Wonderful 101: Remastered", "神奇101：重制版", r#"["神奇101", "Wonderful 101", "TW101"]"#),
            ("Sol Cresta", "索尔克雷斯塔", r#"["索尔克雷斯塔", "Sol Cresta"]"#),
            // 更多独立游戏
            ("Hollow Knight", "空洞骑士", r#"["空洞骑士", "Hollow Knight"]"#),
            ("Hades", "哈迪斯", r#"["哈迪斯", "Hades"]"#),
            ("Dead Cells", "死亡细胞", r#"["死亡细胞", "Dead Cells"]"#),
            ("Celeste", "蔚蓝", r#"["蔚蓝", "Celeste"]"#),
            ("Ori and the Blind Forest", "精灵与森林", r#"["精灵与森林", "Ori 1", "奥日1"]"#),
            ("Ori and the Will of the Wisps", "精灵与萤火意志", r#"["精灵与萤火意志", "Ori 2", "奥日2"]"#),
            ("Cuphead", "茶杯头", r#"["茶杯头", "Cuphead", "杯头"]"#),
            ("Hollow Knight: Silksong", "空洞骑士：丝之歌", r#"["空洞骑士丝之歌", "丝之歌", "Silksong"]"#),
            ("Disco Elysium: The Final Cut", "极乐迪斯科：最终剪辑版", r#"["极乐迪斯科最终版", "Disco Elysium Final Cut"]"#),
            ("Divinity: Original Sin 2", "神界：原罪2", r#"["神界原罪2", "Divinity OS2", "DOS2"]"#),
            ("Baldur's Gate 3", "博德之门3", r#"["博德之门", "BG3", "博德3"]"#),
            ("Pillars of Eternity II: Deadfire", "永恒之柱2：死火", r#"["永恒之柱2", "死火", "PoE2"]"#),
            ("Tyranny", "暴君", r#"["暴君", "Tyranny"]"#),
            ("Torment: Tides of Numenera", "折磨：纽曼诺拉之潮", r#"["纽曼诺拉之潮", "Torment"]"#),
            ("Wasteland 3", "废土3", r#"["废土3", "Wasteland 3"]"#),
            ("Darkest Dungeon", "暗黑地牢", r#"["暗黑地牢", "Darkest Dungeon"]"#),
            ("Slay the Spire", "杀戮尖塔", r#"["杀戮尖塔", "Slay the Spire", "爬塔"]"#),
            ("Monster Train", "怪物火车", r#"["怪物火车", "Monster Train"]"#),
            ("Inscryption", "邪恶冥刻", r#"["邪恶冥刻", "Inscryption"]"#),
            ("Loop Hero", "循环英雄", r#"["循环英雄", "Loop Hero"]"#),
            ("Vampire Survivors", "吸血鬼幸存者", r#"["吸血鬼幸存者", "Vampire Survivors"]"#),
            ("Brotato", "土豆兄弟", r#"["土豆兄弟", "Brotato"]"#),
            ("Gunfire Reborn", "枪火重生", r#"["枪火重生", "Gunfire Reborn"]"#),
            ("Risk of Rain 2", "雨中冒险2", r#"["雨中冒险2", "Risk of Rain 2", "RoR2"]"#),
            ("Deep Rock Galactic", "深岩银河", r#"["深岩银河", "Deep Rock Galactic", "DRG"]"#),
            ("Valheim", "英灵神殿", r#"["英灵神殿", "Valheim"]"#),
            ("Raft", "木筏求生", r#"["木筏求生", "Raft"]"#),
            ("Subnautica", "深海迷航", r#"["深海迷航", "Subnautica", "美丽水世界"]"#),
            ("Subnautica: Below Zero", "深海迷航：零度之下", r#"["深海迷航零度之下", "零度之下", "Below Zero"]"#),
            ("The Forest", "森林", r#"["森林", "The Forest"]"#),
            ("Sons of the Forest", "森林之子", r#"["森林之子", "Sons of the Forest"]"#),
            ("Green Hell", "绿色地狱", r#"["绿色地狱", "Green Hell"]"#),
            ("The Long Dark", "漫漫长夜", r#"["漫漫长夜", "The Long Dark"]"#),
            ("No Man's Sky", "无人深空", r#"["无人深空", "No Man's Sky"]"#),
            ("Sea of Thieves", "盗贼之海", r#"["盗贼之海", "Sea of Thieves"]"#),
            ("Elite Dangerous", "精英：危机", r#"["精英危机", "Elite Dangerous"]"#),
            ("Star Citizen", "星际公民", r#"["星际公民", "Star Citizen"]"#),
            // 体育和赛车游戏
            ("Forza Horizon 5", "极限竞速：地平线5", r#"["地平线5", "FH5", "极限竞速5"]"#),
            ("Forza Horizon 4", "极限竞速：地平线4", r#"["地平线4", "FH4", "极限竞速4"]"#),
            ("Forza Motorsport", "极限竞速", r#"["极限竞速", "FM", "Forza"]"#),
            ("Gran Turismo 7", "跑车浪漫旅7", r#"["跑车浪漫旅7", "GT7", "Gran Turismo 7"]"#),
            ("Need for Speed Heat", "极品飞车：热度", r#"["极品飞车热度", "热度", "NFS Heat"]"#),
            ("Need for Speed Unbound", "极品飞车：不羁", r#"["极品飞车不羁", "不羁", "NFS Unbound"]"#),
            ("Need for Speed: Most Wanted", "极品飞车：最高通缉", r#"["极品飞车最高通缉", "最高通缉", "NFS MW"]"#),
            ("Need for Speed: Payback", "极品飞车：复仇", r#"["极品飞车复仇", "复仇", "NFS Payback"]"#),
            ("F1 23", "F1 23", r#"["F123", "一级方程式23"]"#),
            ("F1 24", "F1 24", r#"["F124", "一级方程式24"]"#),
            ("FIFA 23", "FIFA 23", r#"["FIFA23", "国际足联23"]"#),
            ("EA Sports FC 24", "EA Sports FC 24", r#"["FC24", "FIFA24"]"#),
            ("EA Sports FC 25", "EA Sports FC 25", r#"["FC25", "FIFA25"]"#),
            ("NBA 2K24", "NBA 2K24", r#"["NBA2K24", "2K24"]"#),
            ("NBA 2K25", "NBA 2K25", r#"["NBA2K25", "2K25"]"#),
            ("Madden NFL 24", "麦登橄榄球24", r#"["Madden24", "橄榄球24"]"#),
            ("Madden NFL 25", "麦登橄榄球25", r#"["Madden25", "橄榄球25"]"#),
            ("WWE 2K24", "WWE 2K24", r#"["WWE2K24", "摔角2K24"]"#),
            ("UFC 5", "UFC 5", r#"["UFC5", "终极格斗冠军赛5"]"#),
            ("MLB The Show 24", "MLB The Show 24", r#"["MLB The Show 24", "棒球24"]"#),
            ("Tony Hawk's Pro Skater 1 + 2", "托尼·霍克职业滑板1+2", r#"["托尼霍克滑板", "THPS"]"#),
            ("Riders Republic", "极限共和国", r#"["极限共和国", "Riders Republic"]"#),
            ("Steep", "极限巅峰", r#"["极限巅峰", "Steep"]"#),
            ("Hot Wheels Unleashed", "风火轮：释放", r#"["风火轮释放", "Hot Wheels"]"#),
        ];

        let mapping_count = mappings.len();
        log::info!("准备插入 {} 条游戏名称映射", mapping_count);

        let tx = conn.transaction()?;
        let mut stmt = tx.prepare(
            "INSERT OR IGNORE INTO game_name_mapping (english_name, chinese_name, aliases)
             VALUES (?1, ?2, ?3)"
        )?;

        for (english, chinese, aliases) in mappings {
            stmt.execute(params![english, chinese, aliases])?;
        }

        drop(stmt);
        tx.commit()?;

        log::info!("游戏名称映射初始化完成，共 {} 条记录", mapping_count);
        Ok(())
    })
    .await
}

/// 获取游戏的中文名称（支持模糊匹配）
pub async fn get_chinese_game_name(english_name: String) -> Result<String> {
    println!("[DEBUG] get_chinese_game_name() 输入: {}", english_name);

    with_conn(move |conn| {
        println!("[DEBUG] 数据库连接成功，开始查询");
        log::debug!("查询中文名称: {}", english_name);

        // 1. 先尝试精确匹配
        let chinese_name = conn.query_row(
            "SELECT chinese_name FROM game_name_mapping WHERE english_name = ?1",
            params![english_name],
            |row| row.get(0),
        );

        if let Ok(name) = chinese_name {
            println!("[DEBUG] 找到精确匹配: {} -> {}", english_name, name);
            log::info!("找到精确中文映射");
            return Ok(name);
        }

        println!("[DEBUG] 未找到精确匹配，尝试大小写不敏感匹配");

        // 2. 尝试大小写不敏感匹配
        let chinese_name_case_insensitive = conn.query_row(
            "SELECT chinese_name FROM game_name_mapping WHERE LOWER(english_name) = LOWER(?1)",
            params![english_name],
            |row| row.get(0),
        );

        if let Ok(name) = chinese_name_case_insensitive {
            println!("[DEBUG] 找到大小写不敏感匹配: {} -> {}", english_name, name);
            log::info!("找到大小写不敏感映射");
            return Ok(name);
        }

        println!("[DEBUG] 未找到大小写匹配，尝试部分匹配");

        // 3. 尝试部分匹配（游戏名称包含在数据库名称中，或数据库名称包含在游戏名称中）
        let chinese_name_partial = conn.query_row(
            "SELECT chinese_name FROM game_name_mapping WHERE ?1 LIKE '%' || english_name || '%' OR english_name LIKE '%' || ?1 || '%'",
            params![english_name],
            |row| row.get(0),
        );

        if let Ok(name) = chinese_name_partial {
            println!("[DEBUG] 找到部分匹配: {} -> {}", english_name, name);
            log::info!("找到部分匹配映射");
            return Ok(name);
        }

        println!("[DEBUG] 未找到部分匹配，尝试别名查找");

        // 4. 尝试在别名中查找
        let chinese_name_alias = conn.query_row(
            "SELECT chinese_name FROM game_name_mapping WHERE aliases LIKE '%' || ?1 || '%'",
            params![english_name],
            |row| row.get(0),
        );

        if let Ok(name) = chinese_name_alias {
            println!("[DEBUG] 找到别名匹配: {} -> {}", english_name, name);
            log::info!("找到别名匹配映射");
            return Ok(name);
        }

        println!("[DEBUG] 所有匹配方式都失败，返回原名: {}", english_name);
        log::debug!("未找到任何映射");
        Ok(english_name) // 如果没有映射，返回原名
    })
    .await
}

/// 根据中文名称或别名获取英文名称（用于反向搜索）
pub async fn get_english_game_name(chinese_or_alias: String) -> Result<String> {
    with_conn(move |conn| {
        log::debug!("反向查询英文名称: {}", chinese_or_alias);

        // 1. 尝试中文名称精确匹配
        let english_name = conn.query_row(
            "SELECT english_name FROM game_name_mapping WHERE chinese_name = ?1",
            params![chinese_or_alias],
            |row| row.get(0),
        );

        if let Ok(name) = english_name {
            log::info!("找到中文到英文的精确映射: {} -> {}", chinese_or_alias, name);
            return Ok(name);
        }

        // 2. 尝试中文名称部分匹配
        let english_name_partial = conn.query_row(
            "SELECT english_name FROM game_name_mapping WHERE chinese_name LIKE '%' || ?1 || '%'",
            params![chinese_or_alias],
            |row| row.get(0),
        );

        if let Ok(name) = english_name_partial {
            log::info!("找到中文到英文的部分匹配: {} -> {}", chinese_or_alias, name);
            return Ok(name);
        }

        // 3. 尝试在别名中查找
        let english_name_alias = conn.query_row(
            "SELECT english_name FROM game_name_mapping WHERE aliases LIKE '%' || ?1 || '%'",
            params![chinese_or_alias],
            |row| row.get(0),
        );

        if let Ok(name) = english_name_alias {
            log::info!("找到别名到英文的映射: {} -> {}", chinese_or_alias, name);
            return Ok(name);
        }

        // 4. 检查输入是否已经是英文名称
        let english_name_direct = conn.query_row(
            "SELECT english_name FROM game_name_mapping WHERE english_name = ?1",
            params![chinese_or_alias],
            |row| row.get(0),
        );

        if let Ok(name) = english_name_direct {
            log::info!("输入本身就是英文名称: {}", name);
            return Ok(name);
        }

        log::debug!("未找到反向映射，返回原始输入");
        Ok(chinese_or_alias) // 如果没有映射，返回原始输入
    })
    .await
}

/// 批量转换游戏名称为中文
pub async fn translate_trainer_names(trainers: Vec<Trainer>) -> Result<Vec<Trainer>> {
    println!("[DEBUG] translate_trainer_names() 开始，共 {} 个", trainers.len());
    log::info!("开始批量转换 {} 个修改器名称", trainers.len());
    let mut translated_trainers = Vec::with_capacity(trainers.len());

    for mut trainer in trainers {
        println!("[DEBUG] 处理: {} (id: {})", trainer.name, trainer.id);
        let chinese_name = get_chinese_game_name(trainer.name.clone()).await?;
        println!("[DEBUG] 查询结果: {} -> {}", trainer.name, chinese_name);
        if chinese_name != trainer.name {
            log::info!("映射名称: {} -> {}", trainer.name, chinese_name);
            println!("[DEBUG] 映射成功: {} -> {}", trainer.name, chinese_name);
        } else {
            println!("[DEBUG] 未找到映射，保持原名: {}", trainer.name);
        }
        trainer.name = chinese_name;
        translated_trainers.push(trainer);
    }

    log::info!("批量转换完成，共转换 {} 个名称", translated_trainers.len());
    println!("[DEBUG] translate_trainer_names() 完成");
    Ok(translated_trainers)
}
