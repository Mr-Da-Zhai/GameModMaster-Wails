<script setup lang="ts">
import { onMounted, ref, h } from 'vue'
import {
  NDataTable,
  NButton,
  NTag,
  NIcon,
  NImage,
  NEmpty,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { PlayOutline, TrashOutline } from '@vicons/ionicons5'
import { useMessage } from 'naive-ui'
import * as AppService from '../../bindings/GameModMaster/appservice'
import type { DownloadedTrainer } from '../stores/trainer'

const message = useMessage()
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
    message.error('加载列表失败')
  } finally {
    loading.value = false
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

async function handleDelete(trainerId: number) {
  try {
    await AppService.DeleteTrainer(trainerId)
    await loadDownloaded()
    message.success('已删除')
  } catch (e) {
    console.error('Failed to delete trainer:', e)
    message.error('删除失败')
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
    title: '游戏',
    key: 'game_name',
    minWidth: 280,
    ellipsis: { tooltip: true },
    render(row) {
      const children: any[] = []
      if (row.cover_url) {
        children.push(
          h(NImage, {
            src: row.cover_url,
            width: 42,
            height: 42,
            objectFit: 'cover',
            style: { borderRadius: '8px', flexShrink: '0' },
            previewSrc: '',
            showToolbar: false,
            fallbackSrc: 'data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="42" height="42"><rect fill="%23334155" width="42" height="42" rx="8"/></svg>',
          })
        )
      } else {
        children.push(h('div', { style: { width: '42px', height: '42px', borderRadius: '8px', background: '#334155', flexShrink: '0' } }))
      }
      const nameBox = h('div', { style: { display: 'flex', flexDirection: 'column', gap: '2px', minWidth: '0' } }, [
        h('span', { style: { fontWeight: '600', color: 'var(--text-1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, row.game_name || row.game_name_en || 'Unknown'),
        row.game_name_en && row.game_name_en !== row.game_name
          ? h('span', { style: { fontSize: '11px', color: 'var(--text-3)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, row.game_name_en)
          : null,
      ])
      children.push(nameBox)
      return h('div', { style: { display: 'flex', alignItems: 'center', gap: '12px' } }, children)
    },
  },
  {
    title: '版本',
    key: 'version',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { color: 'var(--text-2)', fontSize: '13px' } }, row.version || '-')
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
    title: '下载时间',
    key: 'installed_at',
    width: 120,
    align: 'center',
    render(row) {
      const ts = row.installed_at || row.updated_at
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, formatDate(ts))
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
            secondary: true,
            type: 'success',
            onClick: (e: Event) => { e.stopPropagation(); handleLaunch(row.id) },
          }, {
            icon: () => h(NIcon, null, { default: () => h(PlayOutline) }),
            default: () => '启动',
          })
        )
      }
      buttons.push(
        h(NButton, {
          size: 'small' as const,
          secondary: true,
          type: 'error',
          onClick: (e: Event) => { e.stopPropagation(); handleDelete(row.id) },
        }, {
          icon: () => h(NIcon, null, { default: () => h(TrashOutline) }),
          default: () => '删除',
        })
      )
      return h('div', { style: { display: 'flex', justifyContent: 'center', gap: '6px' } }, buttons)
    },
  },
]

const rowKey = (row: DownloadedTrainer) => row.id
</script>

<template>
  <div class="downloads-view">
    <div class="page-head">
      <div class="page-head-left">
        <h1 class="page-title">我的修改器</h1>
        <span class="page-count">{{ trainers.length }} 个</span>
      </div>
    </div>

    <div class="table-area">
      <NEmpty
        v-if="!loading && trainers.length === 0"
        description="还没有下载任何修改器"
        class="empty-state"
      />
      <NDataTable
        v-else
        :columns="columns"
        :data="trainers"
        :row-key="rowKey"
        flex-height
        :virtual-scroll="true"
        :bordered="false"
        :single-line="false"
        size="small"
        class="data-table"
      />
    </div>
  </div>
</template>

<script lang="ts">
export default { name: 'DownloadsView' }
</script>

<style scoped>
.downloads-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
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
.page-count {
  font-size: 13px;
  color: var(--text-3);
}

.table-area {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  border-radius: 12px;
  overflow: hidden;
}
.data-table {
  flex: 1;
  min-height: 0;
}
.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
