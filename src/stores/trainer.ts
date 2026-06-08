import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { invoke } from '@tauri-apps/api/core'
import type { MessageApi } from 'naive-ui'
import type { Trainer, InstalledTrainer } from '../types'
import { handleError } from '../utils/errorHandler'
import { StorageService, withRetry } from '../services/storageService'

// 缓存配置
const CACHE_CONFIG = {
  // 缓存过期时间（毫秒）
  expirationTime: 1000 * 60 * 15, // 15分钟
  // 最大重试次数
  maxRetries: 3,
  // 重试延迟（毫秒）
  retryDelay: 1000,
}

// 应用版本（用于数据兼容性检测）
const APP_VERSION = '2.0.0'

/**
 * 检查并清理旧版本数据
 */
async function checkAndCleanOldData() {
  try {
    const storedVersion = localStorage.getItem('app_version')
    if (storedVersion && storedVersion !== APP_VERSION) {
      console.log('Store: 检测到版本不匹配，清理旧数据')
      localStorage.clear()
      localStorage.setItem('app_version', APP_VERSION)
      console.log('Store: 旧数据清理完成')
    } else if (!storedVersion) {
      localStorage.setItem('app_version', APP_VERSION)
    }
  } catch (err) {
    console.warn('Store: 版本检查失败:', err)
  }
}

declare global {
  interface Window {
    $message: MessageApi
  }
}

export const useTrainerStore = defineStore('trainer', () => {
  // 状态
  const trainers = ref<Trainer[]>([]) // 所有修改器列表
  const installedTrainers = ref<InstalledTrainer[]>([]) // 已安装的修改器
  const downloadedTrainers = ref<Trainer[]>([]) // 已下载的修改器
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const totalPages = ref(1)
  const lastUpdated = ref(Date.now()) // 上次更新时间
  const isStorageMigrated = ref(StorageService.isMigrated()) // 存储迁移状态

  // 优化的计算属性 - 使用 Set 提升查找性能
  const downloadedIds = computed(() => new Set(downloadedTrainers.value.map(t => t.id)))
  const installedIds = computed(() => new Set(installedTrainers.value.map(t => t.id)))

  // 计算属性
  const recentlyInstalledTrainers = computed(() => {
    return [...installedTrainers.value]
      .sort((a, b) => new Date(b.install_time).getTime() - new Date(a.install_time).getTime())
      .slice(0, 5)
  })

  const recentlyLaunchedTrainers = computed(() => {
    return [...installedTrainers.value]
      .filter((t) => t.last_launch_time)
      .sort((a, b) => {
        const timeA = a.last_launch_time ? new Date(a.last_launch_time).getTime() : 0
        const timeB = b.last_launch_time ? new Date(b.last_launch_time).getTime() : 0
        return timeB - timeA
      })
      .slice(0, 5)
  })

  // 初始化函数
  async function initialize() {
    console.log('Store: 开始初始化')
    try {
      isLoading.value = true
      error.value = null

      // 检查并清理旧版本数据
      await checkAndCleanOldData()

      // 首先检查并执行存储迁移
      if (!isStorageMigrated.value) {
        console.log('Store: 开始存储迁移')
        await StorageService.migrateFromLocalStorage()
        isStorageMigrated.value = true
        console.log('Store: 存储迁移完成')
      }

      // 从新存储加载数据（本地数据，应该很快）
      const [installed, downloaded] = await Promise.all([
        StorageService.getInstalledTrainers(),
        StorageService.getDownloadedTrainers()
      ])

      installedTrainers.value = installed
      downloadedTrainers.value = downloaded

      console.log('Store: 已加载本地数据:', {
        installed: installedTrainers.value.length,
        downloaded: downloadedTrainers.value.length,
        migrated: isStorageMigrated.value,
      })

      // 清理过期缓存（异步执行，不阻塞）
      StorageService.cleanExpiredCache().catch(err => {
        console.warn('Store: 清理缓存失败:', err)
      })

      // 标记初始化完成（不等待网络请求）
      isLoading.value = false
      console.log('Store: 初始化完成（本地数据已加载）')

      // 更新最后刷新时间
      lastUpdated.value = Date.now()

      // 在后台异步加载远程修改器列表（不阻塞初始化）
      fetchTrainers(1).then(() => {
        console.log('Store: 后台加载远程数据完成')
      }).catch(err => {
        console.warn('Store: 后台加载远程数据失败:', err)
      })
    } catch (err) {
      console.error('Store: 初始化失败:', err)
      error.value = err instanceof Error ? err.message : '加载数据失败'
      handleError(err, window.$message)
      isLoading.value = false
    }
  }

  // 添加修改器
  async function addTrainer(trainer: Trainer) {
    try {
      // 检查是否已经存在
      const exists = installedTrainers.value.some((t) => t.id === trainer.id)
      if (exists) {
        throw new Error('修改器已经安装')
      }

      // 添加安装时间
      const now = new Date().toISOString()
      const trainerWithMeta: InstalledTrainer = {
        ...trainer,
        installed_path: '', // 后端存储，不需要实际路径
        install_time: now,
        last_launch_time: now,
      }

      // 添加到列表
      installedTrainers.value.push(trainerWithMeta)
      // 保存到新存储
      await StorageService.saveInstalledTrainers(installedTrainers.value)

      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : '添加修改器失败'
      console.error('Failed to add trainer:', err)
      return false
    }
  }

  // 移除修改器
  async function removeTrainer(trainerId: string) {
    try {
      // 从列表中移除
      installedTrainers.value = installedTrainers.value.filter((t) => t.id !== trainerId)
      // 更新新存储
      await StorageService.saveInstalledTrainers(installedTrainers.value)
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : '移除修改器失败'
      console.error('Failed to remove trainer:', err)
      return false
    }
  }

  // 更新修改器信息
  async function updateTrainer(trainer: Trainer) {
    try {
      const index = installedTrainers.value.findIndex((t) => t.id === trainer.id)
      if (index === -1) {
        throw new Error('修改器不存在')
      }

      // 更新信息
      installedTrainers.value[index] = {
        ...installedTrainers.value[index],
        ...trainer,
      }

      // 保存到新存储
      await StorageService.saveInstalledTrainers(installedTrainers.value)
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : '更新修改器失败'
      console.error('Failed to update trainer:', err)
      return false
    }
  }

  // 更新启动时间
  async function updateLaunchTime(trainerId: string) {
    try {
      const index = installedTrainers.value.findIndex((t) => t.id === trainerId)
      if (index === -1) {
        throw new Error('修改器不存在')
      }

      // 更新启动时间
      installedTrainers.value[index] = {
        ...installedTrainers.value[index],
        last_launch_time: new Date().toISOString(),
      }

      // 保存到新存储
      await StorageService.saveInstalledTrainers(installedTrainers.value)
      return true
    } catch (err) {
      error.value = err instanceof Error ? err.message : '更新启动时间失败'
      console.error('Failed to update launch time:', err)
      return false
    }
  }

  // 获取修改器信息
  function getTrainer(trainerId: string): Trainer | undefined {
    return installedTrainers.value.find((t: Trainer) => t.id === trainerId)
  }

  // 清除错误
  function clearError() {
    error.value = null
  }

  // 所有已缓存的修改器（用于本地搜索和分页）
  const allCachedTrainers = ref<Trainer[]>([])
  const pageSize = ref(12) // 每页显示条数
  const isCacheLoaded = ref(false) // 缓存是否已加载

  // 获取修改器列表（优化版本，支持本地分页）
  async function fetchTrainers(page: number, newPageSize?: number) {
    try {
      isLoading.value = true
      error.value = null
      currentPage.value = page

      // 更新每页条数
      if (newPageSize) {
        pageSize.value = newPageSize
      }

      // 如果缓存中有数据，使用本地分页
      if (allCachedTrainers.value.length > 0 && !searchQuery.value.trim()) {
        console.log('Store: 使用本地缓存数据进行分页，页码:', page, '每页:', pageSize.value, '总数:', allCachedTrainers.value.length)
        const start = (page - 1) * pageSize.value
        const end = start + pageSize.value
        trainers.value = allCachedTrainers.value.slice(start, end)
        totalPages.value = Math.ceil(allCachedTrainers.value.length / pageSize.value)
        isLoading.value = false
        return
      }

      // 如果缓存未加载，先加载所有缓存数据（获取前 6 页，约 120 条记录）
      if (!isCacheLoaded.value && !searchQuery.value.trim()) {
        console.log('Store: 开始加载所有数据用于本地分页...')
        const allTrainers: Trainer[] = []

        // 获取前 6 页数据
        for (let p = 1; p <= 6; p++) {
          try {
            const response = await withRetry(() =>
              invoke<{
                trainers: Trainer[]
                total: number
              }>('fetch_trainers', { page: p }),
            )
            allTrainers.push(...response.trainers)

            // 如果返回的数据少于预期，说明已经到了最后一页
            if (response.trainers.length === 0) {
              break
            }
          } catch (err) {
            console.warn(`Store: 获取第 ${p} 页数据失败:`, err)
            break
          }
        }

        // 缓存所有数据
        allCachedTrainers.value = allTrainers
        isCacheLoaded.value = true
        console.log('Store: 已缓存所有数据，总数:', allTrainers.length)

        // 使用本地分页返回第一页
        const start = (page - 1) * pageSize.value
        const end = start + pageSize.value
        trainers.value = allCachedTrainers.value.slice(start, end)
        totalPages.value = Math.ceil(allCachedTrainers.value.length / pageSize.value)
        isLoading.value = false
        return
      }

      // 发起API请求（用于搜索等场景）
      const response = await withRetry(() =>
        invoke<{
          trainers: Trainer[]
          total: number
        }>('fetch_trainers', { page }),
      )

      trainers.value = response.trainers
      // 估算总页数（假设每页20条记录）
      totalPages.value = Math.ceil(response.total / 20)

      console.log('Store: 已获取修改器列表，页码:', page, '总数:', response.trainers.length)
    } catch (err) {
      console.error('Store: 获取修改器列表失败:', err)
      error.value = err instanceof Error ? err.message : '获取修改器列表失败'
      handleError(err, window.$message)
    } finally {
      isLoading.value = false
    }
  }

  // 搜索修改器（优化版本，支持中文名和别名）
  async function searchTrainers(query: string, page = 1) {
    try {
      if (!query.trim()) {
        return await fetchTrainers(page)
      }

      isLoading.value = true
      error.value = null
      searchQuery.value = query
      currentPage.value = page

      // 检查缓存是否有效
      const cachedData = await StorageService.getCachedSearchResults(query, page)
      if (cachedData) {
        console.log('Store: 使用缓存的搜索结果，查询:', query, '页码:', page)
        trainers.value = cachedData
        totalPages.value = Math.ceil(cachedData.length / 20)
        return
      }

      // 发起API请求
      const response = await withRetry(() =>
        invoke<{
          trainers: Trainer[]
          total: number
        }>('search_trainers', { query, page }),
      )

      trainers.value = response.trainers
      totalPages.value = Math.ceil(response.total / 20)

      // 缓存结果
      await StorageService.cacheSearchResults(query, page, trainers.value)

      console.log(
        'Store: 搜索完成，查询:',
        query,
        '页码:',
        page,
        '结果数:',
        trainers.value.length,
      )
    } catch (err) {
      console.error('Store: 搜索失败:', err)
      error.value = err instanceof Error ? err.message : '搜索失败'
      handleError(err, window.$message)
    } finally {
      isLoading.value = false
    }
  }

  // 获取修改器详情
  async function getTrainerDetail(id: string) {
    try {
      const result = await invoke<Trainer>('get_trainer_detail', { id })
      return result
    } catch (err) {
      handleError(err, window.$message)
      throw err // 允许调用者处理错误
    }
  }

  // 下载修改器
  async function downloadTrainer(trainer: Trainer) {
    try {
      const result = await invoke<Trainer>('download_trainer', { trainer })

      // 添加到下载记录（使用后端返回的翻译后的trainer对象）
      const exists = downloadedTrainers.value.some((t) => t.id === result.id)
      if (!exists) {
        downloadedTrainers.value.push(result) // 使用翻译后的trainer
        await StorageService.saveDownloadedTrainers(downloadedTrainers.value)
      } else {
        // 如果已存在，更新为中文名
        const index = downloadedTrainers.value.findIndex((t) => t.id === result.id)
        if (index !== -1) {
          downloadedTrainers.value[index] = result
          await StorageService.saveDownloadedTrainers(downloadedTrainers.value)
        }
      }

      // 检查是否需要自动打开文件夹
      try {
        const settings = await invoke<{ auto_open_folder: boolean }>('get_settings')
        if (settings.auto_open_folder) {
          await invoke('open_download_folder')
        }
      } catch (err) {
        console.warn('无法获取设置或打开文件夹:', err)
      }

      return result
    } catch (err) {
      handleError(err, window.$message)
      throw err
    }
  }

  // 删除修改器
  async function deleteTrainer(trainerId: string) {
    try {
      await invoke('delete_trainer', { trainerId })

      // 从下载记录中删除
      downloadedTrainers.value = downloadedTrainers.value.filter((t) => t.id !== trainerId)
      await StorageService.saveDownloadedTrainers(downloadedTrainers.value)

      return true
    } catch (err) {
      handleError(err, window.$message)
      throw err
    }
  }

  // 启动修改器
  async function launchTrainer(trainerId: string) {
    try {
      await invoke('launch_trainer', { trainerId })
      return true
    } catch (err) {
      handleError(err, window.$message)
      throw err
    }
  }

  return {
    // 状态
    trainers,
    installedTrainers,
    downloadedTrainers,
    isLoading,
    error,
    searchQuery,
    currentPage,
    totalPages,
    lastUpdated,
    isStorageMigrated,

    // 计算属性
    recentlyInstalledTrainers,
    recentlyLaunchedTrainers,
    downloadedIds,
    installedIds,

    // 方法
    initialize,
    addTrainer,
    removeTrainer,
    updateTrainer,
    updateLaunchTime,
    getTrainer,
    clearError,
    fetchTrainers,
    searchTrainers,
    getTrainerDetail,
    downloadTrainer,
    deleteTrainer,
    launchTrainer,

    // 缓存管理
    cleanCache: StorageService.cleanExpiredCache,
    refreshData: () => {
      // 强制刷新数据（清除缓存并重新加载）
      allCachedTrainers.value = []
      isCacheLoaded.value = false
      lastUpdated.value = Date.now()
      return fetchTrainers(currentPage.value)
    },

    // 存储管理
    resetMigration: StorageService.resetMigration,
    clearAllStorage: StorageService.clearAllStorage,
  }
})