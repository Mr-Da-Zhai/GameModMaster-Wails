<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { RecycleScroller } from 'vue-virtual-scroller'
import 'vue-virtual-scroller/dist/vue-virtual-scroller.css'

interface Props {
  items: any[]
  itemWidth?: number
  itemHeight?: number
  gap?: number
  buffer?: number
}

const props = withDefaults(defineProps<Props>(), {
  itemWidth: 280,
  itemHeight: 320,
  gap: 20,
  buffer: 200,
})

const scrollerRef = ref<InstanceType<typeof RecycleScroller> | null>(null)
const containerWidth = ref(0)
const resizeObserver = ref<ResizeObserver | null>(null)

// Calculate number of columns based on container width
const columns = computed(() => {
  const availableWidth = containerWidth.value
  const itemWidthWithGap = props.itemWidth + props.gap
  const minColumns = 1
  const calculatedColumns = Math.floor((availableWidth + props.gap) / itemWidthWithGap)
  return Math.max(minColumns, calculatedColumns)
})

// Calculate actual item width to fill container
const actualItemWidth = computed(() => {
  if (columns.value === 1) {
    return props.itemWidth
  }
  const totalGapWidth = (columns.value - 1) * props.gap
  const availableWidth = containerWidth.value - totalGapWidth
  return Math.floor(availableWidth / columns.value)
})

// Group items into rows
const groupedItems = computed(() => {
  const groups: any[][] = []
  for (let i = 0; i < props.items.length; i += columns.value) {
    groups.push(props.items.slice(i, i + columns.value))
  }
  return groups
})

// Row height (item height + gap)
const rowHeight = computed(() => props.itemHeight + props.gap)

// Handle resize
const handleResize = (entries: ResizeObserverEntry[]) => {
  for (const entry of entries) {
    containerWidth.value = entry.contentRect.width
  }
}

onMounted(() => {
  if (scrollerRef.value?.$el) {
    resizeObserver.value = new ResizeObserver(handleResize)
    resizeObserver.value.observe(scrollerRef.value.$el.parentElement || scrollerRef.value.$el)
    // Set initial width
    containerWidth.value = scrollerRef.value.$el.parentElement?.clientWidth || 0
  }
})

onUnmounted(() => {
  if (resizeObserver.value) {
    resizeObserver.value.disconnect()
  }
})
</script>

<template>
  <RecycleScroller
    ref="scrollerRef"
    class="virtual-grid-scroller"
    :items="groupedItems"
    :item-size="rowHeight"
    :buffer="buffer"
    key-field="0.id"
    v-slot="{ item: row }"
  >
    <div class="virtual-grid-row" :style="{ gap: `${gap}px` }">
      <div
        v-for="item in row"
        :key="item.id"
        class="virtual-grid-item"
        :style="{ width: `${actualItemWidth}px`, height: `${itemHeight}px` }"
      >
        <slot :item="item" />
      </div>
    </div>
  </RecycleScroller>
</template>

<style scoped>
.virtual-grid-scroller {
  width: 100%;
  height: 100%;
}

.virtual-grid-row {
  display: flex;
  justify-content: flex-start;
  padding: 0;
}

.virtual-grid-item {
  flex-shrink: 0;
}
</style>
