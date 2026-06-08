<script setup lang="ts">
import { onMounted, ref, h, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  NDataTable,
  NInput,
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import { useTrainerStore, type TrainerItem } from '../stores/trainer'

const router = useRouter()
const store = useTrainerStore()

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  store.loadTrainers(1)
})

function handleSearch(query: string) {
  searchValue.value = query
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    store.searchTrainers(query)
  }, 300)
}

function handleRefresh() {
  store.refreshData()
}

function getGameName(row: TrainerItem): string {
  return row.name_local || row.name_en || 'Unknown'
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
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function handleRowClick(row: TrainerItem) {
  router.push({ name: 'detail', params: { id: row.game_id } })
}

const columns: DataTableColumns<TrainerItem> = [
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
    key: 'name',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { fontWeight: '500' } }, getGameName(row))
    },
  },
  {
    title: '选项数',
    key: 'options_num',
    width: 80,
    align: 'center',
    sorter: (a, b) => a.options_num - b.options_num,
  },
  {
    title: '状态',
    key: 'status',
    width: 90,
    align: 'center',
    sorter: (a, b) => a.status - b.status,
    render(row) {
      const info = getStatusInfo(row.status)
      return h(NTag, { size: 'small', type: info.type, bordered: false }, { default: () => info.label })
    },
  },
  {
    title: '更新',
    key: 'updated_at',
    width: 80,
    align: 'center',
    sorter: (a, b) => a.updated_at - b.updated_at,
    render(row) {
      return h('span', { style: { color: '#999', fontSize: '13px' } }, formatDate(row.updated_at))
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, tertiary: true }
      if (row.status === 2) {
        // 已安装 → 启动
        return h(NButton, { ...btnProps, type: 'success', onClick: (e: Event) => { e.stopPropagation(); store.launchTrainer(row.trainer_id) } }, { default: () => '启动' })
      } else if (row.status === 1) {
        // 已下载 → 安装
        return h(NButton, { ...btnProps, type: 'info', onClick: (e: Event) => { e.stopPropagation(); store.installTrainer(row.trainer_id) } }, { default: () => '安装' })
      } else {
        // 可用 → 下载
        return h(NButton, { ...btnProps, type: 'primary', onClick: (e: Event) => { e.stopPropagation(); store.downloadTrainer(row.trainer_id) } }, { default: () => '下载' })
      }
    },
  },
]

const rowKey = (row: TrainerItem) => `${row.game_id}-${row.trainer_id}`

const calcTableHeight = computed(() => window.innerHeight - 140)
</script>

<template>
  <div class="home-view">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <NInput
          :value="searchValue"
          placeholder="搜索游戏名称..."
          clearable
          style="width: 300px"
          @update:value="handleSearch"
        />
      </div>
      <div class="toolbar-right">
        <NButton
          :loading="store.refreshing"
          quaternary
          circle
          @click="handleRefresh"
        >
          <template #icon>
            <NIcon><RefreshOutline /></NIcon>
          </template>
        </NButton>
      </div>
    </div>

    <!-- Table -->
    <NSpin :show="store.loading">
      <NDataTable
        :columns="columns"
        :data="store.trainers"
        :row-key="rowKey"
        :max-height="calcTableHeight"
        :virtual-scroll="true"
        :bordered="false"
        :single-line="false"
        size="small"
        style="cursor: pointer;"
        :row-props="(row: TrainerItem) => ({ onClick: () => handleRowClick(row) })"
      />
    </NSpin>
  </div>
</template>

<script lang="ts">
export default {
  name: 'HomeView',
}
</script>

<style scoped>
.home-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.toolbar-left {
  display: flex;
  gap: 8px;
  align-items: center;
}

.toolbar-right {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
