<script setup lang="ts">
import { onMounted, ref, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
  NEmpty,
  NDataTable,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { ArrowBackOutline, DownloadOutline, PlayOutline, GameControllerOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { useTrainerStore } from '../stores/trainer'
import type { TrainerDetailResponse, TrainerDetail, GameDetail } from '../stores/trainer'

const route = useRoute()
const router = useRouter()
const store = useTrainerStore()
const message = useMessage()

const loading = ref(false)
const game = ref<GameDetail | null>(null)
const trainers = ref<TrainerDetail[]>([])

onMounted(() => {
  store.bindEvents()
  loadDetail()
})

async function loadDetail() {
  const gameId = Number(route.params.id)
  if (!gameId) return

  loading.value = true
  try {
    const result = await AppService.GetTrainerDetail(gameId) as unknown as TrainerDetailResponse
    if (result) {
      game.value = result.game as GameDetail
      trainers.value = (result.trainers || []) as TrainerDetail[]
    }
  } catch (e) {
    console.error('Failed to load trainer detail:', e)
    message.error('加载详情失败')
  } finally {
    loading.value = false
  }
}

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
  try {
    await AppService.DownloadTrainer(trainerId)
    await loadDetail()
  } catch (e) {
    console.error('Failed to download trainer:', e)
    message.error('下载失败')
  }
}

async function handleInstall(trainerId: number) {
  try {
    await AppService.InstallTrainer(trainerId)
    await loadDetail()
  } catch (e) {
    console.error('Failed to install trainer:', e)
    message.error('安装失败')
  }
}

async function handleLaunch(trainerId: number) {
  try {
    await AppService.LaunchTrainer(trainerId)
  } catch (e) {
    console.error('Failed to launch trainer:', e)
    message.error('启动失败')
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
    width: 110,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, secondary: true }
      const prog = store.downloadProgress[row.id]
      const downloading = !!prog && !prog.done
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
        return h('span', { style: { fontSize: '12px', color: 'var(--accent)', fontWeight: '600' } }, `${pct}%`)
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

    <NSpin :show="loading">
      <NEmpty v-if="!loading && !game" description="未找到游戏信息" />

      <template v-if="game">
        <!-- Game info -->
        <div class="game-info-card">
          <div class="game-cover">
            <NImage
              v-if="game.cover_url"
              :src="game.cover_url"
              width="96"
              height="96"
              object-fit="cover"
              :preview-src="''"
              :show-toolbar="false"
              fallback-src="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='96' height='96'><rect fill='%23334155' width='96' height='96' rx='14'/></svg>"
              class="cover-img"
            />
            <div v-else class="cover-placeholder">
              <NIcon size="32" color="#64748b"><GameControllerOutline /></NIcon>
            </div>
          </div>
          <div class="game-meta">
            <h1 class="game-title">{{ game.display_name || game.name_en }}</h1>
            <div class="game-sub" v-if="game.name_local && game.name_local !== game.display_name">{{ game.name_en }}</div>
            <div class="meta-row">
              <span class="meta-item"><span class="meta-label">选项</span><span class="meta-val">{{ game.options_num || '-' }}</span></span>
              <span class="meta-dot">·</span>
              <span class="meta-item"><span class="meta-label">更新</span><span class="meta-val">{{ formatDate(game.updated_at) }}</span></span>
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

.game-info-card {
  display: flex;
  gap: 20px;
  align-items: flex-start;
  padding: 24px;
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  border-radius: 14px;
}
.game-cover {
  flex-shrink: 0;
}
.cover-img {
  border-radius: 14px !important;
}
.cover-placeholder {
  width: 96px;
  height: 96px;
  border-radius: 14px;
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
  font-size: 24px;
  font-weight: 700;
  color: var(--text-1);
  margin: 0;
  letter-spacing: 0.2px;
}
.game-sub {
  font-size: 13px;
  color: var(--text-3);
  margin-top: 4px;
}
.meta-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 16px;
  flex-wrap: wrap;
}
.meta-item {
  display: flex;
  align-items: baseline;
  gap: 6px;
}
.meta-label {
  font-size: 12px;
  color: var(--text-3);
}
.meta-val {
  font-size: 14px;
  color: var(--text-1);
  font-weight: 600;
}
.meta-dot {
  color: var(--text-3);
}
.source-link {
  margin-left: auto;
  font-size: 13px;
  color: var(--accent);
  text-decoration: none;
}
.source-link:hover {
  text-decoration: underline;
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
