import { createI18n } from 'vue-i18n'
import { zhCN } from './locales/zh-CN'
import { en } from './locales/en'

export type AppLocale = 'zh-CN' | 'en'

const STORAGE_KEY = 'gamm.locale'

function detectInitialLocale(): AppLocale {
  // 1. Explicit user choice persisted in localStorage.
  try {
    const saved = localStorage.getItem(STORAGE_KEY) as AppLocale | null
    if (saved === 'zh-CN' || saved === 'en') return saved
  } catch {
    // localStorage may be unavailable (e.g. sandboxed iframe) — fall through.
  }
  // 2. Browser language preference.
  const nav = (typeof navigator !== 'undefined' ? navigator.language : '') || ''
  if (nav.toLowerCase().startsWith('en')) return 'en'
  return 'zh-CN' // default — the project ships Chinese-first
}

export function rememberLocale(locale: AppLocale) {
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {
    // ignore
  }
}

export const i18n = createI18n({
  legacy: false,
  locale: detectInitialLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    en,
  },
})

// Convenience composable for use in setup scripts.
export function useLocale() {
  function setLocale(locale: AppLocale) {
    i18n.global.locale.value = locale
    rememberLocale(locale)
  }
  function toggle() {
    setLocale(i18n.global.locale.value === 'zh-CN' ? 'en' : 'zh-CN')
  }
  return {
    locale: i18n.global.locale,
    setLocale,
    toggle,
  }
}
