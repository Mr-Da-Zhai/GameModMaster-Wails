use chrono::Local;
use log::{error, warn};
use serde::{Serialize, Serializer};
use thiserror::Error;
use zip::result::ZipError;

// 错误代码枚举，便于前端进行错误处理
#[derive(Debug, Serialize, Clone, Copy)]
pub enum ErrorCode {
    Network = 1000,
    Parse = 2000,
    IO = 3000,
    Download = 4000,
    Config = 5000,
    Json = 6000,
    Zip = 7000,
    Validation = 8000,
    NotFound = 9000,
    Permission = 10000,
    Execution = 11000,
    Database = 12000,
    Unknown = 99999,
}

// 添加注解抑制未使用变体的警告
#[allow(dead_code)]
#[derive(Debug, Error)]
pub enum AppError {
    #[error("网络请求失败: {0}")]
    RequestError(String),

    #[error("HTML解析失败: {0}")]
    ParseError(String),

    #[error("IO操作错误: {0}")]
    IoError(String),

    #[error("下载错误: {0}")]
    DownloadError(String),

    #[error("配置错误: {0}")]
    ConfigError(String),

    #[error("JSON解析错误: {0}")]
    JsonError(String),

    #[error("压缩文件错误: {0}")]
    ZipError(String),

    #[error("验证失败: {0}")]
    ValidationError(String),

    #[error("资源未找到: {0}")]
    NotFoundError(String),

    #[error("权限错误: {0}")]
    PermissionError(String),

    #[error("执行错误: {0}")]
    ExecutionError(String),

    #[error("数据库错误: {0}")]
    DatabaseError(String),

    #[error("未知错误: {0}")]
    UnknownError(String),
}

// 手动实现 From<reqwest::Error>
impl From<reqwest::Error> for AppError {
    fn from(err: reqwest::Error) -> Self {
        AppError::RequestError(err.to_string())
    }
}

// 手动实现 From<std::io::Error>
impl From<std::io::Error> for AppError {
    fn from(err: std::io::Error) -> Self {
        AppError::IoError(err.to_string())
    }
}

// 手动实现 From<serde_json::Error>
impl From<serde_json::Error> for AppError {
    fn from(err: serde_json::Error) -> Self {
        AppError::JsonError(err.to_string())
    }
}

// 手动实现 From<ZipError>
impl From<ZipError> for AppError {
    fn from(err: ZipError) -> Self {
        AppError::ZipError(err.to_string())
    }
}

// 实现 From<anyhow::Error> 以支持 storage 模块的错误转换
impl From<anyhow::Error> for AppError {
    fn from(err: anyhow::Error) -> Self {
        // 尝试向下转换为已知的错误类型
        if let Some(io_err) = err.downcast_ref::<std::io::Error>() {
            return AppError::IoError(io_err.to_string());
        }
        if let Some(json_err) = err.downcast_ref::<serde_json::Error>() {
            return AppError::JsonError(json_err.to_string());
        }
        if let Some(zip_err) = err.downcast_ref::<ZipError>() {
            return AppError::ZipError(zip_err.to_string());
        }

        // 默认转换为数据库错误
        AppError::DatabaseError(err.to_string())
    }
}

// 错误详细信息
#[allow(dead_code)]
#[derive(Debug, Clone)]
pub struct ErrorDetails {
    pub custom_message: Option<String>,
    pub time: String,
    pub source: Option<String>,
}

impl Default for ErrorDetails {
    fn default() -> Self {
        Self {
            custom_message: None,
            time: Local::now().format("%Y-%m-%d %H:%M:%S").to_string(),
            source: None,
        }
    }
}

impl AppError {
    // 获取错误代码
    pub fn error_code(&self) -> ErrorCode {
        match self {
            AppError::RequestError(_) => ErrorCode::Network,
            AppError::ParseError(_) => ErrorCode::Parse,
            AppError::IoError(_) => ErrorCode::IO,
            AppError::DownloadError(_) => ErrorCode::Download,
            AppError::ConfigError(_) => ErrorCode::Config,
            AppError::JsonError(_) => ErrorCode::Json,
            AppError::ZipError(_) => ErrorCode::Zip,
            AppError::ValidationError(_) => ErrorCode::Validation,
            AppError::NotFoundError(_) => ErrorCode::NotFound,
            AppError::PermissionError(_) => ErrorCode::Permission,
            AppError::ExecutionError(_) => ErrorCode::Execution,
            AppError::DatabaseError(_) => ErrorCode::Database,
            AppError::UnknownError(_) => ErrorCode::Unknown,
        }
    }

    // 添加自定义错误详细信息
    #[allow(dead_code)]
    pub fn with_details(self, details: &str) -> Self {
        // 记录错误详情
        error!("[错误详情] {}: {}", self, details);

        match self {
            AppError::ZipError(_) => {
                // 对于ZIP错误，返回一个新的下载错误，带有自定义消息
                AppError::DownloadError(details.to_string())
            }
            _ => self, // 其他错误类型暂时不处理
        }
    }

    // 获取用户友好的错误信息
    pub fn user_message(&self) -> String {
        match self {
            AppError::RequestError(_) => "网络连接异常，请检查您的网络设置后重试".to_string(),
            AppError::ParseError(_) => "数据解析失败，网站结构可能已更改".to_string(),
            AppError::IoError(_) => "文件读写错误，请检查磁盘空间和权限".to_string(),
            AppError::DownloadError(_) => "下载失败，请稍后重试".to_string(),
            AppError::ConfigError(_) => "配置错误，应用程序设置可能已损坏".to_string(),
            AppError::JsonError(_) => "数据格式错误，无法解析JSON数据".to_string(),
            AppError::ZipError(_) => "压缩文件处理失败，文件可能已损坏".to_string(),
            AppError::ValidationError(msg) => format!("验证失败: {}", msg),
            AppError::NotFoundError(msg) => format!("未找到资源: {}", msg),
            AppError::PermissionError(_) => "权限不足，请以管理员身份运行应用程序".to_string(),
            AppError::ExecutionError(_) => "执行操作失败，请确保系统满足运行要求".to_string(),
            AppError::DatabaseError(_) => "数据库操作失败，请重启应用程序".to_string(),
            AppError::UnknownError(_) => "发生未知错误，请尝试重启应用程序".to_string(),
        }
    }

    // 记录错误日志
    pub fn log(&self, source: Option<&str>) {
        let error_code = self.error_code() as i32;
        let source_str = source.unwrap_or("未知来源");
        error!(
            "[错误] 代码: {}, 来源: {}, 消息: {}",
            error_code, source_str, self
        );

        // 对于特定错误类型添加更多信息
        match self {
            AppError::RequestError(msg) => {
                if msg.contains("timeout") {
                    warn!("[网络超时] 请求超时: {}", msg);
                }
                if msg.contains("connect") {
                    warn!("[网络连接] 连接失败: {}", msg);
                }
            }
            AppError::IoError(msg) => {
                warn!("[IO错误] 消息: {}", msg);
            }
            _ => {}
        }
    }
}

// 在错误转换为结果之前记录日志的辅助函数
#[allow(dead_code)]
pub fn log_error<T>(result: AppResult<T>, source: &str) -> AppResult<T> {
    if let Err(ref e) = result {
        e.log(Some(source));
    }
    result
}

// 用于序列化的错误响应结构
#[derive(Serialize)]
struct ErrorResponse {
    code: ErrorCode,
    message: String,
    details: String,
    timestamp: String,
}

impl Serialize for AppError {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
        // 记录错误
        self.log(None);

        let response = ErrorResponse {
            code: self.error_code(),
            message: self.user_message(),
            details: self.to_string(),
            timestamp: Local::now().format("%Y-%m-%d %H:%M:%S").to_string(),
        };
        response.serialize(serializer)
    }
}

pub type AppResult<T> = Result<T, AppError>;
