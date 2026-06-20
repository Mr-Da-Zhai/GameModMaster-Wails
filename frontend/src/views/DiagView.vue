<script setup lang="ts">
import { onMounted, ref } from 'vue'
import * as AppService from '../../bindings/GameModMaster/appservice'

const log = ref<string[]>([])
function add(s: string) {
  log.value.push(s)
  // eslint-disable-next-line no-console
  console.log('[diag]', s)
}

onMounted(async () => {
  add('诊断开始')
  try {
    add('1. 调用 GetTotalGames...')
    const total = await AppService.GetTotalGames()
    add(`   GetTotalGames = ${JSON.stringify(total)}`)
  } catch (e) {
    add(`   GetTotalGames 失败: ${String(e)}`)
  }

  try {
    add('2. 调用 GetTrainers(1, 50)...')
    const result = await AppService.GetTrainers(1, 50)
    add(`   GetTrainers 返回类型: ${typeof result}`)
    add(`   是否数组: ${Array.isArray(result)}`)
    const arr = result as any
    add(`   长度: ${Array.isArray(arr) ? arr.length : 'N/A'}`)
    if (Array.isArray(arr) && arr.length > 0) {
      add(`   第一条: ${JSON.stringify(arr[0])}`)
    }
  } catch (e) {
    add(`   GetTrainers 失败: ${String(e)}`)
  }

  try {
    add('3. 调用 GetDataDir...')
    const dir = await AppService.GetDataDir()
    add(`   GetDataDir = ${JSON.stringify(dir)}`)
  } catch (e) {
    add(`   GetDataDir 失败: ${String(e)}`)
  }
})
</script>

<template>
  <div style="padding: 24px; color: #fff; font-family: monospace; font-size: 13px; white-space: pre-wrap;">
    <div v-for="(line, i) in log" :key="i">{{ line }}</div>
  </div>
</template>
