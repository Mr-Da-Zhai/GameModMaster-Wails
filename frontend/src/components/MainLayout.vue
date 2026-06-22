<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

// The top-bar tabs. The detail route collapses onto the Home tab (you reach
// detail from a card click inside the library, so "Library" stays active).
type Tab = { key: string; label: string }

const tabs = computed<Tab[]>(() => [
  { key: '/', label: t('nav.home') },
  { key: '/downloads', label: t('nav.downloads') },
  { key: '/settings', label: t('nav.settings') },
])

const activeKey = computed(() => {
  if (route.path.startsWith('/downloads')) return '/downloads'
  if (route.path.startsWith('/settings')) return '/settings'
  return '/' // home + detail both highlight Library
})

function go(key: string) {
  if (key !== route.path) router.push(key)
}
</script>

<template>
  <div class="app-shell">
    <!-- Top bar: brand on the left, pill tabs on the right. Frosted-glass
         effect via backdrop-filter so content scrolling under it stays
         legible without a hard divider line. -->
    <header class="topbar">
      <div class="brand" @click="go('/')">
        <div class="brand-logo">
          <span class="brand-emoji">🎮</span>
        </div>
        <div class="brand-text">
          <div class="brand-name">ModMaster</div>
        </div>
      </div>

      <nav class="tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          :class="['tab', { active: activeKey === tab.key }]"
          @click="go(tab.key)"
        >
          {{ tab.label }}
        </button>
      </nav>
    </header>

    <!-- Content area: full-width, scrollable. The 24px horizontal padding
         gives the content room to breathe (Apple-style generous margins). -->
    <main class="app-main">
      <div class="main-inner">
        <router-view />
      </div>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

/* Frosted-glass top bar. backdrop-filter blurs whatever scrolls underneath;
   the semi-transparent dark bg keeps contrast. A single hairline border at
   the bottom replaces the old sider's heavy right border. */
.topbar {
  height: 56px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: rgba(10, 10, 12, 0.72);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border-bottom: 1px solid var(--border-soft);
  position: relative;
  z-index: 10;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  user-select: none;
}
.brand-logo {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--accent) 0%, var(--accent-2) 100%);
  box-shadow: 0 4px 14px var(--accent-glow);
  flex-shrink: 0;
}
.brand-emoji {
  font-size: 16px;
}
.brand-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-1);
  letter-spacing: 0.2px;
}

/* Pill tabs: inactive = ghost text; active = soft accent-tinted background.
   The whole group sits in its own rounded container so the active state
   reads as a segmented control rather than three loose buttons. */
.tabs {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  background: var(--surface-1);
  border-radius: 999px;
}
.tab {
  appearance: none;
  border: none;
  background: transparent;
  color: var(--text-3);
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  padding: 7px 18px;
  border-radius: 999px;
  cursor: pointer;
  transition: color 0.15s ease, background 0.15s ease;
}
.tab:hover {
  color: var(--text-1);
}
.tab.active {
  background: var(--accent-glow);
  color: var(--accent);
}

.app-main {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: transparent;
}
.main-inner {
  height: 100%;
  min-height: 0;
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}
</style>
