use crate::api::error::AppResult;
use crate::api::trainer::PaginatedResponse;
use crate::models::trainer::{Trainer, TrainerInstallInfo};
use crate::services::download_manager;
use crate::services::scraper;
use crate::services::storage;
use crate::utils::path::sanitize_filename;
use crate::utils::zip::extract_zip;
use crate::utils::http::HTTP_CLIENT;
use chrono::Local;
use std::fs;
use std::io::Read;
use std::io::Write;
use std::path::PathBuf;
use std::ptr;
use tauri::Emitter;
#[cfg(target_os = "windows")]
use windows_sys::Win32::UI::Shell::ShellExecuteW;
#[cfg(target_os = "windows")]
use windows_sys::Win32::UI::WindowsAndMessaging::SW_SHOW;

pub async fn fetch_trainers(page: u32) -> AppResult<PaginatedResponse<Trainer>> {
    let url = format!("https://flingtrainer.com/page/{}/", page);
    let response = HTTP_CLIENT.get(&url).send().await?;
    let html = response.text().await?;
    let trainers = scraper::parse_trainer_list(&html)?;

    // 转换为中文名称
    let trainers = storage::translate_trainer_names(trainers).await?;

    let total = 120;

    Ok(PaginatedResponse { trainers, total })
}

pub async fn search_trainers(query: String, page: u32) -> AppResult<PaginatedResponse<Trainer>> {
    log::info!("搜索请求: '{}', 页码: {}", query, page);

    // 尝试将中文查询转换为英文（用于网站搜索）
    let english_query = storage::get_english_game_name(query.clone()).await?;
    if english_query != query {
        log::info!("转换搜索词: '{}' -> '{}'", query, english_query);
    }

    let url = format!("https://flingtrainer.com/page/{}/?s={}", page, english_query);
    let response = HTTP_CLIENT.get(&url).send().await?;
    let html = response.text().await?;
    let trainers = scraper::parse_trainer_list(&html)?;

    // 转换为中文名称
    let trainers = storage::translate_trainer_names(trainers).await?;

    let total = trainers.len() as u32;

    log::info!("搜索完成，找到 {} 个结果", trainers.len());
    Ok(PaginatedResponse { trainers, total })
}

pub async fn get_trainer_detail(id: String) -> AppResult<Trainer> {
    let url = format!("https://flingtrainer.com/trainer/{}/", id);
    let response = HTTP_CLIENT.get(&url).send().await?;
    let html = response.text().await?;
    let mut trainer = scraper::parse_trainer_detail(&html)?;

    // 转换为中文名称
    let chinese_name = storage::get_chinese_game_name(trainer.name.clone()).await?;
    if chinese_name != trainer.name {
        log::info!("映射名称: {} -> {}", trainer.name, chinese_name);
    }
    trainer.name = chinese_name;

    Ok(trainer)
}

pub async fn download_trainer<R: tauri::Runtime>(
    app_handle: tauri::AppHandle<R>,
    mut trainer: Trainer,
) -> AppResult<Trainer> {
    // 先翻译游戏名称为中文
    let chinese_name = storage::get_chinese_game_name(trainer.name.clone()).await?;
    if chinese_name != trainer.name {
        println!("翻译游戏名称: {} -> {}", trainer.name, chinese_name);
    }
    trainer.name = chinese_name.clone();

    println!(
        "开始下载修改器: {} ({})",
        trainer.name, trainer.download_url
    );

    let download_dir = crate::services::settings::get_download_path()?;
    fs::create_dir_all(&download_dir)?;

    // 生成标准化的修改器目录名
    let safe_name = sanitize_filename(&trainer.name);
    let trainer_dir_name = format!("{}_{}", safe_name, trainer.id);
    let final_dir = download_dir.join(&trainer_dir_name);

    // 使用临时目录，确保失败不污染正式目录
    let staging_dir = download_dir.join(format!(
        "._tmp_{}_{}",
        trainer.id,
        chrono::Utc::now().timestamp_millis()
    ));
    if staging_dir.exists() {
        fs::remove_dir_all(&staging_dir)?;
    }
    fs::create_dir_all(&staging_dir)?;

    // 临时zip文件
    let temp_zip = staging_dir.join("package.zip");
    if temp_zip.exists() {
        fs::remove_file(&temp_zip)?;
    }

    // 使用下载管理器下载文件
    download_manager::download_file_with_progress(
        app_handle.clone(),
        &trainer.download_url,
        &trainer.id,
        &temp_zip,
        "download-progress",
    )
    .await?;

    // 验证临时文件
    if !temp_zip.exists() || fs::metadata(&temp_zip)?.len() == 0 {
        return Err(std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "Failed to create temporary file",
        )
        .into());
    }

    // 检查文件类型
    let is_zip_file = is_zip_file(&temp_zip);
    let is_exe_file = is_exe_file(&temp_zip);

    println!("文件类型检测: ZIP={}, EXE={}", is_zip_file, is_exe_file);

    // 发送处理进度
    let _ = app_handle.emit(
        "download-progress",
        serde_json::json!({
            "trainer_id": trainer.id,
            "status": "processing",
            "progress": 100.0,
            "downloaded_bytes": fs::metadata(&temp_zip)?.len(),
            "total_bytes": fs::metadata(&temp_zip)?.len(),
            "speed": null
        }),
    );

    // 处理下载的文件
    if is_zip_file {
        // 如果是ZIP文件，解压
        println!("检测到ZIP文件，开始解压...");
        if let Err(e) = extract_zip(&temp_zip, &staging_dir) {
            println!("解压失败: {}", e);
            let _ = fs::remove_dir_all(&staging_dir);
            let _ = fs::remove_file(&temp_zip);
            return Err(e);
        }
        // 解压后删除临时文件
        fs::remove_file(&temp_zip)?;
    } else if is_exe_file {
        // 如果是EXE文件，直接移动到目标目录
        println!("检测到EXE文件，直接使用...");
        let exe_filename = format!("{}.exe", trainer.id);
        let target_exe_path = staging_dir.join(&exe_filename);
        fs::rename(&temp_zip, &target_exe_path)?;
    } else {
        // 未知文件类型，尝试作为ZIP处理
        println!("未知文件类型，尝试作为ZIP处理...");
        if let Err(e) = extract_zip(&temp_zip, &staging_dir) {
            println!("解压失败，尝试直接复制文件: {}", e);
            // 解压失败，直接复制文件到目标目录
            let target_file_path = staging_dir.join(format!("unknown_file_{}.bin", trainer.id));
            fs::copy(&temp_zip, &target_file_path)?;
            fs::remove_file(&temp_zip)?;
        } else {
            // 解压成功，删除临时文件
            fs::remove_file(&temp_zip)?;
        }
    }

    // 处理目录切换，先备份旧目录，确保状态可恢复
    let backup_dir = download_dir.join(format!("{}_backup", trainer_dir_name));
    if backup_dir.exists() {
        fs::remove_dir_all(&backup_dir)?;
    }
    if final_dir.exists() {
        fs::rename(&final_dir, &backup_dir)?;
    }

    if let Err(rename_err) = fs::rename(&staging_dir, &final_dir) {
        // 恢复旧目录
        if backup_dir.exists() {
            let _ = fs::rename(&backup_dir, &final_dir);
        }
        return Err(std::io::Error::new(
            std::io::ErrorKind::Other,
            format!("切换修改器目录失败: {}", rename_err),
        )
        .into());
    }
    // 清理备份
    if backup_dir.exists() {
        let _ = fs::remove_dir_all(&backup_dir);
    }

    // 保存安装信息（文件 + 数据库）- 保持英文名称
    let install_time = Local::now().to_rfc3339();
    let install_info = TrainerInstallInfo {
        trainer: trainer.clone(),
        install_path: final_dir.to_string_lossy().to_string(),
        install_time: install_time.clone(),
        last_launch_time: None,
    };

    let info_json = serde_json::to_string_pretty(&install_info)?;
    let mut info_file = fs::File::create(final_dir.join("trainer.json"))?;
    info_file.write_all(info_json.as_bytes())?;

    // 保存到数据库（保持英文名称，显示时再翻译）
    let installed_record = crate::models::trainer::InstalledTrainer {
        id: trainer.id.clone(),
        name: trainer.name.clone(), // 保存英文名称
        version: trainer.version.clone(),
        game_version: trainer.game_version.clone(),
        download_url: trainer.download_url.clone(),
        description: trainer.description.clone(),
        thumbnail: trainer.thumbnail.clone(),
        download_count: trainer.download_count,
        last_update: trainer.last_update.clone(),
        installed_path: final_dir.to_string_lossy().to_string(),
        install_time: install_time.clone(),
        last_launch_time: String::new(),
    };

    storage::upsert_installed_trainer(installed_record).await?;
    storage::upsert_downloaded_trainer(trainer.clone()).await?;

    println!("修改器安装成功: {:?}", final_dir);

    Ok(trainer) // 返回翻译后的trainer对象
}

fn is_zip_file(path: &PathBuf) -> bool {
    if let Ok(mut file) = fs::File::open(path) {
        let mut buffer = [0u8; 4];
        if file.read_exact(&mut buffer).is_ok() {
            // ZIP 文件的魔数是 0x504B0304
            return buffer == [0x50, 0x4B, 0x03, 0x04];
        }
    }
    false
}

fn is_exe_file(path: &PathBuf) -> bool {
    if let Ok(mut file) = fs::File::open(path) {
        let mut buffer = [0u8; 2];
        if file.read_exact(&mut buffer).is_ok() {
            // EXE 文件的魔数是 0x4D5A (MZ)
            return buffer == [0x4D, 0x5A];
        }
    }
    false
}

pub async fn delete_trainer(trainer_id: String) -> AppResult<()> {
    let trainer_record = storage::get_installed_trainer_by_id(&trainer_id).await?;

    if let Some(record) = trainer_record {
        let trainer_dir = PathBuf::from(record.installed_path);
        if trainer_dir.exists() {
            fs::remove_dir_all(&trainer_dir)?;
        }
        storage::remove_installed_trainer(&trainer_id).await?;
        storage::remove_downloaded_trainer(&trainer_id).await?;
    }

    Ok(())
}

pub async fn launch_trainer(trainer_id: String) -> AppResult<()> {
    let trainer_record = storage::get_installed_trainer_by_id(&trainer_id).await?;

    let trainer_dir = if let Some(record) = trainer_record {
        PathBuf::from(record.installed_path)
    } else {
        // 兼容旧数据：从文件系统扫描
        let download_dir = crate::services::settings::get_download_path()?;
        let mut found_dir = None;

        if let Ok(entries) = fs::read_dir(&download_dir) {
            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_dir() {
                    let info_path = path.join("trainer.json");
                    if info_path.exists() {
                        if let Ok(info_content) = fs::read_to_string(&info_path) {
                            if let Ok(install_info) =
                                serde_json::from_str::<TrainerInstallInfo>(&info_content)
                            {
                                if install_info.trainer.id == trainer_id {
                                    found_dir = Some(path);
                                    break;
                                }
                            }
                        }
                    }
                }
            }
        }

        found_dir.ok_or_else(|| {
            std::io::Error::new(std::io::ErrorKind::NotFound, "修改器目录未找到")
        })?
    };

    if !trainer_dir.exists() {
        return Err(std::io::Error::new(
            std::io::ErrorKind::NotFound,
            "修改器目录不存在",
        )
        .into());
    }

    // 查找 EXE 文件
    let mut executable_path = None;
    if let Ok(entries) = fs::read_dir(&trainer_dir) {
        for entry in entries.flatten() {
            let path = entry.path();
            if let Some(ext) = path.extension() {
                if ext == "exe" {
                    executable_path = Some(path);
                    break;
                }
            }
        }
    }

    let executable_path =
        executable_path.ok_or_else(|| std::io::Error::new(std::io::ErrorKind::NotFound, "修改器可执行文件未找到"))?;

    println!("启动修改器: {:?}", executable_path);

    // 使用 Windows ShellExecuteW 启动
    #[cfg(target_os = "windows")]
    {
        use std::ffi::OsStr;
        use std::os::windows::ffi::OsStrExt;

        let exe_path_wide: Vec<u16> = OsStr::new(&executable_path)
            .encode_wide()
            .chain(std::iter::once(0))
            .collect();

        let result = unsafe {
            ShellExecuteW(
                0,
                ptr::null_mut(),
                exe_path_wide.as_ptr(),
                ptr::null_mut(),
                ptr::null_mut(),
                SW_SHOW,
            )
        };

        if result as i32 <= 32 {
            return Err(std::io::Error::new(
                std::io::ErrorKind::Other,
                format!("启动修改器失败，错误代码: {}", result),
            )
            .into());
        }
    }

    // 更新最后启动时间
    let now = Local::now().to_rfc3339();
    storage::update_last_launch_time(&trainer_id, &now).await?;

    Ok(())
}
