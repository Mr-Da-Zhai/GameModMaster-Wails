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
  status: number // 1=已下载 2=已安装
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
  file_name: string
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
  summary?: string
}

export interface DownloadProgress {
  trainer_id: number
  downloaded?: number
  total?: number
  speed?: number
  done?: boolean
}

export const useTrainerStore = defineStore('trainer', () => {
  const trainers = ref<GameEntry[]>([])
  const loading = ref(false)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const pageSize = ref(50)
  const totalCount = ref(0)
  const refreshing = ref(false)

  // Refresh progress tracked via the "refresh:progress" event.
  const refreshProgress = ref<RefreshProgress>({})
  const refreshSummary = ref('')

  // Per-trainer download progress tracked via the "download:progress" event.
  const downloadProgress = ref<Record<number, DownloadProgress>>({})

  // Listen to progress events from the backend. Registered once.
  let listenersBound = false
  function bindEvents() {
    if (listenersBound) return
    listenersBound = true
    Events.On('refresh:progress', (ev: any) => {
      const data = ev?.data as RefreshProgress | undefined
      if (!data) return
      refreshProgress.value = data
      if (data.done) {
        refreshing.value = false
        if (data.summary) refreshSummary.value = data.summary
      }
    })
    Events.On('download:progress', (ev: any) => {
      const data = ev?.data as DownloadProgress | undefined
      if (!data || data.trainer_id == null) return
      downloadProgress.value = { ...downloadProgress.value, [data.trainer_id]: data }
      if (data.done) {
        // Clear progress shortly after completion
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
    try {
      const result = await AppService.GetTrainers(page, pageSize.value)
      trainers.value = (result || []) as unknown as GameEntry[]
      currentPage.value = page
      totalCount.value = trainers.value.length
    } catch (e) {
      console.error('Failed to load trainers:', e)
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
    searchQuery.value = query
    try {
      const result = await AppService.SearchTrainers(query)
      trainers.value = (result || []) as unknown as GameEntry[]
      currentPage.value = 1
      totalCount.value = trainers.value.length
    } catch (e) {
      console.error('Failed to search trainers:', e)
    } finally {
      loading.value = false
    }
  }

  // RefreshData is now async on the backend: it returns immediately and
  // reports progress via the "refresh:progress" event. We auto-reload the
  // list when the event signals completion.
  async function refreshData() {
    bindEvents()
    if (refreshing.value) return
    refreshing.value = true
    refreshProgress.value = {}
    refreshSummary.value = ''
    try {
      await AppService.RefreshData()
      // Completion is handled by the event listener above; this reloads data.
    } catch (e) {
      console.error('Failed to refresh data:', e)
      refreshing.value = false
    }
  }

  // Synchronous refresh (blocks until done). Used for first-run seeding.
  async function refreshDataSync() {
    bindEvents()
    refreshing.value = true
    try {
      const summary = await AppService.RefreshDataSync() as unknown as string
      refreshSummary.value = summary || ''
      await loadTrainers(currentPage.value)
    } catch (e) {
      console.error('Failed to refresh data (sync):', e)
    } finally {
      refreshing.value = false
    }
  }

  // Called by the UI when the refresh:progress event reports done, to refresh
  // the visible list with the newly-fetched data.
  async function onRefreshComplete() {
    await loadTrainers(currentPage.value)
  }

  async function downloadTrainer(trainerId: number) {
    bindEvents()
    try {
      await AppService.DownloadTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      console.error('Failed to download trainer:', e)
    }
  }

  async function installTrainer(trainerId: number) {
    try {
      await AppService.InstallTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      console.error('Failed to install trainer:', e)
    }
  }

  async function launchTrainer(trainerId: number) {
    try {
      await AppService.LaunchTrainer(trainerId)
    } catch (e) {
      console.error('Failed to launch trainer:', e)
    }
  }

  async function deleteTrainer(trainerId: number) {
    try {
      await AppService.DeleteTrainer(trainerId)
      await loadTrainers(currentPage.value)
    } catch (e) {
      console.error('Failed to delete trainer:', e)
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
    loadTrainers,
    searchTrainers,
    refreshData,
    refreshDataSync,
    onRefreshComplete,
    downloadTrainer,
    installTrainer,
    launchTrainer,
    deleteTrainer,
    bindEvents,
  }
})
