<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { NModal, NCard, NInput, NIcon, NPagination, NSpin, NEmpty, NTag } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import * as AppService from '../../bindings/GameModMaster/appservice'
import { useFeedback } from '../composables/useConfirm'

interface MappingEntry {
  name_en: string
  name_zh: string
  aliases: string[]
}

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'update:show', v: boolean): void }>()

const { toast } = useFeedback()

const entries = ref<MappingEntry[]>([])
const loading = ref(false)
const query = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
let searchTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  if (props.show) load()
})

watch(
  () => props.show,
  (v) => {
    if (v) load()
  },
)

watch(query, () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    load()
  }, 300)
})

async function load() {
  loading.value = true
  try {
    const result = (await AppService.GetMappingEntries(query.value.trim(), (page.value - 1) * pageSize.value, pageSize.value)) as unknown
    entries.value = Array.isArray(result) ? (result as MappingEntry[]) : []
    // The binding doesn't return a total; derive it from a count call when no
    // query is set, otherwise show "at least this many" if we got a full page.
    if (query.value.trim() === '') {
      total.value = (await AppService.GetMappingCount()) || 0
    } else if (entries.value.length === pageSize.value) {
      // More results likely exist; show "..." by leaving the total one past.
      total.value = page.value * pageSize.value + 1
    } else {
      total.value = (page.value - 1) * pageSize.value + entries.value.length
    }
  } catch (e: any) {
    console.error('[MappingBrowser load]', e)
    toast.error(`${e?.message || e}`)
  } finally {
    loading.value = false
  }
}

function onPageChange(p: number) {
  page.value = p
  load()
}

function close() {
  emit('update:show', false)
}
</script>

<template>
  <NModal
    :show="show"
    @update:show="emit('update:show', $event)"
    preset="card"
    :title="`名称映射表 (${total})`"
    style="width: 760px; max-width: 92vw;"
    :bordered="false"
    size="huge"
    :mask-closable="true"
  >
    <div class="map-toolbar">
      <NInput
        v-model:value="query"
        placeholder="搜索英文名 / 中文名 / 别名…"
        clearable
        size="small"
      >
        <template #prefix>
          <NIcon :component="SearchOutline" />
        </template>
      </NInput>
    </div>

    <NSpin :show="loading" style="min-height: 280px;">
      <NEmpty v-if="!loading && entries.length === 0" description="无匹配条目" style="padding: 40px 0;" />
      <div v-else class="map-list">
        <div v-for="(e, i) in entries" :key="i" class="map-row">
          <div class="row-en">{{ e.name_en }}</div>
          <div class="row-zh">{{ e.name_zh }}</div>
          <div v-if="e.aliases && e.aliases.length" class="row-aliases">
            <NTag v-for="a in e.aliases" :key="a" size="tiny" round :bordered="false" type="info">
              {{ a }}
            </NTag>
          </div>
        </div>
      </div>
    </NSpin>

    <template #footer>
      <div class="map-footer">
        <NPagination
          :page="page"
          :item-count="total"
          :page-size="pageSize"
          :page-slot="7"
          size="small"
          @update:page="onPageChange"
        />
      </div>
    </template>
  </NModal>
</template>

<script lang="ts">
export default { name: 'MappingBrowser' }
</script>

<style scoped>
.map-toolbar {
  margin-bottom: 12px;
}
.map-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 56vh;
  overflow-y: auto;
}
.map-row {
  padding: 10px 12px;
  background: var(--surface-2);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.row-en {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-1);
}
.row-zh {
  font-size: 13px;
  color: var(--accent);
}
.row-aliases {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 2px;
}
.map-footer {
  display: flex;
  justify-content: center;
}
</style>
