<script setup lang="ts">
import { computed } from 'vue'
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
  return () => h(NIcon, { size: 20 }, { default: () => h(icon) })
}

import { h } from 'vue'

const menuOptions: MenuOption[] = [
  {
    label: '首页',
    key: '/',
    icon: renderIcon(HomeOutline),
  },
  {
    label: '已下载',
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
  <NLayout has-sider style="height: 100vh">
    <NLayoutSider
      bordered
      :width="180"
      :native-scrollbar="false"
      content-style="padding-top: 16px;"
      style="background-color: #162130;"
    >
      <div class="logo">
        <span class="logo-text">GameModMaster</span>
      </div>
      <NMenu
        :value="activeKey"
        :options="menuOptions"
        :inverted="true"
        @update:value="handleMenuUpdate"
      />
    </NLayoutSider>
    <NLayout
      :native-scrollbar="false"
      content-style="padding: 20px;"
      style="background-color: #1b2636;"
    >
      <router-view />
    </NLayout>
  </NLayout>
</template>

<style scoped>
.logo {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0 16px 24px;
}

.logo-text {
  font-size: 16px;
  font-weight: 700;
  color: #63e2b7;
  letter-spacing: 1px;
  white-space: nowrap;
}
</style>
