<script setup lang="ts">
import { onMounted, ref, h, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
  NEmpty,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NCard,
  NSpace,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { ArrowBackOutline, DownloadOutline, PlayOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { useTrainerStore } from '../stores/trainer'
import type { TrainerDetailResponse, TrainerDetail, GameDetail } from '../stores/trainer'

const route = useRoute()
const router = useRouter()
const store = useTrainerStore()

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
  }
}

async function handleInstall(trainerId: number) {
  try {
    await AppService.InstallTrainer(trainerId)
    await loadDetail()
  } catch (e) {
    console.error('Failed to install trainer:', e)
  }
}

async function handleLaunch(trainerId: number) {
  try {
    await AppService.LaunchTrainer(trainerId)
  } catch (e) {
    console.error('Failed to launch trainer:', e)
  }
}

const columns: DataTableColumns<TrainerDetail> = [
  {
    title: '修改器版本',
    key: 'version',
    width: 120,
    render(row) {
      return h('span', { style: { fontWeight: '500' } }, row.version || '-')
    },
  },
  {
    title: '游戏版本',
    key: 'game_version',
    minWidth: 200,
    ellipsis: { tooltip: true },
  },
  {
    title: '大小',
    key: 'file_size',
    width: 90,
    align: 'center',
    render(row) {
      return formatFileSize(row.file_size)
    },
  },
  {
    title: '下载次数',
    key: 'download_count',
    width: 100,
    align: 'center',
    sorter: (a, b) => a.download_count - b.download_count,
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 120,
    align: 'center',
    render(row) {
      return h('span', { style: { color: '#999', fontSize: '13px' } }, formatDate(row.updated_at))
    },
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    render(row) {
      const info = getStatusInfo(row.status)
      return h(NTag, { size: 'small', type: info.type, bordered: false }, { default: () => info.label })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 110,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, tertiary: true }
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
        return h('span', { style: { fontSize: '12px', color: '#63e2b7' } }, `${pct}%`)
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
const calcTableHeight = computed(() => window.innerHeight - 360)
</script>

<template>
  <div class="detail-view">
    <NSpin :show="loading">
      <!-- Back button -->
      <NButton quaternary size="small" @click="router.back()" style="margin-bottom: 16px;">
        <template #icon>
          <NIcon><ArrowBackOutline /></NIcon>
        </template>
        返回
      </NButton>

      <NEmpty v-if="!loading && !game" description="未找到游戏信息" />

      <template v-if="game">
        <!-- Game info card -->
        <NCard size="small" style="margin-bottom: 16px;">
          <div class="game-info">
            <div class="game-cover">
              <NImage
                v-if="game.cover_url"
                :src="game.cover_url"
                width="80"
                height="80"
                object-fit="cover"
                :preview-src="''"
                :show-toolbar="false"
                fallback-src="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' width='80' height='80'><rect fill='%23334' width='80' height='80' rx='8'/></svg>"
                style="border-radius: 8px;"
              />
              <div v-else class="cover-placeholder">80x80</div>
            </div>
            <div class="game-meta">
              <h2 class="game-title">{{ game.display_name || game.name_en }}</h2>
              <NDescriptions label-placement="left" :column="2" size="small">
                <NDescriptionsItem label="英文名">{{ game.name_en || '-' }}</NDescriptionsItem>
                <NDescriptionsItem label="选项数">{{ game.options_num || '-' }}</NDescriptionsItem>
                <NDescriptionsItem label="来源">
                  <a v-if="game.source_url" :href="game.source_url" target="_blank" style="color: #63e2b7;">查看原站</a>
                  <span v-else>-</span>
                </NDescriptionsItem>
                <NDescriptionsItem label="更新">{{ formatDate(game.updated_at) }}</NDescriptionsItem>
              </NDescriptions>
            </div>
          </div>
        </NCard>

        <!-- Trainer versions table -->
        <NCard size="small" title="修改器版本">
          <NDataTable
            :columns="columns"
            :data="trainers"
            :row-key="rowKey"
            :max-height="calcTableHeight"
            :bordered="false"
            :single-line="false"
            size="small"
          />
        </NCard>
      </template>
    </NSpin>
  </div>
</template>

<script lang="ts">
export default {
  name: 'DetailView',
}
</script>

<style scoped>
.detail-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.game-info {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.game-cover {
  flex-shrink: 0;
}

.cover-placeholder {
  width: 80px;
  height: 80px;
  border-radius: 8px;
  background: #334;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666;
  font-size: 12px;
}

.game-meta {
  flex: 1;
  min-width: 0;
}

.game-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 8px 0;
}
</style>
