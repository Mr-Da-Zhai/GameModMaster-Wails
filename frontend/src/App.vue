<script setup lang="ts">
import { darkTheme, type GlobalThemeOverrides } from 'naive-ui'
import { NConfigProvider, NMessageProvider, NDialogProvider, NNotificationProvider } from 'naive-ui'
import MainLayout from './components/MainLayout.vue'

// Hybrid visual language: Apple-style spacing + game-cover-led grids + a
// restrained two-color accent system. Deep space-black surfaces (not the
// cold blue-grey of the old theme) read as immersive rather than
// "ops dashboard", while a single electric cyan + warm violet accent keeps
// things bold without being garish.
const themeOverrides: GlobalThemeOverrides = {
  common: {
    // Electric cyan primary — punchier than the old teal, reads as
    // "gaming/neon" rather than "corporate".
    primaryColor: '#22d3ee',
    primaryColorHover: '#67e8f9',
    primaryColorPressed: '#06b6d4',
    primaryColorSuppl: '#22d3ee',

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

    // Deep space-black surfaces — immersive, not cold.
    bodyColor: '#08080a',
    cardColor: '#131316',
    modalColor: '#131316',
    popoverColor: '#1c1c20',
    tableColor: '#131316',
    tableHeaderColor: '#1a1a1e',

    textColorBase: '#ededf0',
    textColor1: '#f4f4f6',
    textColor2: '#c8c8cc',
    textColor3: '#8a8a90',

    // Larger, softer radii — Apple-ish.
    borderRadius: '12px',
    borderRadiusSmall: '10px',

    fontSize: '14px',
    fontSizeSmall: '13px',

    // Borders are barely-there — we separate by space and shadow, not lines.
    dividerColor: '#222226',
    borderColor: '#222226',
  },
  Button: {
    borderRadiusMedium: '10px',
    borderRadiusSmall: '8px',
    fontWeight: '500',
  },
  Card: {
    borderRadius: '16px',
    paddingMedium: '20px 24px',
    paddingSmall: '16px 20px',
  },
  DataTable: {
    borderRadius: '12px',
    fontSizeMedium: '14px',
    fontSizeSmall: '13px',
    thColor: '#1a1a1e',
    thTextColor: '#8a8a90',
    tdColor: '#131316',
    borderColor: '#1f1f23',
  },
  Input: {
    borderRadius: '10px',
  },
  Menu: {
    borderRadius: '10px',
    itemHeight: '42px',
  },
  Tag: {
    borderRadius: '6px',
  },
}
</script>

<template>
  <NConfigProvider :theme="darkTheme" :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NDialogProvider>
        <NNotificationProvider>
          <MainLayout />
        </NNotificationProvider>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

:root {
  /* Surface palette — deep space blacks, layered for depth.
     Old theme was cold blue-grey (#0f172a → #1e293b); this is warmer-neutral
     black which reads as immersive rather than corporate. */
  --surface-0: #08080a; /* app background */
  --surface-1: #131316; /* cards / tables */
  --surface-2: #1a1a1e; /* headers / hover */
  --surface-3: #1c1c20; /* popovers */
  --border: #222226;
  --border-soft: #1f1f23;

  /* Accent system: electric cyan (primary) + warm violet (secondary).
     Used sparingly — most of the UI is monochrome with these as glints. */
  --accent: #22d3ee;
  --accent-hover: #67e8f9;
  --accent-glow: rgba(34, 211, 238, 0.16);
  --accent-2: #a78bfa; /* warm violet secondary accent */

  /* Text — slightly warmer whites than pure #fff to avoid harshness. */
  --text-1: #f4f4f6;
  --text-2: #c8c8cc;
  --text-3: #8a8a90;
}

html,
body,
#app {
  width: 100%;
  height: 100%;
  overflow: hidden;
}

body {
  /* Inter is now actually loaded via index.html; falls back to system UI
     fonts offline. PingFang SC / Microsoft YaHei cover Chinese glyphs. */
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI',
    'PingFang SC', 'Microsoft YaHei', Roboto, sans-serif;
  /* Subtle radial glows in the deep black — gives the app a sense of space
     without any obvious background image. Cyan top-right, violet bottom-left,
     echoing the two-color accent system. */
  background:
    radial-gradient(1100px 560px at 88% -8%, rgba(34, 211, 238, 0.07), transparent 60%),
    radial-gradient(900px 480px at -8% 108%, rgba(167, 139, 250, 0.06), transparent 55%),
    var(--surface-0);
  color: var(--text-1);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  font-feature-settings: 'cv11', 'ss01'; /* Inter: cleaner digits / alt single-storey a */
}

/* Slim, unobtrusive scrollbars */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: #2a2a30;
  border-radius: 8px;
}
::-webkit-scrollbar-thumb:hover {
  background: #3a3a42;
}
</style>
