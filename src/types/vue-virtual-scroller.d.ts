declare module 'vue-virtual-scroller' {
  import { DefineComponent, Plugin } from 'vue'

  export interface RecycleScrollerProps {
    items: any[]
    itemSize: number | ((item: any) => number)
    buffer?: number
    keyField?: string
    pageMode?: boolean
    prerender?: number
    emitUpdate?: boolean
  }

  export const RecycleScroller: DefineComponent<RecycleScrollerProps>
  export const DynamicScroller: DefineComponent<any>
  export const DynamicScrollerItem: DefineComponent<any>

  const VueVirtualScroller: Plugin
  export default VueVirtualScroller
}
