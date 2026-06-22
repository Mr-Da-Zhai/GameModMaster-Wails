<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NIcon,
  NSpin,
  NSpace,
  NInput,
  NSelect,
} from 'naive-ui'
import { FolderOpenOutline, RefreshOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { Events, Dialogs } from '@wailsio/runtime'
import { useTrainerStore } from '../stores/trainer'
import { useFeedback } from '../composables/useConfirm'
import { useLocale, type AppLocale } from '../i18n'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const { toast } = useFeedback()
const store = useTrainerStore()
const { locale, setLocale } = useLocale()

const loading = ref(false)
const refreshing = ref(false)
const dataDir = ref('')
const downloadDir = ref('')
const downloadDirInput = ref('')
const editingDownloadDir = ref(false)
const mappingCount = ref(0)
const totalGames = ref(0)

const localeOptions = computed(() => [
  { label: t('settings.languageZh'), value: 'zh-CN' as AppLocale },
  { label: t('settings.languageEn'), value: 'en' as AppLocale },
])

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
  } catch (e: any) {
    console.error('Failed to load settings:', e)
    toast.error(`${e?.message || e}`)
  } finally {
    loading.value = false
  }
}

async function handleRefresh() {
  refreshing.value = true
  try {
    await store.refreshDataSync()
    await loadSettings()
    toast.success(store.refreshSummary || 'ok')
  } catch (e: any) {
    console.error('Failed to refresh data:', e)
    toast.error(`${e?.message || e}`)
  } finally {
    refreshing.value = false
  }
}

function onChangeLocale(v: AppLocale) {
  setLocale(v)
}

function startEditDownloadDir() {
  downloadDirInput.value = downloadDir.value
  editingDownloadDir.value = true
}

// Open the native OS folder picker and fill the input with the choice.
async function browseDownloadDir() {
  try {
    const result = await Dialogs.OpenFile({
      Title: t('settings.pickFolder'),
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
  } catch (e: any) {
    console.error('browse dir failed:', e)
    toast.error(`${e?.message || e}`)
  }
}

function cancelEditDownloadDir() {
  editingDownloadDir.value = false
  downloadDirInput.value = downloadDir.value
}

async function saveDownloadDir() {
  const dir = downloadDirInput.value.trim()
  if (!dir) {
    toast.warning(t('settings.downloadDir'))
    return
  }
  try {
    await AppService.SetDownloadDir(dir)
    downloadDir.value = dir
    editingDownloadDir.value = false
    toast.success('ok')
  } catch (e: any) {
    console.error('Failed to set download dir:', e)
    toast.error(`${e?.message || e}`)
  }
}
</script>

<template>
  <div class="settings-view">
    <div class="page-head">
      <div class="page-head-left">
        <h1 class="page-title">{{ t('settings.title') }}</h1>
      </div>
      <NButton :loading="refreshing" secondary @click="handleRefresh">
        <template #icon>
          <NIcon><RefreshOutline /></NIcon>
        </template>
        {{ t('home.refresh') }}
      </NButton>
    </div>

    <NSpin :show="loading">
      <!-- Preferences section: language -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">{{ t('settings.language') }}</h2>
        </header>
        <div class="kv-list">
          <div class="kv">
            <div class="kv-label">{{ t('settings.language') }}</div>
            <div class="kv-value" style="max-width: 260px;">
              <NSelect
                :value="locale"
                :options="localeOptions"
                @update:value="onChangeLocale"
                size="small"
              />
            </div>
          </div>
        </div>
      </section>

      <!-- Storage section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">{{ t('settings.dataDir') }}</h2>
        </header>
        <div class="kv-list">
          <div class="kv">
            <div class="kv-label">{{ t('settings.dataDir') }}</div>
            <div class="kv-value mono">{{ dataDir || '-' }}</div>
          </div>
          <div class="kv">
            <div class="kv-label">{{ t('settings.downloadDir') }}</div>
            <div class="kv-value">
              <NSpace v-if="!editingDownloadDir" align="center" :wrap="false">
                <span class="mono">{{ downloadDir || '-' }}</span>
                <NButton size="tiny" quaternary @click="startEditDownloadDir">
                  <template #icon><NIcon><FolderOpenOutline /></NIcon></template>
                  ✎
                </NButton>
              </NSpace>
              <NSpace v-else align="center" :wrap="false">
                <NInput
                  v-model:value="downloadDirInput"
                  size="small"
                  style="width: 380px;"
                  :placeholder="t('settings.downloadDir')"
                />
                <NButton size="small" @click="browseDownloadDir">
                  <template #icon><NIcon><FolderOpenOutline /></NIcon></template>
                  {{ t('settings.pickFolder') }}
                </NButton>
                <NButton size="small" type="primary" @click="saveDownloadDir">{{ t('common.confirm') }}</NButton>
                <NButton size="small" @click="cancelEditDownloadDir">{{ t('common.cancel') }}</NButton>
              </NSpace>
            </div>
          </div>
        </div>
      </section>

      <!-- Data section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">{{ t('settings.stats') }}</h2>
        </header>
        <div class="stat-grid">
          <div class="stat">
            <div class="stat-num">{{ totalGames }}</div>
            <div class="stat-label">{{ t('home.title') }}</div>
          </div>
          <div class="stat">
            <div class="stat-num">{{ mappingCount }}</div>
            <div class="stat-label">{{ t('settings.mappingCount') }}</div>
          </div>
        </div>
      </section>

      <!-- About section -->
      <section class="card">
        <header class="card-head">
          <h2 class="card-title">{{ t('app.name') }}</h2>
        </header>
        <div class="kv-list">
          <div class="kv">
            <div class="kv-label">{{ t('app.name') }}</div>
            <div class="kv-value">GameModMaster</div>
          </div>
          <div class="kv">
            <div class="kv-label">v</div>
            <div class="kv-value">1.0.0</div>
          </div>
          <div class="kv">
            <div class="kv-label">{{ t('app.dataSource') }}</div>
            <div class="kv-value">
              <a href="https://flingtrainer.com" target="_blank" class="link">FLiNG Trainer ↗</a>
            </div>
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
