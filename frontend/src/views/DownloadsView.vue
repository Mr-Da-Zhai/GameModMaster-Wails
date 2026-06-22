<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import {
  NButton,
  NIcon,
  NSpin,
  NEmpty,
} from 'naive-ui'
import {
  PlayOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { useFeedback } from '../composables/useConfirm'
import type { DownloadedTrainer } from '../stores/trainer'

const { confirm, toast } = useFeedback()
const trainers = ref<DownloadedTrainer[]>([])
const loading = ref(false)

onMounted(() => loadDownloaded())

async function loadDownloaded() {
  loading.value = true
  try {
    const result = await AppService.GetDownloadedTrainers()
    const arr = result as unknown
    trainers.value = Array.isArray(arr) ? (arr as DownloadedTrainer[]) : []
  } catch (e: any) {
    console.error('[loadDownloaded]', e)
    toast.error(`加载列表失败：${e?.message || e}`)
  } finally {
    loading.value = false
  }
}

async function handleLaunch(id: number, name?: string) {
  try {
    await AppService.LaunchTrainer(id)
    toast.success(`已启动：${name || ''}`)
  } catch (e: any) {
    console.error('[launch]', e)
    toast.error(`启动失败：${e?.message || e}`)
  }
}

async function handleDelete(t: DownloadedTrainer) {
  const ok = await confirm({
    title: '删除修改器',
    content: `将删除「${t.game_name || t.game_name_en}」的本地文件与状态。此操作不可恢复，是否继续？`,
    type: 'error',
    positiveText: '删除',
  })
  if (!ok) return
  try {
    await AppService.DeleteTrainer(t.id)
    await loadDownloaded()
    toast.success('已删除')
  } catch (e: any) {
    console.error('[delete]', e)
    toast.error(`删除失败：${e?.message || e}`)
  }
}

// Clicking a card launches the trainer directly (this page IS the
// "installed trainers" view — the card click shouldn't navigate away to
// detail). The launch + delete buttons in the overlay stop propagation so
// they don't double-fire.
function openDetail(t: DownloadedTrainer) {
  handleLaunch(t.id, t.game_name)
}

const isEmpty = computed(() => !loading.value && trainers.value.length === 0)
</script>

<template>
  <div class="downloads">
    <header class="head">
      <div class="head-left">
        <h1 class="title">我的修改器</h1>
        <span class="count">{{ trainers.length }} 个</span>
      </div>
    </header>

    <div class="grid-scroll">
      <NSpin :show="loading">
        <NEmpty v-if="isEmpty" description="还没有下载任何修改器" class="empty" />

        <div v-else class="grid">
          <article
            v-for="t in trainers"
            :key="t.id"
            class="card"
            @click="openDetail(t)"
          >
            <div class="cover">
              <img
                v-if="t.cover_url"
                :src="t.cover_url"
                :alt="t.game_name"
                loading="lazy"
                @error="(e) => (e.target as HTMLImageElement).style.display = 'none'"
              />
              <div v-if="!t.cover_url" class="cover-fallback">
                <span>{{ (t.game_name || '?').slice(0, 2) }}</span>
              </div>
              <span :class="['status-badge', t.status === 2 ? 'badge-installed' : 'badge-downloaded']">
                {{ t.status === 2 ? '已安装' : '已下载' }}
              </span>
              <div class="overlay">
                <button class="action-btn" @click.stop="handleLaunch(t.id, t.game_name)">
                  <NIcon size="16"><PlayOutline /></NIcon><span>启动</span>
                </button>
                <button class="action-btn danger" @click.stop="handleDelete(t)">
                  <NIcon size="16"><TrashOutline /></NIcon><span>删除</span>
                </button>
              </div>
            </div>
            <div class="info">
              <div class="name" :title="t.game_name">{{ t.game_name || t.game_name_en }}</div>
              <div class="meta">
                <span v-if="t.version">{{ t.version }}</span>
                <span v-if="t.game_version" class="ver">{{ t.game_version }}</span>
              </div>
            </div>
          </article>
        </div>
      </NSpin>
    </div>
  </div>
</template>

<script lang="ts">
export default { name: 'DownloadsView' }
</script>

<style scoped>
.downloads {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  gap: 16px;
}
.head {
  display: flex;
  align-items: baseline;
  gap: 12px;
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

.grid-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 4px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(168px, 1fr));
  gap: 18px;
  padding-bottom: 8px;
}

.card {
  cursor: pointer;
  background: transparent;
  border: none;
  transition: transform 0.2s ease;
}
.card:hover {
  transform: translateY(-2px);
}
.cover {
  position: relative;
  width: 100%;
  aspect-ratio: 3 / 4;
  background: var(--surface-2);
  overflow: hidden;
  border-radius: 16px;
  transition: box-shadow 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
}
.card:hover .cover {
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.5);
}
.cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
  transition: transform 0.3s ease;
}
.card:hover .cover img {
  transform: scale(1.04);
}
.cover-fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  font-weight: 600;
  color: var(--text-3);
  background: linear-gradient(135deg, var(--surface-2), var(--surface-3));
}
/* Status dot top-right — same minimal treatment as HomeView cards. */
.status-badge {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  padding: 0;
  font-size: 0;
  box-shadow: 0 0 0 2px rgba(8, 8, 10, 0.6), 0 0 8px currentColor;
}
.badge-downloaded {
  background: #38bdf8;
  color: #38bdf8;
}
.badge-installed {
  background: #34d399;
  color: #34d399;
}
.overlay {
  position: absolute;
  inset: 0;
  background: linear-gradient(0deg, rgba(8, 8, 10, 0.92) 0%, rgba(8, 8, 10, 0.3) 50%, transparent 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-bottom: 16px;
  opacity: 0;
  transition: opacity 0.18s ease;
  border-radius: 16px;
}
.card:hover .overlay {
  opacity: 1;
}
.action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  width: 78%;
  padding: 8px 16px;
  border: none;
  border-radius: 999px; /* full pill shape */
  background: var(--accent);
  color: #04201c;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}
.action-btn:hover {
  background: #5eead4;
  transform: translateY(-1px);
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.4);
}
.action-btn.danger {
  background: rgba(248, 113, 113, 0.95);
  color: #2a0a0a;
}
.action-btn.danger:hover {
  background: #fca5a5;
}

.info {
  padding: 10px 12px 12px;
}
.name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  margin-top: 4px;
  font-size: 11.5px;
  color: var(--text-3);
  display: flex;
  gap: 8px;
}
.ver {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.empty {
  padding: 80px 0;
}
</style>
