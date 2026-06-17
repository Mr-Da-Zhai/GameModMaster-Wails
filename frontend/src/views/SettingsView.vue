<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NButton,
  NIcon,
  NSpin,
  NSpace,
  NInput,
  NPopconfirm,
  useMessage,
} from 'naive-ui'
import { FolderOpenOutline, RefreshOutline, TrashOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { Events } from '@wailsio/runtime'
import { useTrainerStore } from '../stores/trainer'

const message = useMessage()
const store = useTrainerStore()

const loading = ref(false)
const refreshing = ref(false)
const dataDir = ref('')
const downloadDir = ref('')
const downloadDirInput = ref('')
const editingDownloadDir = ref(false)
const mappingCount = ref(0)
const totalGames = ref(0)
const settings = ref<Record<string, any>>({})

onMounted(() => {
  loadSettings()
  // Live-update stats after a refresh completes
  Events.On('refresh:progress', (ev: any) => {
    if (ev?.data?.done) loadSettings()
  })
})

async function loadSettings() {
  loading.value = true
  try {
    const [dir, s, mCount, games] = await Promise.all([
      AppService.GetDataDir(),
      AppService.GetSettings(),
      AppService.GetMappingCount(),
      AppService.GetTotalGames(),
    ])
    dataDir.value = dir || ''
    settings.value = s || {}
    downloadDir.value = s?.download_dir || `${dir}/downloads`
    downloadDirInput.value = downloadDir.value
    mappingCount.value = mCount || 0
    totalGames.value = games || 0
  } catch (e) {
    console.error('Failed to load settings:', e)
  } finally {
    loading.value = false
  }
}

async function handleRefresh() {
  refreshing.value = true
  try {
    await store.refreshDataSync()
    await loadSettings()
    message.success(store.refreshSummary || '刷新完成')
  } catch (e) {
    console.error('Failed to refresh data:', e)
    message.error('刷新失败')
  } finally {
    refreshing.value = false
  }
}

function startEditDownloadDir() {
  downloadDirInput.value = downloadDir.value
  editingDownloadDir.value = true
}

function cancelEditDownloadDir() {
  editingDownloadDir.value = false
  downloadDirInput.value = downloadDir.value
}

async function saveDownloadDir() {
  const dir = downloadDirInput.value.trim()
  if (!dir) {
    message.warning('下载路径不能为空')
    return
  }
  try {
    await AppService.SetDownloadDir(dir)
    downloadDir.value = dir
    editingDownloadDir.value = false
    message.success('下载路径已保存')
  } catch (e) {
    console.error('Failed to set download dir:', e)
    message.error('保存失败：路径无效或不可写')
  }
}
</script>

<template>
  <div class="settings-view">
    <div class="page-header">
      <h2 class="page-title">设置</h2>
      <NButton :loading="refreshing" @click="handleRefresh" size="small">
        <template #icon>
          <NIcon><RefreshOutline /></NIcon>
        </template>
        刷新数据
      </NButton>
    </div>

    <NSpin :show="loading">
      <!-- Data section -->
      <NCard title="数据" size="small" style="margin-bottom: 16px;">
        <NDescriptions label-placement="left" :column="1" bordered size="small">
          <NDescriptionsItem label="数据目录">
            <span style="font-family: monospace; font-size: 13px;">{{ dataDir || '-' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="下载路径">
            <NSpace v-if="!editingDownloadDir" align="center" :wrap="false">
              <span style="font-family: monospace; font-size: 13px;">{{ downloadDir || '-' }}</span>
              <NButton size="tiny" quaternary @click="startEditDownloadDir">
                <template #icon><NIcon><FolderOpenOutline /></NIcon></template>
                修改
              </NButton>
            </NSpace>
            <NSpace v-else align="center" :wrap="false">
              <NInput
                v-model:value="downloadDirInput"
                size="small"
                style="width: 380px;"
                placeholder="输入下载目录绝对路径"
              />
              <NButton size="small" type="primary" @click="saveDownloadDir">保存</NButton>
              <NButton size="small" @click="cancelEditDownloadDir">取消</NButton>
            </NSpace>
          </NDescriptionsItem>
          <NDescriptionsItem label="名称映射">
            {{ mappingCount > 0 ? `已加载 ${mappingCount} 条中英文名称映射` : '未加载' }}
          </NDescriptionsItem>
          <NDescriptionsItem label="本地数据">
            {{ totalGames }} 个游戏
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <!-- About section -->
      <NCard title="关于" size="small">
        <NDescriptions label-placement="left" :column="1" bordered size="small">
          <NDescriptionsItem label="应用名称">GameModMaster</NDescriptionsItem>
          <NDescriptionsItem label="版本">1.0.0</NDescriptionsItem>
          <NDescriptionsItem label="数据来源">
            <a href="https://flingtrainer.com" target="_blank" style="color: #63e2b7;">FLiNG Trainer</a>
          </NDescriptionsItem>
          <NDescriptionsItem label="技术栈">Wails v3 + Go + Vue 3 + Naive UI</NDescriptionsItem>
        </NDescriptions>
      </NCard>
    </NSpin>
  </div>
</template>

<script lang="ts">
export default {
  name: 'SettingsView',
}
</script>

<style scoped>
.settings-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}
</style>
