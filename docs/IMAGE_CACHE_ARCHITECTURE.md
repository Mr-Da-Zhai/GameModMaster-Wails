# Image Cache Architecture Diagram

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Frontend (Vue)                          │
│                                                                  │
│  ┌──────────────┐         ┌──────────────────────┐             │
│  │  GameCard    │────────>│ imageCacheService.ts │             │
│  │  Component   │         │  - getCachedImageUrl │             │
│  │              │         │  - getCacheStats     │             │
│  └──────────────┘         │  - cleanupCache      │             │
│                           └──────────┬───────────┘             │
└──────────────────────────────────────┼──────────────────────────┘
                                       │
                                       │ invoke()
                                       ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Tauri IPC Layer                               │
│                                                                   │
│  Commands:                                                        │
│  - get_cached_image(url) -> String                               │
│  - cleanup_image_cache() -> CleanupResult                        │
│  - get_cache_stats() -> CacheStats                               │
│  - clear_image_cache() -> u32                                    │
└──────────────────────────────────────┬───────────────────────────┘
                                       │
                                       ▼
┌──────────────────────────────────────────────────────────────────┐
│                    Backend (Rust)                                │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │         image_cache.rs Service                            │  │
│  │                                                            │  │
│  │  ┌─────────────┐       ┌──────────────┐                  │  │
│  │  │ get_cached  │──────>│ Check SQLite │                  │  │
│  │  │ _image()    │       │   Database   │                  │  │
│  │  └─────────────┘       └──────┬───────┘                  │  │
│  │                               │                           │  │
│  │                    ┌──────────┴────────┐                  │  │
│  │                    │                   │                  │  │
│  │                    ▼                   ▼                  │  │
│  │            ┌──────────┐        ┌───────────┐             │  │
│  │            │  Cached  │        │  Download │             │  │
│  │            │  Exists  │        │   Image   │             │  │
│  │            └─────┬────┘        └─────┬─────┘             │  │
│  │                  │                   │                    │  │
│  │                  │         ┌─────────┴─────────┐          │  │
│  │                  │         │                   │          │  │
│  │                  │         ▼                   ▼          │  │
│  │                  │  ┌─────────────┐    ┌─────────────┐   │  │
│  │                  │  │   Save to   │    │  Update DB  │   │  │
│  │                  │  │   Disk      │    │  Metadata   │   │  │
│  │                  │  └──────┬──────┘    └──────┬──────┘   │  │
│  │                  │         │                   │          │  │
│  │                  └─────────┴─────────┬─────────┘          │  │
│  │                                        │                   │  │
│  │                                        ▼                   │  │
│  │                              ┌─────────────────┐          │  │
│  │                              │ Return Local   │          │  │
│  │                              │ File Path      │          │  │
│  │                              └─────────────────┘          │  │
│  └───────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
                                       │
                                       ▼
┌──────────────────────────────────────────────────────────────────┐
│                      Storage Layer                               │
│                                                                   │
│  ┌──────────────────┐              ┌─────────────────┐          │
│  │  SQLite Database │              │  File System    │          │
│  │  (app.db)        │              │  Cache Dir      │          │
│  │                  │              │                 │          │
│  │  - url (PK)      │              │  image_cache/   │          │
│  │  - local_path    │◄─────────────┤  ├─ abc123.jpg  │          │
│  │  - cache_time    │              │  ├─ def456.png  │          │
│  │  - file_size     │              │  └─ ghi789.webp │          │
│  │  - content_type  │              │                 │          │
│  └──────────────────┘              └─────────────────┘          │
└──────────────────────────────────────────────────────────────────┘
```

## Image Caching Flow

```
User Opens App
      │
      ▼
┌─────────────────┐
│ Load Trainer    │
│ List            │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ For Each Trainer        │
│ with thumbnail URL      │
└────────┬────────────────┘
         │
         ▼
    ┌────────────┐
    │ Cached?    │
    └─────┬──────┘
          │
     ┌────┴────┐
     │         │
    YES       NO
     │         │
     │         └────────────┐
     │                      │
     ▼                      ▼
┌──────────┐        ┌───────────────┐
│ Return   │        │ Download Image│
│ Cached   │        │ from URL      │
│ Path     │        └───────┬───────┘
└──────────┘                │
                            ▼
                    ┌───────────────┐
                    │ Generate SHA256│
                    │ Filename      │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Save to Cache │
                    │ Directory     │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Save Metadata │
                    │ to SQLite     │
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Return Local  │
                    │ Path          │
                    └───────────────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Convert to    │
                    │ Asset URL     │
                    │ (convertFileSrc)
                    └───────┬───────┘
                            │
                            ▼
                    ┌───────────────┐
                    │ Display in    │
                    │ <img> tag     │
                    └───────────────┘
```

## Cache Cleanup Flow

```
App Startup / Manual Trigger
         │
         ▼
┌────────────────────┐
│ cleanup_image_cache│
└─────────┬──────────┘
          │
          ▼
   ┌──────────────┐
   │ Load All     │
   │ Cached Images│
   │ from DB      │
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐
   │ Check Each   │
   │ Entry        │
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐      YES      ┌─────────────┐
   │ Older than   │──────────────>│ Delete File │
   │ 30 days?     │               │ Delete DB   │
   └──────┬───────┘               │ Entry       │
          │ NO                    └──────┬──────┘
          │                              │
          │<─────────────────────────────┘
          │
          ▼
   ┌──────────────┐
   │ Check Total  │
   │ Cache Size   │
   └──────┬───────┘
          │
          ▼
   ┌──────────────┐      YES      ┌─────────────┐
   │ Size > 500MB?│──────────────>│ Sort by     │
   └──────┬───────┘               │ Last Access │
          │ NO                    │ (Oldest 1st)│
          │                       └──────┬──────┘
          │                              │
          │                              ▼
          │                       ┌─────────────┐
          │                       │ Delete Until│
          │                       │ Size < 500MB│
          │                       └──────┬──────┘
          │                              │
          │<─────────────────────────────┘
          │
          ▼
   ┌──────────────┐
   │ Return Stats │
   └──────────────┘
```

## Data Flow Diagram

```
┌──────────┐     URL      ┌───────────┐    Download    ┌──────────┐
│  Remote  │─────────────>│  Backend  │───────────────>│  Local   │
│  Server  │              │  Service  │                │  Cache   │
└──────────┘              └───────────┘                └──────────┘
                                │                            │
                                │                            │
                                ▼                            ▼
                          ┌───────────┐               ┌──────────┐
                          │  SQLite   │<──────────────│ Metadata │
                          │  Database │    Save       │ (path,   │
                          └───────────┘               │  size,   │
                                │                     │  time)   │
                                │                     └──────────┘
                                ▼
                          ┌───────────┐
                          │  Return   │
                          │  Asset URL│
                          └───────────┘
                                │
                                ▼
                          ┌───────────┐
                          │  Frontend │
                          │  Display  │
                          └───────────┘
```

## Component Interaction

```
GameCard.vue
    │
    ├── onMounted()
    │       │
    │       └──> imageCacheService.getCachedImageUrl(url)
    │                   │
    │                   └──> invoke('get_cached_image', {url})
    │                             │
    │                             └──> Rust: get_cached_image()
    │                                       │
    │                                       ├──> Check DB (image exists?)
    │                                       │       │
    │                                       │       ├──> YES: Return path
    │                                       │       │
    │                                       │       └──> NO: Download
    │                                       │               │
    │                                       │               ├──> HTTP GET
    │                                       │               ├──> Save file
    │                                       │               ├──> Update DB
    │                                       │               └──> Return path
    │                                       │
    │                                       └──> convertFileSrc(path)
    │                                             │
    └─────────────────────────────────────────────┘
    │
    └──> <img :src="cachedImageUrl" />
```

## File Structure

```
GameModMaster/
├── src/
│   ├── components/
│   │   └── common/
│   │       └── GameCard.vue ............... Modified (uses cache)
│   └── services/
│       └── imageCacheService.ts ........... New (frontend API)
│
├── src-tauri/
│   ├── src/
│   │   ├── services/
│   │   │   ├── mod.rs ..................... Modified (added export)
│   │   │   └── image_cache.rs ............. New (cache service)
│   │   └── main.rs ........................ Modified (added commands)
│   ├── Cargo.toml ........................ Modified (added deps)
│   └── tauri.conf.json .................... Modified (asset protocol)
│
└── docs/
    ├── IMAGE_CACHE.md ..................... User guide
    ├── IMAGE_CACHE_IMPLEMENTATION.md ...... Implementation details
    ├── IMAGE_CACHE_QUICKSTART.md .......... Quick start
    └── IMAGE_CACHE_ARCHITECTURE.md ........ This file

System Cache Directory (OS-dependent):
Windows: C:\Users\{User}\AppData\Local\GameModMaster\cache\image_cache\
macOS: ~/Library/Caches/com.gamemodmaster/GameModMaster/image_cache/
Linux: ~/.cache/gamemodmaster/GameModMaster/image_cache/
```

## Performance Characteristics

| Operation | Time | Notes |
|-----------|------|-------|
| First load (download) | ~500-2000ms | Network dependent |
| Cached load (local) | ~5-20ms | Disk read |
| Database query | ~1-5ms | Indexed lookup |
| Cache check | ~1ms | In-memory check |
| Batch cache (10 images) | ~5-10s | Parallel downloads |
| Cache cleanup | ~100-500ms | Depends on file count |

## Memory vs Disk Usage

```
┌──────────────────────┐
│   Memory Usage       │
│                      │
│  - Database conn: ~1MB
│  - Active images: ~10MB (browser cache)
│  - Service state: <1MB
│                      │
│  Total: ~12MB        │
└──────────────────────┘

┌──────────────────────┐
│   Disk Usage         │
│                      │
│  - Cache limit: 500MB
│  - Per image: 50-500KB
│  - Database: ~1MB
│                      │
│  Total: ~500MB max   │
└──────────────────────┘
```
