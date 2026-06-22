import { useDialog, useMessage } from 'naive-ui'

/**
 * Tiny helper around Naive UI's dialog/message APIs that gives every view a
 * consistent way to (a) confirm destructive actions and (b) surface errors
 * as user-visible toasts instead of console-only logs.
 *
 * Usage inside a component that is a descendant of NDialogProvider/NMessageProvider:
 *   const { confirm, toast } = useConfirm()
 *   await confirm({ title: '删除', content: '确认删除？', type: 'warning' })
 *   toast.error('下载失败')
 */
export function useFeedback() {
  const dialog = useDialog()
  const message = useMessage()

  function confirm(opts: {
    title: string
    content?: string
    type?: 'info' | 'success' | 'warning' | 'error'
    positiveText?: string
    negativeText?: string
  }): Promise<boolean> {
    return new Promise((resolve) => {
      dialog[opts.type || 'warning']({
        title: opts.title,
        content: opts.content || '',
        positiveText: opts.positiveText || '确认',
        negativeText: opts.negativeText || '取消',
        onPositiveClick: () => resolve(true),
        onNegativeClick: () => resolve(false),
        onMaskClick: () => resolve(false),
        onClose: () => resolve(false),
      })
    })
  }

  return {
    confirm,
    toast: message,
  }
}
