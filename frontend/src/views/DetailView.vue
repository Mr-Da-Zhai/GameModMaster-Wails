<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref, h, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
  NEmpty,
  NDataTable,
  NResult,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { Events } from '@wailsio/runtime'
import { ArrowBackOutline, DownloadOutline, PlayOutline, GameControllerOutline, RefreshOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { useTrainerStore } from '../stores/trainer'
import { useFeedback } from '../composables/useConfirm'
import UsageGuide from '../components/UsageGuide.vue'
import type { TrainerDetailResponse, TrainerDetail, GameDetail } from '../stores/trainer'

const route = useRoute()
const router = useRouter()
const store = useTrainerStore()
const { confirm, toast } = useFeedback()

// loading: initial detail call in flight.
// fetching: background prefetch in flight (after the sentinel "not ready").
// fetchError: last error from a background prefetch, surfaced to the user.
const loading = ref(false)
const fetching = ref(false)
const fetchError = ref('')
const game = ref<GameDetail | null>(null)
const trainers = ref<TrainerDetail[]>([])

// One-shot detail:progress listener. The backend fires it with
// {game_id, done, error} when a background prefetch finishes.
function onDetailProgress(ev: any) {
  const raw = ev?.data
  const data = Array.isArray(raw) ? raw[0] : raw
  if (!data || Number(data.game_id) !== Number(route.params.id)) return
  if (!data.done) return
  fetching.value = false
  if (data.error) {
    fetchError.value = String(data.error)
    toast.error(`详情加载失败：${data.error}`)
  } else {
    fetchError.value = ''
    // Re-render now that trainer rows are cached.
    loadDetail(true)
  }
}

onMounted(() => {
  store.bindEvents()
  Events.On('detail:progress', onDetailProgress as any)
  loadDetail()
})

onBeforeUnmount(() => {
  Events.Off('detail:progress', onDetailProgress as any)
})

// loadDetail tries GetTrainerDetail. If trainer rows aren't cached yet, the
// backend returns the sentinel "detail not cached: prefetch in progress" —
// we then show the spinner and wait for the detail:progress done event.
// Pass silent=true to suppress the "no data" empty state during a re-render.
async function loadDetail(silent = false) {
  const gameId = Number(route.params.id)
  if (!gameId) return

  loading.value = true
  fetchError.value = ''
  try {
    const result = (await AppService.GetTrainerDetail(gameId)) as unknown as
      | TrainerDetailResponse
      | null
    if (result) {
      game.value = result.game as GameDetail
      trainers.value = (result.trainers || []) as TrainerDetail[]
      fetching.value = false
    }
  } catch (e: any) {
    const msg = String(e?.message || e)
    if (msg.includes('prefetch in progress') || msg.includes('detail not cached')) {
      // Sentinel: data isn't cached. Backend has kicked off a prefetch;
      // we'll auto-rerender when detail:progress fires.
      if (!silent) {
        fetching.value = true
        // Also ensure the prefetch is running (no-op if it already is).
        AppService.PrefetchTrainerDetail(gameId).catch(() => {})
      }
    } else {
      console.error('Failed to load trainer detail:', e)
      fetchError.value = msg
      if (!silent) toast.error(`加载详情失败：${msg}`)
    }
  } finally {
    loading.value = false
  }
}

// Manual retry button shown when a prefetch failed.
async function retryLoad() {
  fetching.value = true
  fetchError.value = ''
  const gameId = Number(route.params.id)
  try {
    await AppService.PrefetchTrainerDetail(gameId)
  } catch (e: any) {
    fetching.value = false
    toast.error(`重试失败：${e?.message || e}`)
  }
}

// latestGameVersion surfaces the newest trainer's game-version string (e.g.
// "Steam Xbox/Game Pass v1.0-v1.12+") for the header card so the user sees
// at a glance which game build this page targets. Trainers are newest-first
// by updated_at, so trainers[0] is the latest.
const latestGameVersion = computed(() => {
  if (!trainers.value.length) return ''
  return trainers.value[0]?.game_version || ''
})

function getStatusInfo(status: number): { label: string; type: 'default' | 'info' | 'success' } {
  switch (status) {
    case 1:
      return { label: '已下载', type: 'info' }
    case 2:
      return { label: '已安装', type: 'success' }
    default:
      return { label: '可用', type: 'default' }
  }
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '-'
  const d = new Date(timestamp * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function formatFileSize(size: number): string {
  if (!size) return '-'
  if (size < 1024) return `${size}B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)}KB`
  return `${(size / (1024 * 1024)).toFixed(1)}MB`
}

async function handleDownload(trainerId: number) {
  // Front-end dedupe: if a download is already in flight (either we have a
  // progress entry or the backend reports it as downloading), refuse. The
  // backend also rejects re-entry, but this avoids an extra round-trip and
  // keeps the button visually disabled.
  const prog = store.downloadProgress[trainerId]
  if (prog && !prog.done) {
    toast.warning('该修改器正在下载中')
    return
  }
  let alreadyDownloading = false
  try {
    alreadyDownloading = await AppService.IsDownloading(trainerId)
  } catch {
    // Non-fatal — fall through and let the backend reject if needed.
  }
  if (alreadyDownloading) {
    toast.warning('该修改器正在下载中')
    return
  }

  const ok = await confirm({
    title: '下载修改器',
    content: '将下载到您设置的下载目录中，下载完成后会自动解压并标记为已安装。是否继续？',
    type: 'info',
    positiveText: '下载',
  })
  if (!ok) return
  try {
    await AppService.DownloadTrainer(trainerId)
    toast.success('下载完成,已自动安装,可点击启动')
    await loadDetail(true)
  } catch (e: any) {
    const msg = String(e?.message || e)
    if (msg.includes('cancelled')) {
      toast.info('已取消下载')
    } else {
      console.error('Failed to download trainer:', e)
      toast.error(`下载失败：${msg}`)
    }
  }
}

async function handleInstall(trainerId: number) {
  // Legacy: with auto-install, downloads land in StatusInstalled directly
  // and this handler should never be reachable from the UI. Kept for safety
  // in case any cached trainer is still in StatusDownloaded.
  try {
    await AppService.InstallTrainer(trainerId)
    toast.success('已标记为已安装')
    await loadDetail(true)
  } catch (e: any) {
    console.error('Failed to install trainer:', e)
    toast.error(`安装失败：${e?.message || e}`)
  }
}

async function handleLaunch(trainerId: number) {
  try {
    await AppService.LaunchTrainer(trainerId)
    toast.success('已启动')
  } catch (e: any) {
    console.error('Failed to launch trainer:', e)
    toast.error(`启动失败：${e?.message || e}`)
  }
}

async function handleCancelDownload(trainerId: number) {
  try {
    await AppService.CancelDownload(trainerId)
    toast.info('已请求取消下载')
  } catch (e: any) {
    console.error('Failed to cancel download:', e)
  }
}

const columns: DataTableColumns<TrainerDetail> = [
  {
    title: '修改器版本',
    key: 'version',
    width: 130,
    render(row) {
      return h('span', { style: { fontWeight: '600', color: 'var(--text-1)' } }, row.version || '-')
    },
  },
  {
    title: '游戏版本',
    key: 'game_version',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { color: 'var(--text-2)' } }, row.game_version || '-')
    },
  },
  {
    title: '大小',
    key: 'file_size',
    width: 90,
    align: 'center',
    render(row) {
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, formatFileSize(row.file_size))
    },
  },
  {
    title: '下载次数',
    key: 'download_count',
    width: 100,
    align: 'center',
    sorter: (a, b) => a.download_count - b.download_count,
    render(row) {
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, String(row.download_count))
    },
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 120,
    align: 'center',
    render(row) {
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, formatDate(row.updated_at))
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render(row) {
      const info = getStatusInfo(row.status)
      return h(NTag, { size: 'small', type: info.type, bordered: false, round: true }, { default: () => info.label })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 130,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, secondary: true }
      const prog = store.downloadProgress[row.id]
      const downloading = !!prog && !prog.done
      // Status 2 = installed → launch.
      // Status 1 = downloaded but not installed (only legacy rows) → install.
      // Status 0 / downloading → download (or progress).
      if (row.status === 2) {
        return h(NButton, { ...btnProps, type: 'success', disabled: downloading, onClick: () => handleLaunch(row.id) }, {
          icon: () => h(NIcon, null, { default: () => h(PlayOutline) }),
          default: () => '启动',
        })
      } else if (row.status === 1) {
        return h(NButton, { ...btnProps, type: 'info', disabled: downloading, onClick: () => handleInstall(row.id) }, { default: () => '安装' })
      }
      if (downloading && prog && prog.total && prog.downloaded != null) {
        const pct = Math.min(100, Math.round((prog.downloaded / prog.total) * 100))
        return h(
          'div',
          { style: { display: 'flex', alignItems: 'center', gap: '6px', justifyContent: 'center' } },
          [
            h('span', { style: { fontSize: '12px', color: 'var(--accent)', fontWeight: '600' } }, `${pct}%`),
            h(
              NButton,
              { size: 'tiny', quaternary: true, type: 'error', onClick: () => handleCancelDownload(row.id) },
              { default: () => '取消' },
            ),
          ],
        )
      }
      return h(NButton, {
        ...btnProps,
        type: 'primary',
        loading: downloading,
        disabled: downloading,
        onClick: () => handleDownload(row.id),
      }, {
        icon: () => h(NIcon, null, { default: () => h(DownloadOutline) }),
        default: () => '下载',
      })
    },
  },
]

const rowKey = (row: TrainerDetail) => row.id
</script>

<template>
  <div class="detail-view">
    <NButton quaternary size="small" @click="router.back()" class="back-btn">
      <template #icon>
        <NIcon><ArrowBackOutline /></NIcon>
      </template>
      返回
    </NButton>

    <NSpin :show="loading || fetching">
      <NEmpty v-if="!loading && !fetching && !game" description="未找到游戏信息" />

      <!-- Background prefetch failed: offer manual retry. -->
      <div
        v-else-if="!loading && !fetching && fetchError"
        class="retry-card"
      >
        <NResult
          status="error"
          title="详情加载失败"
          :description="fetchError"
        >
          <template #footer>
            <NButton type="primary" @click="retryLoad">
              <template #icon>
                <NIcon><RefreshOutline /></NIcon>
              </template>
              重试
            </NButton>
          </template>
        </NResult>
      </div>

      <!-- Background prefetch in flight: explain why the table is empty. -->
      <div v-else-if="fetching && game && trainers.length === 0" class="fetching-card">
        <NSpin size="small" />
        <span>正在从原站获取修改器版本…</span>
      </div>

      <template v-if="game">
        <!-- Game info hero: Apple-style. Big cover on the left, generous
             whitespace, a Light-weight hero title, and a quiet meta row
             separated by middle dots. No boxed card — just space + type. -->
        <div class="game-info-card">
          <div class="game-cover">
            <NImage
              v-if="game.cover_url"
              :src="game.cover_url"
              width="128"
              height="128"
              object-fit="cover"
              :preview-src="''"
              :show-toolbar="false"
              fallback-src="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='128' height='128'><rect fill='%231f1f23' width='128' height='128' rx='20'/></svg>"
              class="cover-img"
            />
            <div v-else class="cover-placeholder">
              <NIcon size="40" color="#64748b"><GameControllerOutline /></NIcon>
            </div>
          </div>
          <div class="game-meta">
            <h1 class="game-title">{{ game.display_name || game.name_en }}</h1>
            <div class="game-sub" v-if="game.name_local && game.name_local !== game.display_name">{{ game.name_en }}</div>
            <div class="meta-row">
              <span v-if="latestGameVersion" class="meta-val">{{ latestGameVersion }}</span>
              <span v-if="latestGameVersion" class="meta-dot">·</span>
              <span class="meta-val">{{ game.options_num || 0 }} 项</span>
              <span class="meta-dot">·</span>
              <span class="meta-val">{{ formatDate(game.updated_at) }}</span>
              <a v-if="game.source_url" :href="game.source_url" target="_blank" class="source-link">查看原站 →</a>
            </div>
          </div>
        </div>

        <!-- Trainer versions -->
        <div class="versions-card">
          <div class="versions-head">
            <h2 class="section-title">修改器版本</h2>
            <span class="version-count">{{ trainers.length }} 个版本</span>
          </div>
          <div class="versions-table-wrap">
            <NDataTable
              :columns="columns"
              :data="trainers"
              :row-key="rowKey"
              :max-height="360"
              :bordered="false"
              :single-line="false"
              size="small"
            />
          </div>
        </div>

        <!-- Usage guide: collapsible Chinese/English keyboard reference -->
        <UsageGuide v-if="trainers.length > 0" />
      </template>
    </NSpin>
  </div>
</template>

<script lang="ts">
export default { name: 'DetailView' }
</script>

<style scoped>
.detail-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
}
.back-btn {
  align-self: flex-start;
}

.retry-card,
.fetching-card {
  padding: 36px 24px;
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  border-radius: 14px;
}
.fetching-card {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: var(--text-3);
  font-size: 14px;
  padding: 24px;
}

/* Hero game-info: no boxed card — just space + type (Apple-style). The
   cover sits left, the title is a large Light-weight hero, and the meta
   row uses middle dots instead of label/value pairs. */
.game-info-card {
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 8px 4px 28px;
}
.game-cover {
  flex-shrink: 0;
}
.cover-img {
  border-radius: 20px !important;
}
.cover-placeholder {
  width: 128px;
  height: 128px;
  border-radius: 20px;
  background: var(--surface-2);
  display: flex;
  align-items: center;
  justify-content: center;
}
.game-meta {
  flex: 1;
  min-width: 0;
}
.game-title {
  font-size: 32px;
  font-weight: 300; /* Light — Apple hero */
  color: var(--text-1);
  margin: 0;
  letter-spacing: -0.4px;
  line-height: 1.15;
}
.game-sub {
  font-size: 13px;
  color: var(--text-3);
  margin-top: 6px;
  font-weight: 400;
}
.meta-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
  flex-wrap: wrap;
}
.meta-val {
  font-size: 13px;
  color: var(--text-2);
  font-weight: 400;
}
.meta-dot {
  color: var(--text-3);
  opacity: 0.6;
}
.source-link {
  margin-left: auto;
  font-size: 13px;
  color: var(--accent);
  text-decoration: none;
  font-weight: 500;
}
.source-link:hover {
  color: var(--accent-hover);
}

.versions-card {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  border-radius: 14px;
  overflow: hidden;
}
.versions-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 12px;
  flex-shrink: 0;
}
.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-1);
  margin: 0;
}
.version-count {
  font-size: 12px;
  color: var(--text-3);
}
.versions-table-wrap {
  flex: 1;
  min-height: 0;
  padding: 0 8px 8px;
}
</style>
