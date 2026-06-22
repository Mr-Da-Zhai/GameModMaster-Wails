<script setup lang="ts">
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider } from 'naive-ui'
import MainLayout from './components/MainLayout.vue'

// Modern dark theme: deep midnight surfaces with a teal-emerald accent.
// Rounded corners, soft elevation, and comfortable spacing throughout.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#2dd4bf',
    primaryColorHover: '#5eead4',
    primaryColorPressed: '#14b8a6',
    primaryColorSuppl: '#2dd4bf',

    infoColor: '#38bdf8',
    infoColorHover: '#7dd3fc',
    infoColorPressed: '#0ea5e9',

    successColor: '#34d399',
    successColorHover: '#6ee7b7',
    successColorPressed: '#10b981',

    warningColor: '#fbbf24',
    errorColor: '#f87171',
    errorColorHover: '#fca5a5',
    errorColorPressed: '#ef4444',

    bodyColor: '#0f172a',
    cardColor: '#1e293b',
    modalColor: '#1e293b',
    popoverColor: '#273449',
    tableColor: '#1e293b',
    tableHeaderColor: '#243349',

    textColorBase: '#e2e8f0',
    textColor1: '#f1f5f9',
    textColor2: '#cbd5e1',
    textColor3: '#94a3b8',

    borderRadius: '10px',
    borderRadiusSmall: '8px',

    fontSize: '14px',
    fontSizeSmall: '13px',

    // Subtle borders that read as elevation, not hard lines.
    dividerColor: '#334155',
    borderColor: '#334155',
  },
  Button: {
    borderRadiusMedium: '8px',
    borderRadiusSmall: '6px',
    fontWeight: '500',
  },
  Card: {
    borderRadius: '14px',
    paddingMedium: '20px 24px',
    paddingSmall: '16px 20px',
  },
  DataTable: {
    borderRadius: '10px',
    fontSizeMedium: '14px',
    fontSizeSmall: '13px',
    thColor: '#243349',
    thTextColor: '#94a3b8',
    tdColor: '#1e293b',
    borderColor: '#28384f',
  },
  Input: {
    borderRadius: '8px',
  },
  Menu: {
    borderRadius: '8px',
    itemHeight: '42px',
  },
  Tag: {
    borderRadius: '6px',
  },
}
</script>

<template>
  <NConfigProvider :theme="darkTheme" :theme-overrides="themeOverrides">
    <NLoadingProvider>
      <NMessageProvider>
        <NDialogProvider>
          <NNotificationProvider>
            <MainLayout />
          </NNotificationProvider>
        </NDialogProvider>
      </NMessageProvider>
    </NLoadingProvider>
  </NConfigProvider>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

:root {
  /* Surface palette — layered for depth */
  --surface-0: #0f172a; /* app background */
  --surface-1: #1e293b; /* cards / tables */
  --surface-2: #243349; /* headers / hover */
  --surface-3: #273449; /* popovers */
  --border: #334155;
  --border-soft: #28384f;

  /* Accent */
  --accent: #2dd4bf;
  --accent-glow: rgba(45, 212, 191, 0.18);

  /* Text */
  --text-1: #f1f5f9;
  --text-2: #cbd5e1;
  --text-3: #94a3b8;
}

html,
body,
#app {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

body {
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI',
    'PingFang SC', 'Microsoft YaHei', Roboto, sans-serif;
  background:
    radial-gradient(1200px 600px at 80% -10%, rgba(45, 212, 191, 0.08), transparent 60%),
    radial-gradient(900px 500px at -10% 110%, rgba(56, 189, 248, 0.06), transparent 55%),
    var(--surface-0);
  color: var(--text-1);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

/* Slim, modern scrollbars */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: #3b4a63;
  border-radius: 8px;
}
::-webkit-scrollbar-thumb:hover {
  background: #4b5d7a;
}
</style>
