<script setup lang="ts">
import { onMounted, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
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

const router = useRouter()
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

function openDetail(t: DownloadedTrainer) {
  router.push({ name: 'detail', params: { id: t.game_id } })
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
                <button v-if="t.status === 2" class="action-btn" @click.stop="handleLaunch(t.id, t.game_name)">
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
                <span>{{ t.version || '-' }}</span>
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
  gap: 10px;
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
.action-btn.danger {
  background: rgba(248, 113, 113, 0.9);
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
}
.empty {
  padding: 80px 0;
}
</style>
