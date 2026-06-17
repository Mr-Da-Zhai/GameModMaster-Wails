<script setup lang="ts">
import { onMounted, onUnmounted, ref, h, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NDataTable,
  NInput,
  NButton,
  NTag,
  NIcon,
  NImage,
  NSpin,
  NEmpty,
  NSkeleton,
  NProgress,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshOutline, CloudDownloadOutline } from '@vicons/ionicons5'
import { useTrainerStore, type GameEntry } from '../stores/trainer'

const router = useRouter()
const store = useTrainerStore()

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  store.bindEvents()
  store.loadTrainers(1)
})

// When a refresh completes, reload the current page to pick up new data.
watch(
  () => store.refreshProgress.done,
  (done) => {
    if (done) store.onRefreshComplete()
  }
)

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

// First-run seeding uses the synchronous fetch so the empty state can be
// populated without an extra round-trip.
async function handleLoadData() {
  await store.refreshDataSync()
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

function formatFileSize(size: number): string {
  if (!size) return '-'
  if (size < 1024) return `${size}B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)}KB`
  return `${(size / (1024 * 1024)).toFixed(1)}MB`
}

function formatSpeed(speed: number): string {
  if (!speed) return '-'
  if (speed < 1024) return `${Math.round(speed)}B/s`
  if (speed < 1024 * 1024) return `${(speed / 1024).toFixed(1)}KB/s`
  return `${(speed / (1024 * 1024)).toFixed(1)}MB/s`
}

function handleRowClick(row: GameEntry) {
  router.push({ name: 'detail', params: { id: row.id } })
}

// Refresh progress display: "page 2/3 · 10 games"
const refreshStatus = computed(() => {
  const p = store.refreshProgress
  if (!p || !p.current) return ''
  const gamesPart = p.games ? `${p.games} 游戏` : ''
  const trainersPart = p.trainers ? `· ${p.trainers} 修改器` : ''
  return `抓取中 ${p.current}/${p.total || 3} ${gamesPart} ${trainersPart}`.trim()
})

const columns: DataTableColumns<GameEntry> = [
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
    key: 'display_name',
    minWidth: 200,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { fontWeight: '500' } }, row.display_name || row.name_en || 'Unknown')
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
    title: '版本',
    key: 'version',
    width: 100,
    ellipsis: { tooltip: true },
    render(row) {
      return row.latest_trainer?.version || '-'
    },
  },
  {
    title: '大小',
    key: 'file_size',
    width: 90,
    align: 'center',
    render(row) {
      return row.latest_trainer ? formatFileSize(row.latest_trainer.file_size) : '-'
    },
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
    width: 110,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, tertiary: true }
      const trainerId = row.latest_trainer?.id
      const prog = trainerId != null ? store.downloadProgress[trainerId] : undefined
      const downloading = !!prog && !prog.done
      if (row.status === 2 && row.latest_trainer) {
        return h(NButton, { ...btnProps, type: 'success', disabled: downloading, onClick: (e: Event) => { e.stopPropagation(); store.launchTrainer(row.latest_trainer!.id) } }, { default: () => '启动' })
      } else if (row.status === 1 && row.latest_trainer) {
        return h(NButton, { ...btnProps, type: 'info', disabled: downloading, onClick: (e: Event) => { e.stopPropagation(); store.installTrainer(row.latest_trainer!.id) } }, { default: () => '安装' })
      } else if (row.latest_trainer) {
        if (downloading && prog && prog.total && prog.downloaded != null) {
          const pct = Math.min(100, Math.round((prog.downloaded / prog.total) * 100))
          return h('span', { style: { fontSize: '12px', color: '#63e2b7' } }, `${pct}%`)
        }
        return h(NButton, {
          ...btnProps,
          type: 'primary',
          loading: downloading,
          disabled: downloading,
          onClick: (e: Event) => { e.stopPropagation(); store.downloadTrainer(row.latest_trainer!.id) },
        }, { default: () => '下载' })
      }
      return h('span', { style: { color: '#666' } }, '-')
    },
  },
]

const rowKey = (row: GameEntry) => row.id

const calcTableHeight = computed(() => window.innerHeight - 140)

const isEmpty = computed(() => !store.loading && !store.refreshing && store.trainers.length === 0)
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
        <span v-if="store.refreshing && refreshStatus" class="refresh-status">
          {{ refreshStatus }}
        </span>
        <NButton
          :loading="store.refreshing"
          quaternary
          circle
          @click="handleRefresh"
          title="刷新数据"
        >
          <template #icon>
            <NIcon><RefreshOutline /></NIcon>
          </template>
        </NButton>
      </div>
    </div>

    <!-- Refresh progress bar -->
    <div v-if="store.refreshing" class="refresh-bar">
      <NProgress
        type="line"
        :percentage="Math.min(100, Math.round(((store.refreshProgress.current || 0) / (store.refreshProgress.total || 3)) * 100))"
        :show-indicator="false"
        status="info"
      />
    </div>

    <!-- Loading skeleton -->
    <div v-if="store.loading && store.trainers.length === 0" class="skeleton-wrapper">
      <NSkeleton text :repeat="8" />
    </div>

    <!-- Empty state -->
    <NEmpty
      v-else-if="isEmpty"
      description="暂无数据，首次使用请加载数据"
      style="padding-top: 80px;"
    >
      <template #extra>
        <NButton type="primary" :loading="store.refreshing" @click="handleLoadData">
          <template #icon>
            <NIcon><CloudDownloadOutline /></NIcon>
          </template>
          加载数据
        </NButton>
      </template>
    </NEmpty>

    <!-- Table -->
    <NSpin v-else :show="store.loading">
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
        :row-props="(row: GameEntry) => ({ onClick: () => handleRowClick(row) })"
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

.skeleton-wrapper {
  padding: 20px 0;
}

.refresh-status {
  font-size: 12px;
  color: #999;
  margin-right: 8px;
  white-space: nowrap;
}

.refresh-bar {
  margin-bottom: 12px;
  padding: 0 4px;
}
</style>
