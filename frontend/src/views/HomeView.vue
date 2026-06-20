<script setup lang="ts">
import { onMounted, ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NInput,
  NButton,
  NIcon,
  NPagination,
  NSpin,
  NEmpty,
} from 'naive-ui'
import {
  RefreshOutline,
  CloudDownloadOutline,
  SearchOutline,
  DownloadOutline,
  PlayOutline,
  CheckmarkCircle,
} from '@vicons/ionicons5'
import { useTrainerStore, type GameEntry } from '../stores/trainer'

const router = useRouter()
const store = useTrainerStore()

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  store.bindEvents()
  store.loadTrainers(1)
})

watch(
  () => store.refreshProgress.done,
  (done) => {
    if (done) store.onRefreshComplete()
  }
)

function handleSearch(query: string) {
  searchValue.value = query
  if (searchTimer) clearTimeout(searchTimer)
  // Remote search hits the network, so debounce a bit longer than a local
  // lookup would, and clear immediately when the input is emptied.
  if (!query.trim()) {
    store.searchTrainers('')
    return
  }
  searchTimer = setTimeout(() => store.searchTrainers(query), 500)
}

function handleSearchEnter() {
  // Enter commits the search immediately instead of waiting for debounce.
  if (searchTimer) clearTimeout(searchTimer)
  if (searchValue.value.trim()) store.searchTrainers(searchValue.value)
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

function openDetail(row: GameEntry) {
  router.push({ name: 'detail', params: { id: row.id } })
}

function statusBadge(status: number) {
  switch (status) {
    case 1:
      return { text: '已下载', cls: 'badge-downloaded' }
    case 2:
      return { text: '已安装', cls: 'badge-installed' }
    default:
      return null
  }
}

function formatDate(ts: number) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const refreshStatus = computed(() => {
  const p = store.refreshProgress
  if (!p || !p.current) return ''
  return `抓取中 ${p.current}/${p.total || 3} · ${p.games || 0} 游戏`
})

const isEmpty = computed(
  () => !store.loading && !store.refreshing && store.trainers.length === 0
)

function onCardAction(e: Event, row: GameEntry) {
  e.stopPropagation()
  const t = row.latest_trainer
  if (!t) return
  if (row.status === 2) store.launchTrainer(t.id)
  else if (row.status === 1) store.installTrainer(t.id)
  else store.downloadTrainer(t.id)
}

function actionLabel(status: number) {
  if (status === 2) return '启动'
  if (status === 1) return '安装'
  return '下载'
}
</script>

<template>
  <div class="home">
    <!-- Header bar -->
    <header class="head">
      <div class="head-left">
        <h1 class="title">游戏库</h1>
        <span class="count">{{ store.totalCount }} 款</span>
        <span v-if="store.refreshing && refreshStatus" class="refresh-tag">
          <span class="dot pulse"></span>{{ refreshStatus }}
        </span>
      </div>
      <div class="head-center">
        <NInput
          :value="searchValue"
          placeholder="搜索游戏（中英文均可，联网搜索全部游戏）…"
          clearable
          class="search"
          @update:value="handleSearch"
          @keyup.enter="handleSearchEnter"
        >
          <template #prefix>
            <NIcon :component="SearchOutline" class="search-ic" />
          </template>
        </NInput>
      </div>
      <div class="head-right">
        <NButton
          :loading="store.refreshing"
          quaternary
          circle
          @click="handleRefresh"
          title="刷新数据"
        >
          <template #icon><NIcon><RefreshOutline /></NIcon></template>
        </NButton>
      </div>
    </header>

    <!-- Error banner (never silently swallow failures) -->
    <div v-if="store.lastError" class="error-bar">
      <strong>⚠</strong> {{ store.lastError }}
    </div>

    <!-- Card grid: CSS Grid auto-fills columns based on width.
         Outer container scrolls; cards never collapse. -->
    <div class="grid-scroll">
      <NSpin :show="store.loading">
        <NEmpty
          v-if="isEmpty"
          description="暂无游戏数据"
          class="empty"
        >
          <template #extra>
            <NButton type="primary" :loading="store.refreshing" @click="handleLoadData">
              <template #icon><NIcon><CloudDownloadOutline /></NIcon></template>
              立即加载
            </NButton>
          </template>
        </NEmpty>

        <div v-else class="grid">
          <article
            v-for="g in store.trainers"
            :key="g.id"
            class="card"
            @click="openDetail(g)"
          >
            <div class="cover">
              <img
                v-if="g.cover_url"
                :src="g.cover_url"
                :alt="g.display_name"
                loading="lazy"
                @error="(e) => (e.target as HTMLImageElement).style.display = 'none'"
              />
              <div v-if="!g.cover_url" class="cover-fallback">
                <span>{{ (g.display_name || g.name_en || '?').slice(0, 2) }}</span>
              </div>

              <!-- status badge -->
              <span v-if="statusBadge(g.status)" :class="['status-badge', statusBadge(g.status)!.cls]">
                {{ statusBadge(g.status)!.text }}
              </span>

              <!-- hover action overlay -->
              <div class="overlay">
                <button class="action-btn" @click="onCardAction($event, g)">
                  <NIcon size="16">
                    <PlayOutline v-if="g.status === 2" />
                    <CheckmarkCircle v-else-if="g.status === 1" />
                    <DownloadOutline v-else />
                  </NIcon>
                  <span>{{ actionLabel(g.status) }}</span>
                </button>
              </div>
            </div>

            <div class="info">
              <div class="name" :title="g.display_name || g.name_en">
                {{ g.display_name || g.name_en || 'Unknown' }}
              </div>
              <div class="meta">
                <span v-if="g.options_num">{{ g.options_num }} 项</span>
                <span v-if="g.latest_trainer?.game_version" class="ver">{{ g.latest_trainer.game_version }}</span>
              </div>
            </div>
          </article>
        </div>
      </NSpin>
    </div>

    <!-- Pagination -->
    <footer v-if="store.totalCount > 0 && !isEmpty" class="pager">
      <NPagination
        :page="store.currentPage"
        :item-count="store.totalCount"
        :page-size="store.pageSize"
        :page-slot="7"
        show-quick-jumper
        @update:page="handlePageChange"
      />
    </footer>
  </div>
</template>

<script lang="ts">
export default { name: 'HomeView' }
</script>

<style scoped>
.home {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
}

.head {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  gap: 16px;
}
.head-left {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-shrink: 0;
}
.head-center {
  flex: 1;
  display: flex;
  justify-content: center;
}
.head-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-1);
}
.count {
  font-size: 13px;
  color: var(--text-3);
}
.refresh-tag {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--accent);
}
.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent);
}
.pulse {
  animation: pulse 1s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.3; }
}

.head-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.search {
  width: 100%;
  max-width: 480px;
}
.search-ic {
  color: var(--text-3);
}

.error-bar {
  flex-shrink: 0;
  background: rgba(248, 113, 113, 0.12);
  border: 1px solid rgba(248, 113, 113, 0.4);
  color: #fca5a5;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
}

/* Scrollable grid container — the real fix for layout collapse */
.grid-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}

/* Auto-filling responsive grid: 5 cols on wide, fewer on narrow */
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 18px;
  padding-bottom: 8px;
}

.card {
  cursor: pointer;
  border-radius: 12px;
  overflow: hidden;
  background: var(--surface-1);
  border: 1px solid var(--border-soft);
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}
.card:hover {
  transform: translateY(-3px);
  border-color: var(--accent);
  box-shadow: 0 12px 28px rgba(0, 0, 0, 0.4);
}

.cover {
  position: relative;
  width: 100%;
  aspect-ratio: 3 / 4;
  background: var(--surface-2);
  overflow: hidden;
}
.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.cover-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  font-weight: 700;
  color: var(--text-3);
  background: linear-gradient(135deg, var(--surface-2), #1a2740);
}

.status-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 3px 9px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  backdrop-filter: blur(6px);
}
.badge-downloaded {
  background: rgba(56, 189, 248, 0.85);
  color: #0c2433;
}
.badge-installed {
  background: rgba(52, 211, 153, 0.9);
  color: #052e1e;
}

.overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(0deg, rgba(15, 23, 42, 0.92) 0%, rgba(15, 23, 42, 0.3) 50%, transparent 100%);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding-bottom: 14px;
  opacity: 0;
  transition: opacity 0.18s ease;
}
.card:hover .overlay {
  opacity: 1;
}
.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 16px;
  border: none;
  border-radius: 20px;
  background: var(--accent);
  color: #04201c;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, transform 0.15s ease;
}
.action-btn:hover {
  background: #5eead4;
  transform: scale(1.05);
}

.info {
  padding: 10px 12px 12px;
}
.name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-1);
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  display: flex;
  gap: 8px;
  margin-top: 4px;
  font-size: 11.5px;
  color: var(--text-3);
}
.ver {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.empty {
  padding: 80px 0;
}

.pager {
  flex-shrink: 0;
  display: flex;
  justify-content: center;
  padding-top: 4px;
}
</style>
