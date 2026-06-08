<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage, useDialog } from 'naive-ui'
import { DownloadOutline, PlayOutline, TrashOutline, TimeOutline, ChevronDownOutline, ChevronUpOutline } from '@vicons/ionicons5'
import type { Trainer } from '@/types'
import { useTrainerStore } from '@/stores/trainer'
import { useI18n } from 'vue-i18n'
import { getCachedImageUrl } from '@/services/imageCacheService'

const props = defineProps<{
  trainer: Trainer
}>()

const { t, locale } = useI18n()
const router = useRouter()
const store = useTrainerStore()
const message = useMessage()
const dialog = useDialog()

const isDownloaded = computed(() => {
  return store.downloadedIds.has(props.trainer.id)
})

const isExpanded = ref(false)

// Cached image URL state
const cachedImageUrl = ref<string>(props.trainer.thumbnail || '/placeholder.png')

// Load cached image on mount
onMounted(async () => {
  await loadCachedImage()
})

// Watch for trainer changes
watch(() => props.trainer.thumbnail, async () => {
  await loadCachedImage()
})

async function loadCachedImage() {
  if (props.trainer.thumbnail) {
    try {
      cachedImageUrl.value = await getCachedImageUrl(props.trainer.thumbnail)
    } catch (error) {
      console.error('Failed to load cached image:', error)
      cachedImageUrl.value = '/placeholder.png'
    }
  }
}

const formatDate = (dateString: string) => {
  if (!dateString) return ''
  const date = new Date(dateString)
  return new Intl.DateTimeFormat(locale.value, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  }).format(date)
}

const handleImageError = (e: Event) => {
  const target = e.target as HTMLImageElement
  target.src = '/placeholder.png'
}

const handleCardClick = () => {
  router.push(`/detail/${props.trainer.id}`)
}

const handleDownload = async (e: Event) => {
  e.stopPropagation()
  try {
    message.loading(t('common.loading'))
    const detail = await store.getTrainerDetail(props.trainer.id)
    await store.downloadTrainer(detail)
    message.success(t('gameCard.messages.downloadSuccess'))
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('gameCard.messages.downloadFailed'))
  }
}

const handleLaunch = async (e: Event) => {
  e.stopPropagation()
  try {
    await store.launchTrainer(props.trainer.id)
    message.success(t('gameCard.messages.launchSuccess'))
  } catch (error) {
    message.error(error instanceof Error ? error.message : t('gameCard.messages.launchFailed'))
  }
}

const handleDelete = async (e: Event) => {
  e.stopPropagation()
  dialog.warning({
    title: t('gameCard.deleteConfirm.title'),
    content: t('gameCard.deleteConfirm.content', { name: props.trainer.name }),
    positiveText: t('gameCard.deleteConfirm.positive'),
    negativeText: t('gameCard.deleteConfirm.negative'),
    onPositiveClick: async () => {
      try {
        await store.deleteTrainer(props.trainer.id)
        message.success(t('gameCard.messages.deleteSuccess'))
      } catch (error) {
        message.error(t('gameCard.messages.deleteFailed'))
      }
    },
  })
}

const toggleExpand = (e: Event) => {
  e.stopPropagation()
  isExpanded.value = !isExpanded.value
}
</script>

<template>
  <div class="trainer-list-item" @click="handleCardClick">
    <div class="item-main">
      <img
        :src="cachedImageUrl"
        :alt="trainer.name"
        class="item-thumbnail"
        loading="lazy"
        @error="handleImageError"
      />

      <div class="item-info">
        <div class="item-header">
          <h3 class="item-name" :title="trainer.name">{{ trainer.name }}</h3>
          <div class="item-badges">
            <span class="badge version">{{ trainer.version }}</span>
            <span class="badge game-version">{{ trainer.game_version }}</span>
          </div>
        </div>

        <div class="item-meta">
          <div class="meta-row">
            <span class="meta-item">
              <NIcon size="14"><TimeOutline /></NIcon>
              {{ formatDate(trainer.last_update) }}
            </span>
            <span class="meta-item">
              <NIcon size="14"><DownloadOutline /></NIcon>
              {{ trainer.download_count }} {{ t('detail.meta.downloads') }}
            </span>
          </div>
          <div class="meta-row" v-if="isExpanded">
            <p class="item-description">{{ trainer.description || t('detail.description.empty') }}</p>
          </div>
        </div>
      </div>

      <div class="item-actions">
        <button class="action-btn expand-btn" @click="toggleExpand">
          <NIcon size="16">
            <ChevronDownOutline v-if="!isExpanded" />
            <ChevronUpOutline v-else />
          </NIcon>
        </button>

        <template v-if="isDownloaded">
          <button class="action-btn danger" @click="handleDelete">
            <NIcon size="16"><TrashOutline /></NIcon>
          </button>
          <button class="action-btn primary" @click="handleLaunch">
            <NIcon size="16"><PlayOutline /></NIcon>
            <span>{{ t('gameCard.actions.launch') }}</span>
          </button>
        </template>
        <template v-else>
          <button class="action-btn primary" @click="handleDownload">
            <NIcon size="16"><DownloadOutline /></NIcon>
            <span>{{ t('gameCard.actions.download') }}</span>
          </button>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.trainer-list-item {
  background: rgba(255, 255, 255, 0.9);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  overflow: hidden;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.04);
  cursor: pointer;
  transition: all 0.2s ease;
  border: 1px solid rgba(0, 0, 0, 0.04);
}

.trainer-list-item:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.08);
}

.dark .trainer-list-item {
  background: rgba(30, 41, 59, 0.9);
  border-color: rgba(255, 255, 255, 0.06);
}

.dark .trainer-list-item:hover {
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.3);
}

.item-main {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  padding: 16px;
}

.item-thumbnail {
  width: 80px;
  height: 80px;
  object-fit: cover;
  border-radius: 12px;
  flex-shrink: 0;
}

.item-info {
  flex: 1;
  min-width: 0;
}

.item-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.item-name {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: #1f2937;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dark .item-name {
  color: #e2e8f0;
}

.item-badges {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

.badge {
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 0.75rem;
  font-weight: 600;
}

.badge.version {
  background: rgba(124, 58, 237, 0.1);
  color: #7c3aed;
}

.badge.game-version {
  background: rgba(8, 145, 178, 0.1);
  color: #0891b2;
}

.dark .badge.version {
  background: rgba(124, 58, 237, 0.2);
}

.dark .badge.game-version {
  background: rgba(8, 145, 178, 0.2);
}

.item-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.meta-row {
  display: flex;
  align-items: center;
  gap: 16px;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 0.813rem;
  color: #64748b;
}

.dark .meta-item {
  color: #94a3b8;
}

.item-description {
  margin: 0;
  font-size: 0.813rem;
  color: #64748b;
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.dark .item-description {
  color: #94a3b8;
}

.item-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 12px;
  border: none;
  border-radius: 10px;
  font-size: 0.813rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.action-btn.expand-btn {
  width: 32px;
  height: 32px;
  padding: 0;
  background: rgba(0, 0, 0, 0.04);
  color: #64748b;
}

.action-btn.expand-btn:hover {
  background: rgba(0, 0, 0, 0.08);
  color: #1f2937;
}

.dark .action-btn.expand-btn {
  background: rgba(255, 255, 255, 0.06);
  color: #94a3b8;
}

.dark .action-btn.expand-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #e2e8f0;
}

.action-btn.primary {
  background: linear-gradient(135deg, #7c3aed 0%, #6d28d9 100%);
  color: white;
}

.action-btn.primary:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(124, 58, 237, 0.3);
}

.action-btn.danger {
  padding: 8px;
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

.action-btn.danger:hover {
  background: #ef4444;
  color: white;
}
</style>
