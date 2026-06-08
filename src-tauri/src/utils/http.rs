use once_cell::sync::Lazy;
use reqwest::Client;
use std::time::Duration;

/// 全局 HTTP 客户端（带连接池和超时配置）
pub static HTTP_CLIENT: Lazy<Client> = Lazy::new(|| {
    Client::builder()
        .timeout(Duration::from_secs(30)) // 总超时 30 秒
        .connect_timeout(Duration::from_secs(10)) // 连接超时 10 秒
        .pool_max_idle_per_host(10) // 连接池大小
        .pool_idle_timeout(Duration::from_secs(60)) // 空闲连接超时
        .user_agent("GameModMaster/2.0")
        .build()
        .expect("Failed to create HTTP client")
});

/// 快速客户端（短超时，用于快速检查）
pub static FAST_CLIENT: Lazy<Client> = Lazy::new(|| {
    Client::builder()
        .timeout(Duration::from_secs(5))
        .connect_timeout(Duration::from_secs(3))
        .user_agent("GameModMaster/2.0")
        .build()
        .expect("Failed to create fast HTTP client")
});
