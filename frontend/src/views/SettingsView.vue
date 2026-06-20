<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NButton,
  NIcon,
  NSpin,
  NSpace,
  NInput,
  useMessage,
} from 'naive-ui'
import { FolderOpenOutline, RefreshOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { Events, Dialogs } from '@wailsio/runtime'
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

onMounted(() => {
  loadSettings()
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
    downloadDir.value = s?.download_dir || `${dir}/downloads`
    downloadDirInput.value = downloadDir.value
    mappingCount.value = mCount || 0
    totalGames.value = games || 0
  } catch (e) {
    console.error('Failed to load settings:', e)
    message.error('加载设置失败')
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

// Open the native OS folder picker and fill the input with the choice.
async function browseDownloadDir() {
  try {
    const result = await Dialogs.OpenFile({
      Title: '选择下载目录',
      CanChooseDirectories: true,
      CanChooseFiles: false,
      CanCreateDirectories: true,
      Directory: downloadDirInput.value || downloadDir.value || '',
    })
    if (result) {
      // OpenFile may return a string or string[] depending on options
      const picked = Array.isArray(result) ? result[0] : result
      if (picked) downloadDirInput.value = picked
    }
  } catch (e) {
    console.error('browse dir failed:', e)
    message.error('打开文件夹选择器失败')
  }
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
    <div class="page-head">
      <div class="page-head-left">
        <h1 class="page-title">设置</h1>
      </div>
      <NButton :loading="refreshing" secondary @click="handleRefresh">
        <template #icon>
          <NIcon><RefreshOutline /></NIcon>
        </template>
        刷新数据
      </NButton>
    </div>

    <NSpin :show="loading">
      <!-- Storage section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">存储</h2>
          <span class="card-desc">数据保存在系统用户目录，不会跟随程序被删除</span>
        </header>
        <div class="kv-list">
          <div class="kv">
            <div class="kv-label">数据目录</div>
            <div class="kv-value mono">{{ dataDir || '-' }}</div>
          </div>
          <div class="kv">
            <div class="kv-label">下载路径</div>
            <div class="kv-value">
              <NSpace v-if="!editingDownloadDir" align="center" :wrap="false">
                <span class="mono">{{ downloadDir || '-' }}</span>
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
                <NButton size="small" @click="browseDownloadDir">
                  <template #icon><NIcon><FolderOpenOutline /></NIcon></template>
                  浏览
                </NButton>
                <NButton size="small" type="primary" @click="saveDownloadDir">保存</NButton>
                <NButton size="small" @click="cancelEditDownloadDir">取消</NButton>
              </NSpace>
            </div>
          </div>
        </div>
      </section>

      <!-- Data section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">数据统计</h2>
        </header>
        <div class="stat-grid">
          <div class="stat">
            <div class="stat-num">{{ totalGames }}</div>
            <div class="stat-label">本地游戏数</div>
          </div>
          <div class="stat">
            <div class="stat-num">{{ mappingCount }}</div>
            <div class="stat-label">名称映射条目</div>
          </div>
        </div>
      </section>

      <!-- About section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">关于</h2>
        </header>
        <div class="kv-list">
          <div class="kv">
            <div class="kv-label">应用名称</div>
            <div class="kv-value">GameModMaster</div>
          </div>
          <div class="kv">
            <div class="kv-label">版本</div>
            <div class="kv-value">1.0.0</div>
          </div>
          <div class="kv">
            <div class="kv-label">数据来源</div>
            <div class="kv-value">
              <a href="https://flingtrainer.com" target="_blank" class="link">FLiNG Trainer ↗</a>
            </div>
          </div>
          <div class="kv">
            <div class="kv-label">技术栈</div>
            <div class="kv-value">Wails v3 · Go · Vue 3 · Naive UI</div>
          </div>
        </div>
      </section>
    </NSpin>
  </div>
</template>

<script lang="ts">
export default { name: 'SettingsView' }
</script>

<style scoped>
.settings-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
  overflow-y: auto;
}

.page-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-shrink: 0;
}
.page-head-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
}
.page-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-1);
}

.card {
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  overflow: hidden;
}
.card-head {
  padding: 18px 24px 12px;
  border-bottom: 1px solid var(--border-soft);
}
.card-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-1);
  margin: 0;
}
.card-desc {
  display: block;
  font-size: 12px;
  color: var(--text-3);
  margin-top: 4px;
}

.kv-list {
  padding: 8px 24px 20px;
}
.kv {
  display: flex;
  align-items: center;
  padding: 14px 0;
  border-bottom: 1px solid rgba(148, 163, 184, 0.06);
  gap: 16px;
}
.kv:last-child {
  border-bottom: none;
}
.kv-label {
  width: 120px;
  flex-shrink: 0;
  font-size: 13px;
  color: var(--text-3);
}
.kv-value {
  flex: 1;
  min-width: 0;
  font-size: 14px;
  color: var(--text-1);
}
.mono {
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12.5px;
  color: var(--text-2);
  word-break: break-all;
}

.stat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
  padding: 20px 24px;
}
.stat {
  background: var(--surface-2);
  border-radius: 12px;
  padding: 18px 20px;
}
.stat-num {
  font-size: 28px;
  font-weight: 700;
  color: var(--accent);
  line-height: 1.2;
}
.stat-label {
  font-size: 13px;
  color: var(--text-3);
  margin-top: 4px;
}

.link {
  color: var(--accent);
  text-decoration: none;
}
.link:hover {
  text-decoration: underline;
}
</style>
