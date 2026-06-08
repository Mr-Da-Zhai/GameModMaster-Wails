<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NButton,
  NIcon,
  NSpin,
} from 'naive-ui'
import { RefreshOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'

const loading = ref(false)
const dataDir = ref('')
const mappingCount = ref(0)
const settings = ref<Record<string, any>>({})

onMounted(() => {
  loadSettings()
})

async function loadSettings() {
  loading.value = true
  try {
    const [dir, s] = await Promise.all([
      AppService.GetDataDir(),
      AppService.GetSettings(),
    ])
    dataDir.value = dir || ''
    settings.value = s || {}
    // mappingCount comes from settings if the backend stores it
    mappingCount.value = s?.mapping_count || 0
  } catch (e) {
    console.error('Failed to load settings:', e)
  } finally {
    loading.value = false
  }
}

async function handleRefresh() {
  await loadSettings()
}
</script>

<template>
  <div class="settings-view">
    <div class="page-header">
      <h2 class="page-title">设置</h2>
    </div>

    <NSpin :show="loading">
      <!-- Data section -->
      <NCard title="数据" size="small" style="margin-bottom: 16px;">
        <NDescriptions label-placement="left" :column="1" bordered size="small">
          <NDescriptionsItem label="数据目录">
            <span style="font-family: monospace; font-size: 13px;">{{ dataDir || '-' }}</span>
          </NDescriptionsItem>
          <NDescriptionsItem label="映射表">
            {{ mappingCount > 0 ? `已加载 ${mappingCount} 条名称映射` : '未加载' }}
          </NDescriptionsItem>
        </NDescriptions>
      </NCard>

      <!-- About section -->
      <NCard title="关于" size="small">
        <NDescriptions label-placement="left" :column="1" bordered size="small">
          <NDescriptionsItem label="应用名称">GameModMaster</NDescriptionsItem>
          <NDescriptionsItem label="版本">1.0.0</NDescriptionsItem>
          <NDescriptionsItem label="数据来源">
            <a href="https://flingtrainer.com" target="_blank" style="color: #63e2b7;">FLiNG Trainer</a>
          </NDescriptionsItem>
          <NDescriptionsItem label="技术栈">Wails v3 + Go + Vue 3 + Naive UI</NDescriptionsItem>
        </NDescriptions>
      </NCard>
    </NSpin>
  </div>
</template>

<script lang="ts">
export default {
  name: 'SettingsView',
}
</script>

<style scoped>
.settings-view {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.page-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
}
</style>
