use crate::api::error::{AppError, AppResult};
use crate::utils::http::HTTP_CLIENT;
use chrono::{DateTime, Utc};
use rusqlite::OptionalExtension;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::fs;
use std::io::Write;
use std::path::PathBuf;
use std::time::Duration;

/// Image cache metadata stored in the database
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CachedImage {
    pub url: String,
    pub local_path: String,
    pub cache_time: String,
    pub last_access_time: String,
    pub file_size: u64,
    pub content_type: Option<String>,
}

/// Cache configuration
const CACHE_DIR_NAME: &str = "image_cache";
const MAX_CACHE_SIZE_MB: u64 = 500; // 500MB max cache size
const CACHE_EXPIRATION_DAYS: i64 = 30; // 30 days expiration

/// Get the image cache directory path
fn get_cache_dir() -> AppResult<PathBuf> {
    let cache_dir = directories::ProjectDirs::from("com", "gamemodmaster", "GameModMaster")
        .map(|dirs| dirs.cache_dir().join(CACHE_DIR_NAME))
        .ok_or_else(|| {
            AppError::IoError(
                std::io::Error::new(
                    std::io::ErrorKind::NotFound,
                    "Cannot determine cache directory",
                )
                .to_string(),
            )
        })?;

    // Create cache directory if it doesn't exist
    if !cache_dir.exists() {
        fs::create_dir_all(&cache_dir)?;
    }

    Ok(cache_dir)
}

/// Generate a unique filename for a URL using SHA256 hash
fn generate_cache_filename(url: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(url.as_bytes());
    let hash = hasher.finalize();
    format!("{:x}", hash)
}

/// Infer file extension from URL or content type
fn infer_extension(url: &str, content_type: Option<&str>) -> String {
    // Try to get extension from URL
    if let Some(ext) = url.split('?').next().and_then(|s| s.rsplit('.').next()) {
        match ext.to_lowercase().as_str() {
            "jpg" | "jpeg" | "png" | "gif" | "webp" | "bmp" | "svg" => return format!(".{}", ext),
            _ => {}
        }
    }

    // Try to get extension from content type
    if let Some(ct) = content_type {
        match ct.to_lowercase().as_str() {
            "image/jpeg" | "image/jpg" => return ".jpg".to_string(),
            "image/png" => return ".png".to_string(),
            "image/gif" => return ".gif".to_string(),
            "image/webp" => return ".webp".to_string(),
            "image/bmp" => return ".bmp".to_string(),
            "image/svg+xml" => return ".svg".to_string(),
            _ => {}
        }
    }

    // Default to .jpg if unknown
    ".jpg".to_string()
}

/// Get or download an image, returning the local cached path
#[tauri::command]
pub async fn get_cached_image(url: String) -> AppResult<String> {
    // Validate URL
    if url.is_empty() {
        return Err(AppError::ValidationError("Image URL cannot be empty".to_string()));
    }

    // Only allow http/https URLs
    if !url.starts_with("http://") && !url.starts_with("https://") {
        return Err(AppError::ValidationError("Only HTTP/HTTPS URLs are supported".to_string()));
    }

    let cache_dir = get_cache_dir()?;
    let filename = generate_cache_filename(&url);

    // Check database for existing cache entry
    if let Ok(Some(cached)) = get_cached_image_from_db(&url).await {
        let local_path = PathBuf::from(&cached.local_path);

        // Check if file exists
        if local_path.exists() {
            // Update last access time
            let _ = update_last_access_time(&url).await;

            // Convert to file URL that can be used by frontend
            return Ok(convert_to_file_url(&local_path));
        }
    }

    // Download image if not cached or cache is invalid
    download_and_cache_image(&url, &cache_dir, &filename).await
}

/// Download and cache an image
async fn download_and_cache_image(
    url: &str,
    cache_dir: &PathBuf,
    filename: &str,
) -> AppResult<String> {
    println!("Downloading image: {}", url);

    // Download image with timeout
    let response = HTTP_CLIENT
        .get(url)
        .timeout(Duration::from_secs(30))
        .send()
        .await
        .map_err(|e| AppError::DownloadError(format!("Failed to download image: {}", e)))?;

    if !response.status().is_success() {
        return Err(AppError::DownloadError(format!(
            "Failed to download image, status: {}",
            response.status()
        )));
    }

    // Get content type
    let content_type = response
        .headers()
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .map(|s| s.to_string());

    // Get image data
    let image_data = response
        .bytes()
        .await
        .map_err(|e| AppError::DownloadError(format!("Failed to read image data: {}", e)))?;

    // Determine file extension
    let extension = infer_extension(url, content_type.as_deref());
    let full_filename = format!("{}{}", filename, extension);
    let local_path = cache_dir.join(&full_filename);

    // Write to file
    let mut file = fs::File::create(&local_path)?;
    file.write_all(&image_data)?;
    file.flush()?;

    let file_size = image_data.len() as u64;

    // Save metadata to database
    let cached_image = CachedImage {
        url: url.to_string(),
        local_path: local_path.to_string_lossy().to_string(),
        cache_time: Utc::now().to_rfc3339(),
        last_access_time: Utc::now().to_rfc3339(),
        file_size,
        content_type,
    };

    save_cached_image_to_db(cached_image).await?;

    println!("Image cached successfully: {:?}", local_path);

    // Convert to file URL
    Ok(convert_to_file_url(&local_path))
}

/// Convert local path to file URL for frontend use
fn convert_to_file_url(path: &PathBuf) -> String {
    // For Tauri's asset protocol, we use the convertFileSrc from frontend
    // Here we just return the path and let frontend handle the conversion
    path.to_string_lossy().to_string()
}

/// Clean up old cached images
#[tauri::command]
pub async fn cleanup_image_cache() -> AppResult<CleanupResult> {
    println!("Starting image cache cleanup...");

    let cache_dir = get_cache_dir()?;
    let expiration_date = Utc::now() - chrono::Duration::days(CACHE_EXPIRATION_DAYS);

    // Get all cached images from database
    let cached_images = get_all_cached_images().await?;

    let mut deleted_count = 0;
    let mut deleted_size = 0u64;
    let mut errors = Vec::new();

    for cached in cached_images {
        // Check if expired
        let cache_time = match DateTime::parse_from_rfc3339(&cached.cache_time) {
            Ok(dt) => dt.with_timezone(&Utc),
            Err(_) => continue,
        };

        if cache_time < expiration_date {
            // Delete file
            let local_path = PathBuf::from(&cached.local_path);
            if local_path.exists() {
                match fs::remove_file(&local_path) {
                    Ok(_) => {
                        deleted_size += cached.file_size;
                        deleted_count += 1;

                        // Remove from database
                        let _ = remove_cached_image_from_db(&cached.url).await;
                    }
                    Err(e) => {
                        errors.push(format!("Failed to delete {:?}: {}", local_path, e));
                    }
                }
            } else {
                // File doesn't exist, just remove from database
                let _ = remove_cached_image_from_db(&cached.url).await;
            }
        }
    }

    // Check total cache size and enforce limit
    let total_size = calculate_cache_size(&cache_dir)?;
    if total_size > MAX_CACHE_SIZE_MB * 1024 * 1024 {
        // Delete oldest files until under limit
        enforce_cache_size_limit(&cache_dir).await?;
    }

    let result = CleanupResult {
        deleted_count,
        deleted_size_bytes: deleted_size,
        errors,
    };

    println!("Cache cleanup completed: {:?}", result);

    Ok(result)
}

#[derive(Debug, Serialize)]
pub struct CleanupResult {
    pub deleted_count: u32,
    pub deleted_size_bytes: u64,
    pub errors: Vec<String>,
}

/// Calculate total cache size
fn calculate_cache_size(cache_dir: &PathBuf) -> AppResult<u64> {
    let mut total_size = 0u64;

    if cache_dir.exists() {
        for entry in fs::read_dir(cache_dir)? {
            let entry = entry?;
            if entry.file_type()?.is_file() {
                total_size += entry.metadata()?.len();
            }
        }
    }

    Ok(total_size)
}

/// Enforce cache size limit by deleting oldest files
async fn enforce_cache_size_limit(cache_dir: &PathBuf) -> AppResult<()> {
    let max_size = MAX_CACHE_SIZE_MB * 1024 * 1024;
    let mut cached_images = get_all_cached_images().await?;

    // Sort by last access time (oldest first)
    cached_images.sort_by(|a, b| {
        let time_a = DateTime::parse_from_rfc3339(&a.last_access_time)
            .map(|dt| dt.timestamp())
            .unwrap_or(0);
        let time_b = DateTime::parse_from_rfc3339(&b.last_access_time)
            .map(|dt| dt.timestamp())
            .unwrap_or(0);
        time_a.cmp(&time_b)
    });

    let mut current_size = calculate_cache_size(cache_dir)?;

    for cached in cached_images {
        if current_size <= max_size {
            break;
        }

        let local_path = PathBuf::from(&cached.local_path);
        if local_path.exists() {
            match fs::remove_file(&local_path) {
                Ok(_) => {
                    current_size -= cached.file_size;
                    let _ = remove_cached_image_from_db(&cached.url).await;
                }
                Err(_) => {}
            }
        }
    }

    Ok(())
}

/// Get cache statistics
#[tauri::command]
pub async fn get_cache_stats() -> AppResult<CacheStats> {
    let cache_dir = get_cache_dir()?;

    let cached_images = get_all_cached_images().await?;
    let total_size = calculate_cache_size(&cache_dir)?;

    let file_count = cached_images.len() as u32;

    Ok(CacheStats {
        file_count,
        total_size_bytes: total_size,
        max_size_bytes: MAX_CACHE_SIZE_MB * 1024 * 1024,
        cache_dir: cache_dir.to_string_lossy().to_string(),
    })
}

#[derive(Debug, Serialize)]
pub struct CacheStats {
    pub file_count: u32,
    pub total_size_bytes: u64,
    pub max_size_bytes: u64,
    pub cache_dir: String,
}

/// Clear all cached images
#[tauri::command]
pub async fn clear_image_cache() -> AppResult<u32> {
    let _cache_dir = get_cache_dir()?;
    let cached_images = get_all_cached_images().await?;

    let mut deleted_count = 0;

    for cached in cached_images {
        let local_path = PathBuf::from(&cached.local_path);
        if local_path.exists() {
            match fs::remove_file(&local_path) {
                Ok(_) => {
                    deleted_count += 1;
                }
                Err(_) => {}
            }
        }
    }

    // Clear database
    clear_all_cached_images_db().await?;

    Ok(deleted_count)
}

// Database operations (these would integrate with your existing storage system)

async fn get_cached_image_from_db(url: &str) -> AppResult<Option<CachedImage>> {
    let url = url.to_string();
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;
        let query = "SELECT url, local_path, cache_time, last_access_time, file_size, content_type FROM image_cache WHERE url = ?";

        let mut stmt = db.prepare(query)
            .map_err(|e| AppError::DatabaseError(format!("Failed to prepare query: {}", e)))?;

        let result = stmt
            .query_row(rusqlite::params![url], |row| {
                Ok(CachedImage {
                    url: row.get(0)?,
                    local_path: row.get(1)?,
                    cache_time: row.get(2)?,
                    last_access_time: row.get(3)?,
                    file_size: row.get(4)?,
                    content_type: row.get(5)?,
                })
            })
            .optional()
            .map_err(|e| AppError::DatabaseError(format!("Failed to query cached image: {}", e)))?;

        Ok(result)
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}

async fn save_cached_image_to_db(cached: CachedImage) -> AppResult<()> {
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;

        // Create table if not exists
        db.execute(
            "CREATE TABLE IF NOT EXISTS image_cache (
            url TEXT PRIMARY KEY,
            local_path TEXT NOT NULL,
            cache_time TEXT NOT NULL,
            last_access_time TEXT NOT NULL,
            file_size INTEGER NOT NULL,
            content_type TEXT
        )",
            [],
        )
        .map_err(|e| AppError::DatabaseError(format!("Failed to create image_cache table: {}", e)))?;

        // Insert or replace
        let query = "INSERT OR REPLACE INTO image_cache (url, local_path, cache_time, last_access_time, file_size, content_type) VALUES (?, ?, ?, ?, ?, ?)";

        db.execute(
            query,
            rusqlite::params![
                cached.url,
                cached.local_path,
                cached.cache_time,
                cached.last_access_time,
                cached.file_size,
                cached.content_type,
            ],
        )
        .map_err(|e| AppError::DatabaseError(format!("Failed to save cached image: {}", e)))?;

        Ok(())
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}

async fn update_last_access_time(url: &str) -> AppResult<()> {
    let url = url.to_string();
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;
        let query = "UPDATE image_cache SET last_access_time = ? WHERE url = ?";
        let now = Utc::now().to_rfc3339();

        db.execute(query, rusqlite::params![now, url])
            .map_err(|e| AppError::DatabaseError(format!("Failed to update access time: {}", e)))?;

        Ok(())
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}

async fn get_all_cached_images() -> AppResult<Vec<CachedImage>> {
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;
        let query = "SELECT url, local_path, cache_time, last_access_time, file_size, content_type FROM image_cache";

        let mut stmt = db.prepare(query)
            .map_err(|e| AppError::DatabaseError(format!("Failed to prepare query: {}", e)))?;

        let results = stmt
            .query_map([], |row| {
                Ok(CachedImage {
                    url: row.get(0)?,
                    local_path: row.get(1)?,
                    cache_time: row.get(2)?,
                    last_access_time: row.get(3)?,
                    file_size: row.get(4)?,
                    content_type: row.get(5)?,
                })
            })
            .map_err(|e| AppError::DatabaseError(format!("Failed to query cached images: {}", e)))?
            .collect::<Result<Vec<_>, _>>()
            .map_err(|e| AppError::DatabaseError(format!("Failed to collect cached images: {}", e)))?;

        Ok(results)
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}

async fn remove_cached_image_from_db(url: &str) -> AppResult<()> {
    let url = url.to_string();
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;
        let query = "DELETE FROM image_cache WHERE url = ?";

        db.execute(query, rusqlite::params![url])
            .map_err(|e| AppError::DatabaseError(format!("Failed to delete cached image: {}", e)))?;

        Ok(())
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}

async fn clear_all_cached_images_db() -> AppResult<()> {
    tauri::async_runtime::spawn_blocking(move || {
        let db = crate::services::storage::get_conn()?;
        let query = "DELETE FROM image_cache";

        db.execute(query, [])
            .map_err(|e| AppError::DatabaseError(format!("Failed to clear image cache: {}", e)))?;

        Ok(())
    })
    .await
    .map_err(|e| AppError::DatabaseError(format!("Database task failed: {}", e)))?
}
