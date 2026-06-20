<script setup lang="ts">
import { onMounted, ref, h, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NDataTable,
  NInput,
  NButton,
  NTag,
  NIcon,
  NImage,
  NEmpty,
  NPagination,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { RefreshOutline, CloudDownloadOutline, SearchOutline } from '@vicons/ionicons5'
import { useTrainerStore, type GameEntry } from '../stores/trainer'

const router = useRouter()
const store = useTrainerStore()

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  store.bindEvents()
  store.loadTrainers(1)
})

// Auto-reload the list when an async refresh completes.
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

async function handleLoadData() {
  await store.refreshDataSync()
}

function handlePageChange(page: number) {
  store.loadTrainers(page)
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

function handleRowClick(row: GameEntry) {
  router.push({ name: 'detail', params: { id: row.id } })
}

const refreshStatus = computed(() => {
  const p = store.refreshProgress
  if (!p || !p.current) return ''
  const gamesPart = p.games ? `${p.games} 游戏` : ''
  const trainersPart = p.trainers ? `· ${p.trainers} 修改器` : ''
  return `抓取中 ${p.current}/${p.total || 3} ${gamesPart} ${trainersPart}`.trim()
})

const columns: DataTableColumns<GameEntry> = [
  {
    title: '游戏',
    key: 'name',
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
        children.push(
          h('div', { style: { width: '42px', height: '42px', borderRadius: '8px', background: '#334155', flexShrink: '0' } })
        )
      }
      const nameBox = h('div', { style: { display: 'flex', flexDirection: 'column', gap: '2px', minWidth: '0' } }, [
        h('span', { style: { fontWeight: '600', color: 'var(--text-1)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, row.display_name || row.name_en || 'Unknown'),
        row.name_local && row.name_local !== row.display_name
          ? h('span', { style: { fontSize: '11px', color: 'var(--text-3)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' } }, row.name_en)
          : null,
      ])
      children.push(nameBox)
      return h('div', { style: { display: 'flex', alignItems: 'center', gap: '12px' } }, children)
    },
  },
  {
    title: '选项',
    key: 'options_num',
    width: 70,
    align: 'center',
    sorter: (a, b) => a.options_num - b.options_num,
    render(row) {
      return row.options_num
        ? h('span', { style: { color: 'var(--text-2)', fontWeight: '500' } }, `${row.options_num}`)
        : h('span', { style: { color: 'var(--text-3)' } }, '-')
    },
  },
  {
    title: '版本',
    key: 'version',
    width: 110,
    ellipsis: { tooltip: true },
    render(row) {
      return h('span', { style: { color: 'var(--text-2)', fontSize: '13px' } }, row.latest_trainer?.version || '-')
    },
  },
  {
    title: '大小',
    key: 'file_size',
    width: 90,
    align: 'center',
    render(row) {
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, row.latest_trainer ? formatFileSize(row.latest_trainer.file_size) : '-')
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
      return h(NTag, { size: 'small', type: info.type, bordered: false, round: true }, { default: () => info.label })
    },
  },
  {
    title: '更新',
    key: 'updated_at',
    width: 80,
    align: 'center',
    sorter: (a, b) => a.updated_at - b.updated_at,
    render(row) {
      return h('span', { style: { color: 'var(--text-3)', fontSize: '13px' } }, formatDate(row.updated_at))
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 96,
    align: 'center',
    fixed: 'right',
    render(row) {
      const btnProps = { size: 'small' as const, secondary: true }
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
          return h('span', { style: { fontSize: '12px', color: 'var(--accent)', fontWeight: '600' } }, `${pct}%`)
        }
        return h(NButton, {
          ...btnProps,
          type: 'primary',
          loading: downloading,
          disabled: downloading,
          onClick: (e: Event) => { e.stopPropagation(); store.downloadTrainer(row.latest_trainer!.id) },
        }, { default: () => '下载' })
      }
      return h('span', { style: { color: 'var(--text-3)' } }, '-')
    },
  },
]

const rowKey = (row: GameEntry) => row.id

// Total games drives the pager. When searching we show the result slice only.
const totalForPager = computed(() => store.totalCount)

const isEmpty = computed(
  () => !store.loading && !store.refreshing && store.trainers.length === 0
)
</script>

<template>
  <div class="home-view">
    <!-- Page header -->
    <div class="page-head">
      <div class="page-head-left">
        <h1 class="page-title">游戏列表</h1>
        <span class="page-count">{{ store.totalCount }} 款游戏</span>
      </div>
      <div class="page-head-right">
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

    <!-- Toolbar -->
    <div class="toolbar">
      <NInput
        :value="searchValue"
        placeholder="搜索游戏名称（支持中英文）…"
        clearable
        size="medium"
        class="search-input"
        @update:value="handleSearch"
      >
        <template #prefix>
          <NIcon :component="SearchOutline" class="search-icon" />
        </template>
      </NInput>
    </div>

    <!-- Body: flex column; the table area grows to fill -->
    <div class="table-area">
      <!-- Empty state -->
      <NEmpty
        v-if="isEmpty"
        description="正在从服务器获取数据，请稍候…"
        class="empty-state"
      >
        <template #extra>
          <NButton type="primary" :loading="store.refreshing" @click="handleLoadData">
            <template #icon>
              <NIcon><CloudDownloadOutline /></NIcon>
            </template>
            手动加载
          </NButton>
        </template>
      </NEmpty>

      <!-- Table fills the available height via flex-height (responsive to resize) -->
      <NDataTable
        v-else
        :columns="columns"
        :data="store.trainers"
        :row-key="rowKey"
        flex-height
        :virtual-scroll="true"
        :bordered="false"
        :single-line="false"
        size="small"
        class="data-table"
        :row-props="(row: GameEntry) => ({ onClick: () => handleRowClick(row) })"
      />
    </div>

    <!-- Pagination -->
    <div v-if="totalForPager > 0 && !isEmpty" class="pager">
      <NPagination
        :page="store.currentPage"
        :item-count="totalForPager"
        :page-size="store.pageSize"
        :page-slot="7"
        show-quick-jumper
        @update:page="handlePageChange"
      />
    </div>
  </div>
</template>

<script lang="ts">
export default { name: 'HomeView' }
</script>

<style scoped>
.home-view {
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
  letter-spacing: 0.2px;
}
.page-count {
  font-size: 13px;
  color: var(--text-3);
}
.page-head-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.refresh-status {
  font-size: 12px;
  color: var(--accent);
  white-space: nowrap;
}

.toolbar {
  flex-shrink: 0;
}
.search-input {
  width: 100%;
  max-width: 420px;
}
.search-icon {
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

/* NDataTable with flex-height needs a positioned, bounded parent */
.data-table {
  flex: 1;
  min-height: 0;
  cursor: pointer;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pager {
  flex-shrink: 0;
  display: flex;
  justify-content: center;
  padding-top: 4px;
}
</style>
