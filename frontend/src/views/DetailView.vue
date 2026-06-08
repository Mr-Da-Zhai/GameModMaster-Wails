<script setup lang="ts">
import { useRoute } from 'vue-router'
import { NButton } from 'naive-ui'

const route = useRoute()
const gameId = route.params.id
</script>

<template>
  <div class="page-container">
    <NButton quaternary size="small" @click="$router.back()" style="margin-bottom: 16px;">← 返回</NButton>
    <h2>游戏详情</h2>
    <p style="color: #999; margin-top: 12px;">Game ID: {{ gameId }}</p>
  </div>
</template>

<style scoped>
.page-container {
  padding: 16px;
}
</style>
