<script setup lang="ts">
import { computed, h } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { NLayout, NLayoutSider, NMenu, NIcon } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import {
  HomeOutline,
  DownloadOutline,
  SettingsOutline,
} from '@vicons/ionicons5'

const route = useRoute()
const router = useRouter()

const activeKey = computed(() => {
  if (route.path.startsWith('/downloads')) return '/downloads'
  if (route.path.startsWith('/settings')) return '/settings'
  return '/'
})

function handleMenuUpdate(key: string) {
  router.push(key)
}

function renderIcon(icon: any) {
  return () => h(NIcon, { size: 19 }, { default: () => h(icon) })
}

const menuOptions: MenuOption[] = [
  {
    label: '游戏列表',
    key: '/',
    icon: renderIcon(HomeOutline),
  },
  {
    label: '我的修改器',
    key: '/downloads',
    icon: renderIcon(DownloadOutline),
  },
  {
    label: '设置',
    key: '/settings',
    icon: renderIcon(SettingsOutline),
  },
]
</script>

<template>
  <NLayout has-sider class="app-shell">
    <NLayoutSider
      bordered
      :width="208"
      :native-scrollbar="false"
      content-style="display:flex; flex-direction:column; height:100%;"
      class="app-sider"
    >
      <!-- Brand -->
      <div class="brand">
        <div class="brand-logo">
          <span class="brand-emoji">🎮</span>
        </div>
        <div class="brand-text">
          <div class="brand-name">ModMaster</div>
          <div class="brand-sub">游戏修改器大师</div>
        </div>
      </div>

      <!-- Nav -->
      <div class="nav-wrap">
        <NMenu
          :value="activeKey"
          :options="menuOptions"
          :inverted="true"
          @update:value="handleMenuUpdate"
        />
      </div>

      <!-- Footer -->
      <div class="sider-footer">
        <div class="footer-dot"></div>
        <span>数据源：FLiNG Trainer</span>
      </div>
    </NLayoutSider>

    <NLayout class="app-main" :native-scrollbar="false">
      <div class="main-inner">
        <router-view />
      </div>
    </NLayout>
  </NLayout>
</template>

<style scoped>
.app-shell {
  height: 100vh;
}

.app-sider {
  background: linear-gradient(180deg, #131c2e 0%, #0f1729 100%) !important;
  border-right: 1px solid var(--border) !important;
}

/* Brand area */
.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 22px 18px 20px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.08);
}
.brand-logo {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, var(--accent) 0%, #0ea5e9 100%);
  box-shadow: 0 6px 18px var(--accent-glow);
  flex-shrink: 0;
}
.brand-emoji {
  font-size: 20px;
}
.brand-text {
  min-width: 0;
}
.brand-name {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-1);
  letter-spacing: 0.3px;
  line-height: 1.2;
}
.brand-sub {
  font-size: 11px;
  color: var(--text-3);
  margin-top: 2px;
}

.nav-wrap {
  flex: 1;
  padding: 14px 12px;
  overflow-y: auto;
}

.sider-footer {
  padding: 14px 18px 18px;
  border-top: 1px solid rgba(148, 163, 184, 0.08);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--text-3);
}
.footer-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent);
  box-shadow: 0 0 8px var(--accent);
  flex-shrink: 0;
}

.app-main {
  background: transparent !important;
}

.main-inner {
  height: 100%;
  padding: 24px 28px;
  display: flex;
  flex-direction: column;
  min-height: 0; /* allow children to shrink + scroll */
}
</style>
