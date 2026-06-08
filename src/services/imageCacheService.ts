import { invoke } from '@tauri-apps/api/core'
import { convertFileSrc } from '@tauri-apps/api/core'

/**
 * Image Cache API
 * Provides frontend methods to interact with the Rust image caching backend
 */

export interface CacheStats {
  file_count: number
  total_size_bytes: number
  max_size_bytes: number
  cache_dir: string
}

export interface CleanupResult {
  deleted_count: number
  deleted_size_bytes: number
  errors: string[]
}

/**
 * Get a cached image URL
 * Downloads and caches the image if not already cached
 * Returns a Tauri asset URL that can be used in img src
 */
export async function getCachedImageUrl(remoteUrl: string): Promise<string> {
  if (!remoteUrl) {
    return '/placeholder.png'
  }

  try {
    // Get the local path from Rust backend
    const localPath = await invoke<string>('get_cached_image', { url: remoteUrl })

    // Convert local file path to Tauri asset URL
    // This uses Tauri's convertFileSrc to create a proper asset URL
    const assetUrl = convertFileSrc(localPath)

    return assetUrl
  } catch (error) {
    console.error('Failed to cache image:', error)
    // Return original URL as fallback
    return remoteUrl
  }
}

/**
 * Get cache statistics
 */
export async function getCacheStats(): Promise<CacheStats> {
  return await invoke<CacheStats>('get_cache_stats')
}

/**
 * Clean up old cached images (older than 30 days)
 */
export async function cleanupImageCache(): Promise<CleanupResult> {
  return await invoke<CleanupResult>('cleanup_image_cache')
}

/**
 * Clear all cached images
 */
export async function clearImageCache(): Promise<number> {
  return await invoke<number>('clear_image_cache')
}

/**
 * Batch cache multiple images
 * Returns a map of original URLs to cached URLs
 */
export async function batchCacheImages(urls: string[]): Promise<Map<string, string>> {
  const cachedUrls = new Map<string, string>()

  // Cache images in parallel for performance
  const cachePromises = urls.map(async (url) => {
    const cachedUrl = await getCachedImageUrl(url)
    cachedUrls.set(url, cachedUrl)
  })

  await Promise.all(cachePromises)

  return cachedUrls
}

/**
 * React/Vue composable for automatic image caching
 * Can be used as a utility function in components
 */
export function useCachedImage() {
  const cacheImage = async (url: string): Promise<string> => {
    return await getCachedImageUrl(url)
  }

  const cacheImages = async (urls: string[]): Promise<Map<string, string>> => {
    return await batchCacheImages(urls)
  }

  return {
    cacheImage,
    cacheImages,
    getStats: getCacheStats,
    cleanup: cleanupImageCache,
    clearAll: clearImageCache,
  }
}
