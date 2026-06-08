<script setup lang="ts">
import { darkTheme } from 'naive-ui'
import { NConfigProvider } from 'naive-ui'
import MainLayout from './components/MainLayout.vue'
</script>

<template>
  <NConfigProvider :theme="darkTheme">
    <MainLayout />
  </NConfigProvider>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  background-color: #1b2636;
  color: #fff;
}

#app {
  width: 100vw;
  height: 100vh;
}
</style>
