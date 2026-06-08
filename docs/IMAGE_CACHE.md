# Image Caching System

This document explains the image caching system implemented for the GameModMaster project.

## Overview

The image caching system provides:
- **Automatic image downloading and caching** from remote URLs
- **Persistent local storage** in the system cache directory
- **SQLite database tracking** for cache metadata
- **Automatic cache cleanup** based on age and size limits
- **Frontend integration** via Tauri commands

## Architecture

### Backend (Rust)

Location: `src-tauri/src/services/image_cache.rs`

Key components:
- **Cache Storage**: Images stored in `{CacheDir}/image_cache/`
- **Database**: Metadata tracked in SQLite `image_cache` table
- **SHA256 Hashing**: Unique filenames generated from URLs
- **Size Management**: 500MB max cache size, 30-day expiration

### Frontend (TypeScript)

Location: `src/services/imageCacheService.ts`

Key functions:
- `getCachedImageUrl()`: Get cached image URL (downloads if not cached)
- `getCacheStats()`: Get cache statistics
- `cleanupImageCache()`: Clean up old cached images
- `clearImageCache()`: Clear all cached images

## Usage

### In Vue Components

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getCachedImageUrl } from '@/services/imageCacheService'

const imageUrl = ref('')

onMounted(async () => {
  // Get cached image URL (will download and cache if not already cached)
  imageUrl.value = await getCachedImageUrl('https://example.com/image.jpg')
})
</script>

<template>
  <img :src="imageUrl" alt="Cached image" />
</template>
```

### In React Components

```tsx
import { useState, useEffect } from 'react'
import { getCachedImageUrl } from '@/services/imageCacheService'

function CachedImage({ url }: { url: string }) {
  const [cachedUrl, setCachedUrl] = useState(url)

  useEffect(() => {
    async function loadImage() {
      const cached = await getCachedImageUrl(url)
      setCachedUrl(cached)
    }
    loadImage()
  }, [url])

  return <img src={cachedUrl} alt="Cached image" />
}
```

### Direct API Calls

```typescript
import {
  getCachedImageUrl,
  getCacheStats,
  cleanupImageCache,
  clearImageCache
} from '@/services/imageCacheService'

// Cache a single image
const cachedUrl = await getCachedImageUrl('https://example.com/image.jpg')

// Get cache statistics
const stats = await getCacheStats()
console.log(`Cache size: ${stats.total_size_bytes} bytes`)
console.log(`File count: ${stats.file_count}`)

// Clean up old images (older than 30 days)
const cleanupResult = await cleanupImageCache()
console.log(`Deleted ${cleanupResult.deleted_count} files`)

// Clear all cached images
const deletedCount = await clearImageCache()
console.log(`Cleared ${deletedCount} cached images`)
```

## Configuration

### Cache Settings

Edit `src-tauri/src/services/image_cache.rs`:

```rust
const MAX_CACHE_SIZE_MB: u64 = 500; // Maximum cache size in MB
const CACHE_EXPIRATION_DAYS: i64 = 30; // Days before cache expires
```

### Asset Protocol

The asset protocol is configured in `tauri.conf.json`:

```json
{
  "app": {
    "security": {
      "assetProtocol": {
        "enable": true,
        "scope": ["$CACHE/**", "$RESOURCE/**", "**"]
      }
    }
  }
}
```

## Cache Management

### Automatic Cleanup

The system automatically manages cache:
1. **Age-based expiration**: Images older than 30 days are removed
2. **Size-based cleanup**: When cache exceeds 500MB, oldest files are deleted
3. **Last access tracking**: Frequently accessed images are kept longer

### Manual Cleanup

Users can manually clear cache through the UI:

```typescript
// Add a "Clear Cache" button
async function handleClearCache() {
  const deletedCount = await clearImageCache()
  message.success(`Cleared ${deletedCount} cached images`)
}
```

## Database Schema

The `image_cache` table stores metadata:

```sql
CREATE TABLE image_cache (
    url TEXT PRIMARY KEY,
    local_path TEXT NOT NULL,
    cache_time TEXT NOT NULL,
    last_access_time TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    content_type TEXT
);
```

## Error Handling

The system handles errors gracefully:

```typescript
try {
  const cachedUrl = await getCachedImageUrl(remoteUrl)
  // Use cached URL
} catch (error) {
  console.error('Failed to cache image:', error)
  // Fall back to original URL
  return remoteUrl
}
```

## Performance Benefits

1. **Reduced bandwidth**: Images downloaded once and reused
2. **Faster loading**: Local file access instead of network requests
3. **Offline support**: Images available without internet connection
4. **Reduced server load**: Fewer requests to remote servers

## Best Practices

1. **Always handle errors**: Fall back to original URL if caching fails
2. **Use lazy loading**: Let images load as needed (already in GameCard.vue)
3. **Monitor cache size**: Periodically check and clean cache
4. **Test offline**: Verify images load without network access

## Troubleshooting

### Images not loading

1. Check asset protocol configuration in tauri.conf.json
2. Verify cache directory exists and is writable
3. Check database for cached image entries
4. Test with `getCacheStats()` to verify cache status

### Cache taking too much space

1. Reduce `MAX_CACHE_SIZE_MB` in image_cache.rs
2. Call `cleanupImageCache()` more frequently
3. Reduce `CACHE_EXPIRATION_DAYS` to expire images sooner

### Database errors

1. Verify SQLite database is accessible
2. Check `image_cache` table exists
3. Review error logs for specific error messages

## Future Improvements

Potential enhancements:
- [ ] Image compression before caching
- [ ] Support for WebP conversion
- [ ] Progressive image loading
- [ ] Prefetching for better UX
- [ ] Cache statistics in settings UI
- [ ] Automatic background cleanup on app startup
