<script setup lang="ts">
import { onMounted, ref, computed, watch, h } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NIcon,
  NPagination,
  NSpin,
  NEmpty,
  NProgress,
  NAutoComplete,
} from 'naive-ui'
import type { AutoCompleteOption } from 'naive-ui'
import {
  RefreshOutline,
  CloudDownloadOutline,
  SearchOutline,
  DownloadOutline,
  PlayOutline,
  CheckmarkCircle,
  CloseCircleOutline,
  GlobeOutline,
} from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'
import { useTrainerStore, type GameEntry, type Suggestion } from '../stores/trainer'
import { useFeedback } from '../composables/useConfirm'

const { t } = useI18n()
const router = useRouter()
const store = useTrainerStore()
const { toast } = useFeedback()

const searchValue = ref('')
let suggestTimer: ReturnType<typeof setTimeout> | null = null

// Sentinel value the dropdown emits when the user picks the
// "🔍 联网搜索更多..." row. Distinct from any real game id.
const REMOTE_MARKER = '__remote__'

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

// As the user types, fetch local suggestions (instant) to populate the
// autocomplete dropdown. Debounced 150ms; only fires for ≥2 chars.
function handleSearchInput(value: string) {
  searchValue.value = value
  if (suggestTimer) clearTimeout(suggestTimer)
  if (!value.trim() || value.trim().length < 2) {
    store.clearSuggestions()
    return
  }
  suggestTimer = setTimeout(() => {
    store.loadSuggestions(value, 10)
  }, 150)
}

// Build NAutoComplete options: real suggestions first, then a trailing
// "联网搜索更多" row that triggers an explicit remote query.
// The option's label is a plain string (Naive UI 1.x AutoCompleteOption
// requires string); we carry the full suggestion in a side map keyed by
// the option value so the render-label slot and select handler can reach it.
const suggestionsByValue = ref<Record<string, Suggestion>>({})

const searchOptions = computed<AutoCompleteOption[]>(() => {
  const map: Record<string, Suggestion> = {}
  const opts: AutoCompleteOption[] = store.suggestions.map((s) => {
    const v = String(s.id)
    map[v] = s
    return { label: s.display_name || s.name_en, value: v }
  })
  // Always offer the remote escape hatch when there's a query, even if
  // we have local hits (the user may be looking for something newer than
  // the local library knows about).
  if (searchValue.value.trim().length >= 2) {
    opts.push({ label: REMOTE_MARKER, value: REMOTE_MARKER })
  }
  suggestionsByValue.value = map
  return opts
})

// Custom render for an option's label (cover thumb + names + opts count).
// Receives the option value; we look up the suggestion from the side map.
function renderLabel(option: AutoCompleteOption) {
  const value = String(option.value)
  if (value === REMOTE_MARKER) {
    return h(
      'div',
      { class: 'sug-remote' },
      [
        h(NIcon, { size: 14, class: 'sug-remote-ic' }, { default: () => h(GlobeOutline) }),
        h('span', t('home.searchRemote', { query: searchValue.value.trim() })),
      ],
    )
  }
  const s = suggestionsByValue.value[value]
  if (!s) return String(option.label)
  return h('div', { class: 'sug-row' }, [
    h('img', {
      src: s.cover_url || '',
      class: 'sug-thumb',
      loading: 'lazy',
      onError: (e: Event) => ((e.target as HTMLImageElement).style.visibility = 'hidden'),
    }),
    h('div', { class: 'sug-text' }, [
      h('div', { class: 'sug-name' }, s.display_name || s.name_en),
      s.name_local && s.name_local !== s.display_name
        ? h('div', { class: 'sug-sub' }, s.name_en)
        : null,
    ]),
    s.options_num ? h('div', { class: 'sug-opts' }, `${s.options_num}项`) : null,
  ])
}

// User selected a dropdown row (via click or Enter after keyboard nav).
// If it's a real game → jump straight to detail. If it's the remote
// marker → fire the explicit remote search.
function handleSearchSelect(value: string) {
  if (suggestTimer) clearTimeout(suggestTimer)
  if (value === REMOTE_MARKER) {
    const q = searchValue.value.trim()
    if (!q) return
    store.searchRemote(q).then(() => {
      toast.success(t('home.remoteDone', { count: store.trainers.length }))
    })
    return
  }
  // Real game selected — jump to detail page directly.
  const id = Number(value)
  if (id > 0) {
    router.push({ name: 'detail', params: { id } })
    // Clear the dropdown so it doesn't reappear on return.
    store.clearSuggestions()
  }
}

// Enter pressed without selecting a dropdown row → run a local full
// search (not remote; the user can pick the remote row explicitly).
function handleSearchEnter() {
  if (suggestTimer) clearTimeout(suggestTimer)
  const q = searchValue.value.trim()
  if (q) store.searchTrainers(q)
}

function handleRefresh() {
  store.refreshData()
}

async function handleCancelRefresh() {
  try {
    await store.cancelRefresh()
    toast.info(t('home.refreshCancelled'))
  } catch (e: any) {
    toast.error(e?.message || String(e))
  }
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
      return { text: t('detail.status.downloaded'), cls: 'badge-downloaded' }
    case 2:
      return { text: t('detail.status.installed'), cls: 'badge-installed' }
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
  const pct = p.total ? Math.round((p.current / p.total) * 100) : 0
  const errs = p.detail_errors ? ` · ${p.detail_errors} ${t('home.failed')}` : ''
  return `${t('home.crawling')} ${p.current}/${p.total || 3} (${pct}%) · ${p.games || 0} ${t('home.games')}${errs}`
})

const refreshPercent = computed(() => {
  const p = store.refreshProgress
  if (!p || !p.total || !p.current) return 0
  return Math.min(100, Math.round((p.current / p.total) * 100))
})

const isEmpty = computed(
  () => !store.loading && !store.refreshing && !store.remoteLoading && store.trainers.length === 0
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
        <NAutoComplete
          :value="searchValue"
          :options="searchOptions"
          :render-label="renderLabel"
          :placeholder="t('home.searchPlaceholder')"
          :loading="store.suggestionsLoading"
          clearable
          class="search"
          @update:value="handleSearchInput"
          @select="handleSearchSelect"
          @keyup.enter="handleSearchEnter"
        >
          <template #prefix>
            <NIcon :component="SearchOutline" class="search-ic" />
          </template>
        </NAutoComplete>
        <NIcon
          v-if="store.remoteLoading"
          size="14"
          class="remote-spinner"
        >
          <RefreshOutline />
        </NIcon>
      </div>
      <div class="head-right">
        <NButton
          v-if="store.refreshing"
          quaternary
          circle
          type="error"
          @click="handleCancelRefresh"
          title="取消刷新（进度已保存）"
        >
          <template #icon><NIcon><CloseCircleOutline /></NIcon></template>
        </NButton>
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

    <!-- Slim progress bar while crawling -->
    <div v-if="store.refreshing && store.refreshProgress?.total" class="progress-row">
      <NProgress
        type="line"
        :percentage="refreshPercent"
        :height="4"
        :show-indicator="false"
        :border-radius="2"
        color="var(--accent)"
        rail-color="var(--surface-2)"
      />
    </div>

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

/* Remote-search spinner shown next to the box while a remote query is in flight */
.remote-spinner {
  color: var(--accent);
  animation: spin 0.9s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Autocomplete suggestion rows — these live in a teleport'd dropdown so the
   styles must be global (non-scoped). The :deep() wrapper targets them from
   inside the scoped style block. */
:deep(.sug-row) {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 4px;
}
:deep(.sug-thumb) {
  width: 30px;
  height: 40px;
  border-radius: 4px;
  object-fit: cover;
  flex-shrink: 0;
  background: var(--surface-2);
}
:deep(.sug-text) {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
:deep(.sug-name) {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.sug-sub) {
  font-size: 11px;
  color: var(--text-3);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
:deep(.sug-opts) {
  font-size: 11px;
  color: var(--accent);
  background: var(--accent-glow);
  padding: 2px 7px;
  border-radius: 10px;
  flex-shrink: 0;
}
:deep(.sug-remote) {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 4px;
  font-size: 13px;
  color: var(--accent);
  border-top: 1px dashed var(--border);
}
:deep(.sug-remote-ic) {
  color: var(--accent);
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

.progress-row {
  flex-shrink: 0;
  padding: 0 2px;
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
