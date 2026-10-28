import { createApp, defineComponent, h, shallowReactive, shallowRef } from 'vue'
import { NConfigProvider, NMessageProvider, darkTheme } from 'naive-ui'
import { connectPluginPage } from '../.dian115/examples/sdk/page'
import type { PluginPageConnection, PluginPageProps, PluginUIBridge } from '../.dian115/examples/sdk/ui'
import AppPage from './AppPage.vue'
import { emptyConfig, type Bridge, type State } from './types'
import './preview.css'

// 本地预览桥：保存为内存模拟；签到/登录需真实宿主运行时。
function createPreview(): PluginUIBridge {
  let snapshot: State = {
    config: emptyConfig(),
    config_revision: '',
    mode: 'preview-only',
    live_execution: false,
    version: 'preview',
  }
  let revision = 0
  const unsupported = async (): Promise<never> => { throw new Error('本地预览未连接宿主') }
  return {
    capabilities: { isolation: 'opaque-origin-v1', storage: 'host-ui-v1', jobs: true },
    storage: { get: async <T,>() => null as T | null, set: async () => undefined, remove: async () => undefined, clear: async () => undefined },
    jobs: { start: unsupported, get: unsupported, cancel: unsupported, resume: unsupported },
    getState: async () => ({ state_version: 'preview-v1', state: structuredClone(snapshot) as unknown as Record<string, unknown> }),
    refresh: async () => structuredClone(snapshot) as unknown as Record<string, unknown>,
    invokeAction: async (action: string, input: any) => {
      if (action === 'save-config') {
        if (input.revision !== snapshot.config_revision) return { invocation_id: 'preview', replayed: false, result: { status: 'failed', message: '配置冲突' } }
        snapshot = { ...snapshot, config: structuredClone(input.config), config_revision: `preview-${++revision}` }
        return { invocation_id: 'preview', replayed: false, result: { status: 'succeeded' } }
      }
      if (action === 'clear-history') {
        snapshot = { ...snapshot, runtime: undefined }
        return { invocation_id: 'preview', replayed: false, result: { status: 'succeeded' } }
      }
      return { invocation_id: 'preview', replayed: false, result: { status: 'skipped', message: '本地页面仅模拟配置保存；签到请安装到 DIAN115 由真实运行时执行。' } }
    },
    reportError: async () => 'diag_preview',
  }
}

function theme() {
  const style = document.documentElement.style
  const color = (name: string, fallback: string) => style.getPropertyValue(`--dian-${name}`).trim() || fallback
  return {
    theme: color('color-scheme', 'light') === 'dark' ? darkTheme : null,
    themeOverrides: { common: { primaryColor: color('primary', '#2196f3'), primaryColorHover: color('primary-hover', '#1976d2'), primaryColorPressed: color('primary-pressed', '#1565c0') } },
  }
}

async function start() {
  let connection: PluginPageConnection | undefined
  let initial: PluginPageProps
  if (import.meta.env.DEV && window.parent === window) {
    initial = { api: createPreview(), installationId: 1, pluginId: 'community.dian115-autosignin', runtime: null, runtimeState: { preview: true }, navKey: 'main', themeContract: 'dian115-theme-v1' }
  } else {
    connection = await connectPluginPage()
    initial = { api: connection.api, ...connection.context }
  }
  const props = shallowReactive(initial)
  const appearance = shallowRef(theme())
  connection?.subscribe(context => { Object.assign(props, context); appearance.value = theme() })
  const Page = defineComponent({
    setup: () => () => h(NConfigProvider, appearance.value, {
      default: () => h(NMessageProvider, {}, { default: () => h(AppPage, { api: props.api as unknown as Bridge }) }),
    }),
  })
  const app = createApp(Page)
  app.config.errorHandler = reason => {
    console.error('[plugin-page]', reason)
    void props.api.reportError('ui_render', reason).catch(() => undefined)
  }
  app.mount('#app')
}

void start().catch(reason => {
  const root = document.getElementById('app')
  if (root) root.textContent = reason instanceof Error ? reason.message : '插件页面启动失败'
})
