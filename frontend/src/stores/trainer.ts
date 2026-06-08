import { defineStore } from 'pinia'
import { ref } from 'vue'
import * as AppService from '../../bindings/GameModMaster/appservice'

export interface TrainerItem {
  id: number
  game_id: number
  name_en: string
  name_local: string
  cover_url: string
  source_url: string
  options_num: number
  updated_at: number
  // Trainer fields
  trainer_id: number
  version: string
  game_version: string
  download_url: string
  file_size: number
  file_name: string
  download_count: number
  // State fields
  status: number // 0=可用 1=已下载 2=已安装
  local_path: string
  installed_at: number
}

export const useTrainerStore = defineStore('trainer', () => {
  const trainers = ref<TrainerItem[]>([])
  const loading = ref(false)
  const searchQuery = ref('')
  const currentPage = ref(1)
  const pageSize = ref(50)
  const totalCount = ref(0)
  const refreshing = ref(false)

  async function loadTrainers(page: number = 1) {
    loading.value = true
    try {
      const result = await AppService.GetTrainers(page, pageSize.value)
      trainers.value = (result || []) as TrainerItem[]
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
      trainers.value = (result || []) as TrainerItem[]
      currentPage.value = 1
      totalCount.value = trainers.value.length
    } catch (e) {
      console.error('Failed to search trainers:', e)
    } finally {
      loading.value = false
    }
  }

  async function refreshData() {
    refreshing.value = true
    try {
      await AppService.RefreshData()
      await loadTrainers(currentPage.value)
    } catch (e) {
      console.error('Failed to refresh data:', e)
    } finally {
      refreshing.value = false
    }
  }

  async function downloadTrainer(trainerId: number) {
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
    loadTrainers,
    searchTrainers,
    refreshData,
    downloadTrainer,
    installTrainer,
    launchTrainer,
    deleteTrainer,
  }
})
