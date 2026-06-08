<script setup lang="ts">
import { onMounted, ref, h, computed } from 'vue'
import {
  NDataTable,
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
  NEmpty,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { PlayOutline, TrashOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import type { DownloadedTrainer } from '../stores/trainer'

const trainers = ref<DownloadedTrainer[]>([])
const loading = ref(false)

onMounted(() => {
  loadDownloaded()
})

async function loadDownloaded() {
  loading.value = true
  try {
    const result = await AppService.GetDownloadedTrainers()
    trainers.value = (result || []) as unknown as DownloadedTrainer[]
  } catch (e) {
    console.error('Failed to load downloaded trainers:', e)
  } finally {
    loading.value = false
  }
}

async function handleLaunch(trainerId: number) {
  try {
    await AppService.LaunchTrainer(trainerId)
  } catch (e) {
    console.error('Failed to launch trainer:', e)
  }
}

async function handleDelete(trainerId: number) {
  try {
    await AppService.DeleteTrainer(trainerId)
    await loadDownloaded()
  } catch (e) {
    console.error('Failed to delete trainer:', e)
  }
}

function getStatusInfo(status: number): { label: string; type: 'info' | 'success' } {
  switch (status) {
    case 2:
      return { label: '已安装', type: 'success' }
    default:
      return { label: '已下载', type: 'info' }
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

const columns: DataTableColumns<DownloadedTrainer> = [
  {
    title: '封面',
    key: 'cover_url',
    width: 56,
    render(row) {
      if (row.cover_url) {
        return h(NImage, {
          src: row.cover_url,
          width: 40,
          height: 40,
          objectFit: 'cover',
          style: { borderRadius: '4px' },
          previewSrc: '',
          showToolbar: false,
          fallbackSrc: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="40" height="40"><rect fill="%23334" width="40" height="40" rx="4"/></svg>',
        })
      }
      return h('div', {
        style: {
          width: '40px',
          height: '40px',
          borderRadius: '4px',
          background: '#334',
          display: 'inline-block',
        },
      })
    },
  },
  {
    title: '游戏名称',
    key: 'game_name',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { fontWeight: '500' } }, row.game_name || row.game_name_en || 'Unknown')
    },
  },
  {
    title: '版本',
    key: 'version',
    width: 100,
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
    title: '下载时间',
    key: 'installed_at',
    width: 120,
    align: 'center',
    render(row) {
      const ts = row.installed_at || row.updated_at
      return h('span', { style: { color: '#999', fontSize: '13px' } }, formatDate(ts))
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 130,
    align: 'center',
    fixed: 'right',
    render(row) {
      const buttons: any[] = []
      if (row.status === 2) {
        buttons.push(
          h(NButton, {
            size: 'small' as const,
            tertiary: true,
            type: 'success',
            onClick: (e: Event) => { e.stopPropagation(); handleLaunch(row.id) },
            style: { marginRight: '4px' },
          }, {
            icon: () => h(NIcon, null, { default: () => h(PlayOutline) }),
            default: () => '启动',
          })
        )
      }
      buttons.push(
        h(NButton, {
          size: 'small' as const,
          tertiary: true,
          type: 'error',
          onClick: (e: Event) => { e.stopPropagation(); handleDelete(row.id) },
        }, {
          icon: () => h(NIcon, null, { default: () => h(TrashOutline) }),
          default: () => '删除',
        })
      )
      return h('div', { style: { display: 'flex', justifyContent: 'center', gap: '4px' } }, buttons)
    },
  },
]

const rowKey = (row: DownloadedTrainer) => row.id
const calcTableHeight = computed(() => window.innerHeight - 140)
</script>

<template>
  <div class="downloads-view">
    <div class="page-header">
      <h2 class="page-title">已下载</h2>
    </div>

    <NSpin :show="loading">
      <NEmpty
        v-if="!loading && trainers.length === 0"
        description="暂无已下载的修改器"
        style="padding-top: 60px;"
      />
      <NDataTable
        v-else
        :columns="columns"
        :data="trainers"
        :row-key="rowKey"
        :max-height="calcTableHeight"
        :virtual-scroll="true"
        :bordered="false"
        :single-line="false"
        size="small"
      />
    </NSpin>
  </div>
</template>

<script lang="ts">
export default {
  name: 'DownloadsView',
}
</script>

<style scoped>
.downloads-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  margin-bottom: 16px;
  flex-shrink: 0;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}
</style>
