// Chinese (Simplified) — the project's default locale.
// Keys are organised by feature area so additions stay scannable.
export const zhCN = {
  app: {
    name: '游戏修改器大师',
    dataSource: '数据源：FLiNG Trainer',
  },
  nav: {
    home: '游戏列表',
    downloads: '我的修改器',
    settings: '设置',
  },
  home: {
    title: '游戏库',
    searchPlaceholder: '搜索游戏（中英文，本地即时匹配）…',
    countSuffix: '款',
    refresh: '刷新数据',
    cancelRefresh: '取消刷新（进度已保存）',
    refreshStatus: '抓取中 {current}/{total} ({percent}%) · {games} 游戏',
    refreshStatusWithErrors: '抓取中 {current}/{total} ({percent}%) · {games} 游戏 · {errors} 失败',
    refreshCancelled: '已请求取消刷新（进度已保存）',
    empty: '暂无游戏数据',
    loadData: '立即加载',
    crawling: '抓取中',
    games: '游戏',
    failed: '失败',
  },
  detail: {
    back: '返回',
    loading: '正在从原站获取修改器版本…',
    loadFailed: '详情加载失败',
    retry: '重试',
    notFound: '未找到游戏信息',
    options: '选项',
    updated: '更新',
    viewSource: '查看原站 →',
    versions: '修改器版本',
    versionCount: '{count} 个版本',
    columns: {
      version: '修改器版本',
      gameVersion: '游戏版本',
      size: '大小',
      downloads: '下载次数',
      updatedAt: '更新时间',
      status: '状态',
      actions: '操作',
    },
    actions: {
      download: '下载',
      install: '安装',
      launch: '启动',
      cancel: '取消',
    },
    status: {
      available: '可用',
      downloaded: '已下载',
      installed: '已安装',
    },
    confirm: {
      download: {
        title: '下载修改器',
        content: '将下载到您设置的下载目录中，下载完成后会自动解压。是否继续？',
        ok: '下载',
      },
      install: {
        title: '标记为已安装',
        content: '确认将此修改器标记为「已安装」？您之后可一键启动。',
        ok: '确认',
      },
    },
    toast: {
      downloadDone: '下载完成',
      downloadCancelled: '已取消下载',
      downloadFailed: '下载失败：{msg}',
      installDone: '已标记为已安装',
      installFailed: '安装失败：{msg}',
      launchDone: '已启动',
      launchFailed: '启动失败：{msg}',
      loadFailed: '详情加载失败：{msg}',
      retryFailed: '重试失败：{msg}',
    },
  },
  downloads: {
    title: '我的修改器',
    countSuffix: '个',
    empty: '还没有下载任何修改器',
    actions: {
      launch: '启动',
      delete: '删除',
    },
    confirm: {
      delete: {
        title: '删除修改器',
        content: '将删除「{name}」的本地文件与状态。此操作不可恢复，是否继续？',
        ok: '删除',
      },
    },
    toast: {
      launchDone: '已启动：{name}',
      launchFailed: '启动失败：{msg}',
      deleteDone: '已删除',
      deleteFailed: '删除失败：{msg}',
      loadFailed: '加载列表失败：{msg}',
    },
  },
  settings: {
    title: '设置',
    language: '界面语言',
    languageZh: '简体中文',
    languageEn: 'English',
    dataDir: '数据目录',
    downloadDir: '下载目录',
    pickFolder: '选择文件夹',
    mappingCount: '名称映射条目',
    stats: '统计信息',
  },
  common: {
    cancel: '取消',
    confirm: '确认',
  },
  usage: {
    title: '📖 使用说明(中文快捷键指南)',
    intro:
      '下载并启动修改器后,它会打开一个独立窗口,顶部列出所有功能及对应的快捷键。下面是 FLiNG 修改器的通用按键方案(绝大多数游戏一致,具体以修改器窗口显示为准):',
    shortcuts: {
      f1: {
        label: '开关 / 静音',
        desc: '开启或关闭所有修改功能;长按可静音提示音',
      },
      home: {
        label: '关闭全部',
        desc: '一键关闭所有已开启的修改功能',
      },
      num0: {
        label: '全部开启',
        desc: '一键开启所有可用的修改功能',
      },
      num1to8: {
        label: '功能 1-8',
        desc: '小键盘数字键 1 到 8,分别对应修改器窗口里列出的第 1~8 项功能,按一下开/关',
      },
      num9: {
        label: '检查更新',
        desc: '检测该修改器是否有新版本(部分修改器支持)',
      },
    },
    notes: {
      title: '⚠ 使用注意',
      version: '修改器版本必须与你的游戏版本匹配,否则可能无效或闪退。请在本页选择对应游戏版本的修改器下载。',
      order: '建议先启动游戏进入主界面/存档,再启动修改器(以管理员身份运行修改器可解决大部分无效问题)。',
      antivirus: '修改器可能被杀毒软件误报。如确认从 FLiNG 官方下载,可加入白名单或临时关闭杀毒后再启动。',
      oneday: '修改器是按"键"切换的,不是按"按住"。按一下即开,再按一下即关,修改器窗口里对应项会有 √ 或颜色变化。',
    },
  },
}
