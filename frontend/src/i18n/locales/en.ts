// English locale — mirrors zh-CN key-for-key.
export const en = {
  app: {
    name: 'Game Mod Master',
    dataSource: 'Source: FLiNG Trainer',
  },
  nav: {
    home: 'Library',
    downloads: 'My Trainers',
    settings: 'Settings',
  },
  home: {
    title: 'Game Library',
    searchPlaceholder: 'Search games (Chinese or English; instant local match)…',
    countSuffix: '',
    refresh: 'Refresh data',
    cancelRefresh: 'Cancel refresh (progress is saved)',
    refreshStatus: 'Crawling {current}/{total} ({percent}%) · {games} games',
    refreshStatusWithErrors: 'Crawling {current}/{total} ({percent}%) · {games} games · {errors} failed',
    refreshCancelled: 'Refresh cancel requested (progress saved)',
    empty: 'No games yet',
    loadData: 'Load now',
    crawling: 'Crawling',
    games: 'games',
    failed: 'failed',
  },
  detail: {
    back: 'Back',
    loading: 'Fetching trainer versions from source…',
    loadFailed: 'Failed to load details',
    retry: 'Retry',
    notFound: 'Game not found',
    options: 'Options',
    updated: 'Updated',
    viewSource: 'View on source →',
    versions: 'Trainer versions',
    versionCount: '{count} versions',
    columns: {
      version: 'Trainer version',
      gameVersion: 'Game version',
      size: 'Size',
      downloads: 'Downloads',
      updatedAt: 'Updated',
      status: 'Status',
      actions: 'Actions',
    },
    actions: {
      download: 'Download',
      install: 'Install',
      launch: 'Launch',
      cancel: 'Cancel',
    },
    status: {
      available: 'Available',
      downloaded: 'Downloaded',
      installed: 'Installed',
    },
    confirm: {
      download: {
        title: 'Download trainer',
        content: 'This will download to your configured folder and auto-extract the archive. Continue?',
        ok: 'Download',
      },
      install: {
        title: 'Mark as installed',
        content: 'Mark this trainer as installed? You can then launch it with one click.',
        ok: 'Confirm',
      },
    },
    toast: {
      downloadDone: 'Download complete',
      downloadCancelled: 'Download cancelled',
      downloadFailed: 'Download failed: {msg}',
      installDone: 'Marked as installed',
      installFailed: 'Install failed: {msg}',
      launchDone: 'Launched',
      launchFailed: 'Launch failed: {msg}',
      loadFailed: 'Failed to load details: {msg}',
      retryFailed: 'Retry failed: {msg}',
    },
  },
  downloads: {
    title: 'My Trainers',
    countSuffix: '',
    empty: 'No trainers downloaded yet',
    actions: {
      launch: 'Launch',
      delete: 'Delete',
    },
    confirm: {
      delete: {
        title: 'Delete trainer',
        content: 'Delete the local files and state for "{name}"? This cannot be undone.',
        ok: 'Delete',
      },
    },
    toast: {
      launchDone: 'Launched: {name}',
      launchFailed: 'Launch failed: {msg}',
      deleteDone: 'Deleted',
      deleteFailed: 'Delete failed: {msg}',
      loadFailed: 'Failed to load list: {msg}',
    },
  },
  settings: {
    title: 'Settings',
    language: 'Language',
    languageZh: '简体中文',
    languageEn: 'English',
    dataDir: 'Data directory',
    downloadDir: 'Download directory',
    pickFolder: 'Pick folder',
    mappingCount: 'Name mapping entries',
    stats: 'Statistics',
  },
  common: {
    cancel: 'Cancel',
    confirm: 'Confirm',
  },
  usage: {
    title: '📖 Usage Guide (keyboard shortcuts)',
    intro:
      "After launching, the trainer opens its own window listing every feature and its hotkey. These are FLiNG's standard bindings (most games are identical; always confirm against the trainer window):",
    shortcuts: {
      f1: {
        label: 'Toggle / mute',
        desc: 'Turn all features on or off; hold to mute sound prompts',
      },
      home: {
        label: 'Disable all',
        desc: 'One-key disable for every active feature',
      },
      num0: {
        label: 'Enable all',
        desc: 'One-key enable for every available feature',
      },
      num1to8: {
        label: 'Feature 1-8',
        desc: 'Numpad keys 1-8 toggle features 1-8 listed in the trainer window — press once to enable, again to disable',
      },
      num9: {
        label: 'Check update',
        desc: 'Check whether a newer trainer version exists (some trainers only)',
      },
    },
    notes: {
      title: '⚠ Notes',
      version: 'The trainer version MUST match your game version, otherwise it may not work or may crash. Pick the matching one on this page.',
      order: 'Launch the game first and reach the main menu / a save, THEN launch the trainer (run the trainer as administrator to fix most "not working" issues).',
      antivirus: 'Trainers are often false-flagged by antivirus. If you downloaded from the official FLiNG site, whitelist it or temporarily disable AV before launching.',
      oneday: 'Trainers toggle on key PRESS, not key HOLD. Press once to enable, again to disable — the row in the trainer window shows a √ or color change.',
    },
  },
}
