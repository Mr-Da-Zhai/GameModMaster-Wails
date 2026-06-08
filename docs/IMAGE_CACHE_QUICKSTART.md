# Image Cache Quick Start Guide

## Quick Implementation (2 minutes)

### Step 1: Import the service
```typescript
import { getCachedImageUrl } from '@/services/imageCacheService'
```

### Step 2: Use in component
```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getCachedImageUrl } from '@/services/imageCacheService'

const props = defineProps<{ thumbnail: string }>()
const cachedUrl = ref(props.thumbnail)

onMounted(async () => {
  cachedUrl.value = await getCachedImageUrl(props.thumbnail)
})
</script>

<template>
  <img :src="cachedUrl" />
</template>
```

## That's it! The image is now cached. 🎉

## Common Patterns

### Pattern 1: Simple Image Caching
```typescript
const cachedUrl = await getCachedImageUrl(remoteUrl)
```

### Pattern 2: Batch Cache Multiple Images
```typescript
import { batchCacheImages } from '@/services/imageCacheService'

const urls = trainers.map(t => t.thumbnail)
const cachedUrls = await batchCacheImages(urls)

// Use cachedUrls.get(originalUrl) to get cached version
```

### Pattern 3: Composable (Recommended for Vue)
```typescript
import { useCachedImage } from '@/services/imageCacheService'

const { cacheImage, getStats, cleanup } = useCachedImage()

// Cache single image
const url = await cacheImage(remoteUrl)

// Get cache stats
const stats = await getStats()

// Clean up old images
await cleanup()
```

## Error Handling

```typescript
try {
  const cachedUrl = await getCachedImageUrl(url)
  return cachedUrl
} catch (error) {
  console.error('Cache failed:', error)
  return url // Fallback to original
}
```

## Cache Management

### Check cache size
```typescript
const stats = await getCacheStats()
console.log(`Cache: ${stats.total_size_bytes / 1024 / 1024} MB`)
```

### Clear old images
```typescript
const result = await cleanupImageCache()
console.log(`Deleted ${result.deleted_count} old images`)
```

### Clear all cache
```typescript
const count = await clearImageCache()
console.log(`Cleared ${count} images`)
```

## Configuration

### Change cache limits
Edit `src-tauri/src/services/image_cache.rs`:
```rust
const MAX_CACHE_SIZE_MB: u64 = 500;  // Change to desired size
const CACHE_EXPIRATION_DAYS: i64 = 30;  // Change expiration days
```

## How It Works

1. **First Load**: Image downloads and saves to cache
2. **Next Load**: Image loads from local cache (instant!)
3. **Cache Full**: Oldest images automatically removed
4. **Offline**: Cached images still work!

## Debug Mode

```typescript
// Check if image is cached
const stats = await getCacheStats()
console.log('Cache stats:', stats)

// Force re-cache
await clearImageCache()
const newUrl = await getCachedImageUrl(url)
```

## Pro Tips

1. **Pre-cache images on app load** for instant display later
2. **Use batch caching** when loading lists of trainers
3. **Monitor cache stats** in settings UI
4. **Add "Clear Cache" button** in settings
5. **Handle errors gracefully** with fallback to original URL

## Troubleshooting

**Q: Images not caching?**
- Check asset protocol in tauri.conf.json
- Verify cache directory permissions
- Check console for errors

**Q: Cache taking too much space?**
- Reduce MAX_CACHE_SIZE_MB
- Run cleanupImageCache()
- Reduce CACHE_EXPIRATION_DAYS

**Q: Images showing placeholder?**
- Check network connectivity
- Verify image URL is valid
- Check error handler is working

## Need Help?

- Full docs: `docs/IMAGE_CACHE.md`
- Implementation details: `docs/IMAGE_CACHE_IMPLEMENTATION.md`
- Source code: `src-tauri/src/services/image_cache.rs`
- Frontend API: `src/services/imageCacheService.ts`
