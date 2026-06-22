import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import * as AppService from '../../bindings/GameModMaster/appservice'

// Matches backend buildGameEntry response
export interface LatestTrainer {
  id: number
  version: string
  game_version: string
  download_count: number
  file_size: number
}

export interface GameEntry {
  id: number
  source_id: string
  name_en: string
  name_local: string
  display_name: string
  cover_url: string
  source_url: string
  options_num: number
  updated_at: number
  trainer_count: number
  latest_trainer?: LatestTrainer
  status: number // 0=可用 1=已下载 2=已安装
  local_path: string
}

// Matches backend buildTrainerWithGameEntry response
export interface DownloadedTrainer {
  id: number
  game_id: number
  game_name: string
  game_name_en: string
  cover_url: string
  version: string
  game_version: string
  download_url: string
  file_size: number
  file_name: string
  download_count: number
  source_hash: string
  updated_at: number
  status: number
  local_path: string
  installed_at: number
  launched_at: number
}

// Matches backend GetTrainerDetail response
export interface TrainerDetail {
  id: number
  game_id: number
  version: string
  game_version: string
  download_url: string
  file_size: number
  file_name: number
  download_count: number
  source_hash: string
  updated_at: number
  status: number
  local_path: string
  installed_at: number
  launched_at: number
}

export interface GameDetail {
  id: number
  source_id: string
  name_en: string
  name_local: string
  display_name: string
  cover_url: string
  source_url: string
  options_num: number
  updated_at: number
}

export interface TrainerDetailResponse {
  game: GameDetail
  trainers: TrainerDetail[]
}

export interface RefreshProgress {
  page?: number
  total?: number
  current?: number
  games?: number
  trainers?: number
  done?: boolean
  cancelled?: boolean
  summary?: string
  detail_errors?: number
}

export interface DownloadProgress {
  trainer_id: number
  downloaded?: number
  total?: number
  speed?: number
  done?: boolean
}

// Normalize whatever the binding returns into a plain array.
// Wails bindings may wrap results; this guards against shape mismatches.
function toArray(result: unknown): GameEntry[] {
  if (!result) return []
  if (Array.isArray(result)) return result as GameEntry[]
  // Some wrappers expose the array under .data
  const r = result as Record<string, unknown>
  if (r && Array.isArray(r.data)) return r.data as GameEntry[]
  return []
}

export const useTrainerStore = defineStore('trainer', () => {
  const trainers = ref<GameEntry[]>([])
  const loading = ref(false)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const pageSize = ref(60)
  const totalCount = ref(0)
  const refreshing = ref(false)

  // Last error surfaced to the UI (no longer swallowed in console only).
  const lastError = ref('')

  const refreshProgress = ref<RefreshProgress>({})
  const refreshSummary = ref('')

  const downloadProgress = ref<Record<number, DownloadProgress>>({})

  let listenersBound = false
  // Reload the home grid every N pages during a crawl so games appear
  // incrementally instead of the list staying empty for minutes.
  let lastReloadedAt = 0
  function bindEvents() {
    if (listenersBound) return
    listenersBound = true
    Events.On('refresh:progress', (ev: any) => {
      // ev.data may be the payload object directly, or an array of args.
      const raw = ev?.data
      const data: RefreshProgress = Array.isArray(raw) ? raw[0] : raw
      if (!data) return
      refreshProgress.value = data
      if (data.done) {
        refreshing.value = false
        if (data.summary) refreshSummary.value = data.summary
        // Auto-reload the visible list with freshly fetched data.
        loadTrainers(currentPage.value)
        lastReloadedAt = data.current || 0
      } else if (data.current && data.current - lastReloadedAt >= 5) {
        // Incremental reload while crawling (the backend rebuilds the index
        // every 5 pages, so this picks up the newly-stored games).
        loadTrainers(currentPage.value)
        lastReloadedAt = data.current
      }
    })
    Events.On('download:progress', (ev: any) => {
      const raw = ev?.data
      const data: DownloadProgress = Array.isArray(raw) ? raw[0] : raw
      if (!data || data.trainer_id == null) return
      downloadProgress.value = { ...downloadProgress.value, [data.trainer_id]: data }
      if (data.done) {
        const id = data.trainer_id
        setTimeout(() => {
          const next = { ...downloadProgress.value }
          delete next[id]
          downloadProgress.value = next
        }, 1500)
      }
    })
  }

  async function loadTrainers(page: number = 1) {
    loading.value = true
    lastError.value = ''
    try {
      const result = await AppService.GetTrainers(page, pageSize.value)
      trainers.value = toArray(result)
      currentPage.value = page
      totalCount.value = await AppService.GetTotalGames()
    } catch (e) {
      lastError.value = `加载列表失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[loadTrainers]', e)
    } finally {
      loading.value = false
    }
  }

  async function searchTrainers(query: string) {
    if (!query.trim()) {
      await loadTrainers(1)
      return
    }
    loading.value = true
    lastError.value = ''
    searchQuery.value = query
    try {
      const result = await AppService.SearchTrainers(query)
      trainers.value = toArray(result)
      currentPage.value = 1
      totalCount.value = trainers.value.length
    } catch (e) {
      lastError.value = `搜索失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[searchTrainers]', e)
    } finally {
      loading.value = false
    }
  }

  async function refreshData() {
    bindEvents()
    if (refreshing.value) return
    refreshing.value = true
    refreshProgress.value = {}
    refreshSummary.value = ''
    lastError.value = ''
    try {
      await AppService.RefreshData()
    } catch (e) {
      lastError.value = `刷新失败: ${String(e)}`
      refreshing.value = false
      // eslint-disable-next-line no-console
      console.error('[refreshData]', e)
    }
  }

  async function refreshDataSync() {
    bindEvents()
    refreshing.value = true
    lastError.value = ''
    try {
      const summary = await AppService.RefreshDataSync()
      refreshSummary.value = (summary as unknown as string) || ''
      await loadTrainers(currentPage.value)
    } catch (e) {
      lastError.value = `刷新失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[refreshDataSync]', e)
    } finally {
      refreshing.value = false
    }
  }

  async function onRefreshComplete() {
    await loadTrainers(currentPage.value)
  }

  async function cancelRefresh() {
    try {
      await AppService.CancelRefresh()
    } catch (e) {
      // eslint-disable-next-line no-console
      console.error('[cancelRefresh]', e)
      throw e
    }
  }

  async function downloadTrainer(trainerId: number) {
    bindEvents()
    lastError.value = ''
    try {
      await AppService.DownloadTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      lastError.value = `下载失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[downloadTrainer]', e)
    }
  }

  async function installTrainer(trainerId: number) {
    lastError.value = ''
    try {
      await AppService.InstallTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      lastError.value = `安装失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[installTrainer]', e)
    }
  }

  async function launchTrainer(trainerId: number) {
    lastError.value = ''
    try {
      await AppService.LaunchTrainer(trainerId)
    } catch (e) {
      lastError.value = `启动失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[launchTrainer]', e)
    }
  }

  async function deleteTrainer(trainerId: number) {
    lastError.value = ''
    try {
      await AppService.DeleteTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      lastError.value = `删除失败: ${String(e)}`
      // eslint-disable-next-line no-console
      console.error('[deleteTrainer]', e)
    }
  }

  return {
    trainers,
    loading,
    searchQuery,
    currentPage,
    pageSize,
    totalCount,
    refreshing,
    refreshProgress,
    refreshSummary,
    downloadProgress,
    lastError,
    loadTrainers,
    searchTrainers,
    refreshData,
    refreshDataSync,
    onRefreshComplete,
    cancelRefresh,
    downloadTrainer,
    installTrainer,
    launchTrainer,
    deleteTrainer,
    bindEvents,
  }
})
