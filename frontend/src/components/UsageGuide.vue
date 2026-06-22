<script setup lang="ts">
import { computed } from 'vue'
import { NCollapse, NCollapseItem } from 'naive-ui'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

// Each key maps to { key, label, desc } so the template can render a uniform
// table of shortcuts. Sourced from i18n so it follows the active locale.
const shortcuts = computed(() => [
  {
    key: 'F1',
    label: t('usage.shortcuts.f1.label'),
    desc: t('usage.shortcuts.f1.desc'),
  },
  {
    key: 'Home',
    label: t('usage.shortcuts.home.label'),
    desc: t('usage.shortcuts.home.desc'),
  },
  {
    key: 'Num 0',
    label: t('usage.shortcuts.num0.label'),
    desc: t('usage.shortcuts.num0.desc'),
  },
  {
    key: 'Num 1-8',
    label: t('usage.shortcuts.num1to8.label'),
    desc: t('usage.shortcuts.num1to8.desc'),
  },
  {
    key: 'Num 9',
    label: t('usage.shortcuts.num9.label'),
    desc: t('usage.shortcuts.num9.desc'),
  },
])
</script>

<template>
  <NCollapse accordion>
    <NCollapseItem :title="t('usage.title')" name="usage">
      <div class="usage">
        <p class="intro">{{ t('usage.intro') }}</p>

        <div class="shortcuts">
          <div v-for="s in shortcuts" :key="s.key" class="shortcut-row">
            <div class="shortcut-key">
              <kbd>{{ s.key }}</kbd>
            </div>
            <div class="shortcut-body">
              <div class="shortcut-label">{{ s.label }}</div>
              <div class="shortcut-desc">{{ s.desc }}</div>
            </div>
          </div>
        </div>

        <div class="notes">
          <h4>{{ t('usage.notes.title') }}</h4>
          <ul>
            <li>{{ t('usage.notes.version') }}</li>
            <li>{{ t('usage.notes.order') }}</li>
            <li>{{ t('usage.notes.antivirus') }}</li>
            <li>{{ t('usage.notes.oneday') }}</li>
          </ul>
        </div>
      </div>
    </NCollapseItem>
  </NCollapse>
</template>

<script lang="ts">
export default { name: 'UsageGuide' }
</script>

<style scoped>
.usage {
  font-size: 13.5px;
  color: var(--text-2);
  line-height: 1.6;
}
.intro {
  margin: 0 0 16px;
  color: var(--text-2);
}

.shortcuts {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 18px;
}
.shortcut-row {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 10px 12px;
  background: var(--surface-2);
  border-radius: 8px;
}
.shortcut-key {
  flex-shrink: 0;
  width: 78px;
}
.shortcut-key kbd {
  display: inline-block;
  min-width: 56px;
  padding: 4px 8px;
  background: var(--surface-0);
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  text-align: center;
}
.shortcut-body {
  flex: 1;
  min-width: 0;
}
.shortcut-label {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-1);
}
.shortcut-desc {
  font-size: 12.5px;
  color: var(--text-3);
  margin-top: 2px;
}

.notes {
  padding: 12px 14px;
  background: rgba(251, 191, 36, 0.08);
  border: 1px solid rgba(251, 191, 36, 0.25);
  border-radius: 8px;
}
.notes h4 {
  margin: 0 0 8px;
  font-size: 13px;
  color: #fbbf24;
}
.notes ul {
  margin: 0;
  padding-left: 18px;
}
.notes li {
  font-size: 12.5px;
  color: var(--text-2);
  margin-bottom: 4px;
}
.notes li:last-child {
  margin-bottom: 0;
}
</style>
