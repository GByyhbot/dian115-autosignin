<template>
  <div class="autosignin-page">
    <n-card size="small" :bordered="false" class="hero">
      <div class="hero-row">
        <div>
          <div class="hero-title">站点自动签到</div>
          <div class="hero-sub">DIAN115 版 · 适配自 MoviePilot autosignin 3.0.3 · v{{ snapshot.version }}</div>
        </div>
        <div class="hero-actions">
          <n-button size="small" :loading="busy" @click="refresh">刷新</n-button>
          <n-button size="small" type="primary" :loading="saving" @click="save">保存配置</n-button>
          <n-button size="small" type="info" :loading="running" @click="run('sign')">立即签到</n-button>
          <n-button size="small" type="info" ghost :loading="running" @click="run('login')">立即登录</n-button>
          <n-button size="small" type="error" ghost :disabled="!hasHistory" @click="clearHistory">清空历史</n-button>
        </div>
      </div>
      <div v-if="liveNotice" class="live-banner">常驻运行中，调度按 Cron/周期/随机自动执行</div>
      <div v-else class="live-banner muted">预览模式：仅本地模拟配置保存</div>
    </n-card>

    <n-card size="small" title="全局设置" class="card">
      <n-form label-placement="left" label-width="120" size="small">
        <n-grid :cols="2" :x-gap="16" responsive="screen" item-responsive>
          <n-form-item-gi label="启用调度">
            <n-switch v-model:value="draft.enabled" />
            <span class="hint">关闭后不再按计划执行，仍可手动签到</span>
          </n-form-item-gi>
          <n-form-item-gi label="发送通知">
            <n-switch v-model:value="draft.notify" />
            <span class="hint">结果摘要发送到宿主通知渠道</span>
          </n-form-item-gi>
          <n-form-item-gi label="调度规则" span="2">
            <n-input v-model:value="draft.cron" placeholder="留空=每天随机2次；'0 8 * * *'=定时；'2.3/9-23'=周期窗口；'2.3'=每2.3小时" />
          </n-form-item-gi>
          <n-form-item-gi label="并发数">
            <n-input-number v-model:value="draft.queue_cnt" :min="1" :max="20" style="width: 120px" />
          </n-form-item-gi>
          <n-form-item-gi label="重试关键词">
            <n-input v-model:value="draft.retry_keyword" placeholder="正则，如：失败|错误" />
          </n-form-item-gi>
        </n-grid>
      </n-form>
      <div class="meta-line">
        上次执行：{{ lastRun }} · 状态：{{ heartbeatStatus }} · 启用站点：{{ enabledCount }}
        <span v-if="runtime?.last_error" class="error-text">错误：{{ runtime.last_error }}</span>
      </div>
    </n-card>

    <n-card size="small" title="站点列表" class="card">
      <template #header-extra>
        <n-button size="tiny" type="primary" dashed @click="addSite()">＋ 添加站点</n-button>
      </template>
      <div class="all-row">
        <span class="all-label">一键：</span>
        <label><input type="checkbox" v-model="allSign" /> 全部签到</label>
        <label><input type="checkbox" v-model="allLogin" /> 全部登录保号</label>
      </div>
      <div class="cred-tip">
        托管凭据（推荐）：插件中心 → 本安装实例 → 托管凭据（Secret Bindings）→ 按站点域名创建 Cookie 绑定，把返回的 <code>cred_xxx</code> 填入右侧「托管凭据引用」列；与直填 Cookie 二选一、优先托管凭据。自定义站点不在内置清单内，需直填 Cookie。
      </div>
      <n-data-table
        :columns="columns"
        :data="draft.sites"
        :bordered="false"
        :max-height="420"
        size="small"
        :row-key="(row: any) => row.id"
      />
      <n-empty v-if="draft.sites.length === 0" description="暂无站点，点击“添加站点”配置签到目标（内置 22 个站点预设，其余站点走通用签到）" style="padding: 24px 0" />
    </n-card>

    <n-card size="small" title="历史记录（最近 14 天）" class="card">
      <n-data-table :columns="historyColumns" :data="historyRows" :bordered="false" :max-height="360" size="small" :row-key="(row: any) => row.key" />
      <n-empty v-if="historyRows.length === 0" description="暂无历史记录" style="padding: 24px 0" />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, NCard, NDataTable, NEmpty, NForm, NFormItemGi, NGrid, NInput, NInputNumber, NSelect, NSwitch, NTag, useMessage } from 'naive-ui'
import { emptyConfig, PRESETS, type Bridge, type Config, type SiteRow, type State } from './types'

const props = defineProps<{ api: Bridge }>()

const snapshot = ref<State>({ config: emptyConfig(), config_revision: '', mode: 'preview-only', live_execution: false, version: '1.0.0' })
const draft = ref<Config>(emptyConfig())
const busy = ref(false)
const saving = ref(false)
const running = ref(false)
const message = useMessage()

const runtime = computed(() => snapshot.value.runtime)
const lastRun = computed(() => runtime.value?.last_run ? new Date(runtime.value.last_run).toLocaleString() : '—')
const heartbeatStatus = computed(() => snapshot.value.heartbeat?.status || '—')
const enabledCount = computed(() => snapshot.value.config.sites.length)
const hasHistory = computed(() => historyRows.value.length > 0)
const liveNotice = computed(() => snapshot.value.live_execution)

async function refresh() {
  busy.value = true
  try {
    const { state } = await props.api.getState()
    if (state) {
      snapshot.value = state
      draft.value = structuredClone(state.config)
    }
  } catch (e: any) {
    message.error('加载状态失败：' + (e?.message || e))
  } finally {
    busy.value = false
  }
}

async function save() {
  saving.value = true
  try {
    const res = await props.api.invokeAction('save-config', { config: draft.value, revision: snapshot.value.config_revision })
    if (res.result?.status === 'succeeded') {
      message.success('配置已保存')
      await refresh()
    } else {
      message.warning(res.result?.message || '保存失败')
      if (res.result?.message?.includes('refresh')) await refresh()
    }
  } catch (e: any) {
    message.error('保存失败：' + (e?.message || e))
  } finally {
    saving.value = false
  }
}

async function run(type: 'sign' | 'login') {
  running.value = true
  try {
    const res = await props.api.invokeAction('sign-now', { type })
    if (res.result?.status === 'succeeded') {
      const r = res.result.report
      if (r) message.success(`完成：全部 ${r.all}，成功 ${r.success}，仅登录 ${r.logged_in}，失败 ${r.failed}`)
      else message.success('执行完成')
      await refresh()
    } else {
      message.warning(res.result?.message || '执行失败')
    }
  } catch (e: any) {
    message.error('执行失败：' + (e?.message || e))
  } finally {
    running.value = false
  }
}

async function clearHistory() {
  const res = await props.api.invokeAction('clear-history', {})
  if (res.result?.status === 'succeeded') { message.success('历史已清空'); await refresh() }
  else message.warning(res.result?.message || '清空失败')
}

function genId() {
  return 'site-' + Math.random().toString(36).slice(2, 10)
}

function addSite(presetKey = '') {
  const preset = PRESETS.find(p => p.key === presetKey)
  const row: SiteRow = {
    id: genId(),
    key: preset?.key || '',
    name: preset?.name || '自定义站点',
    url: preset ? `https://${preset.domain}` : '',
    cookie: '',
    credential_ref: '',
    ua: '',
    timeout: 20,
    apikey: '',
    token: '',
    custom: !preset,
  }
  draft.value.sites.push(row)
}

function removeSite(row: SiteRow) {
  draft.value.sites = draft.value.sites.filter(s => s.id !== row.id)
  draft.value.sign_sites = draft.value.sign_sites.filter(id => id !== row.id)
  draft.value.login_sites = draft.value.login_sites.filter(id => id !== row.id)
}

function toggleSign(row: SiteRow, on: boolean) {
  const set = new Set(draft.value.sign_sites)
  if (on) set.add(row.id); else set.delete(row.id)
  if (set.has('all')) set.delete('all')
  draft.value.sign_sites = [...set]
}
function toggleLogin(row: SiteRow, on: boolean) {
  const set = new Set(draft.value.login_sites)
  if (on) set.add(row.id); else set.delete(row.id)
  if (set.has('all')) set.delete('all')
  draft.value.login_sites = [...set]
}
const allSign = computed({
  get: () => draft.value.sign_sites.includes('all'),
  set: (on: boolean) => {
    const set = new Set(draft.value.sign_sites)
    if (on) set.add('all'); else set.delete('all')
    draft.value.sign_sites = [...set]
  },
})
const allLogin = computed({
  get: () => draft.value.login_sites.includes('all'),
  set: (on: boolean) => {
    const set = new Set(draft.value.login_sites)
    if (on) set.add('all'); else set.delete('all')
    draft.value.login_sites = [...set]
  },
})
const signActive = (id: string) => draft.value.sign_sites.includes('all') || draft.value.sign_sites.includes(id)
const loginActive = (id: string) => draft.value.login_sites.includes('all') || draft.value.login_sites.includes(id)

const presetOptions = [
  { label: '通用签到（自定义站点）', value: '' },
  ...PRESETS.map(p => ({ label: `${p.name}（${p.domain}）`, value: p.key })),
]

const columns = [
  {
    title: '名称', key: 'name', width: 150,
    render: (row: SiteRow) => h(NInput, { value: row.name, size: 'small', onUpdateValue: (v: string) => { row.name = v } }),
  },
  {
    title: '预设', key: 'key', width: 210,
    render: (row: SiteRow) => h(NSelect, {
      value: row.key, size: 'small', options: presetOptions,
      onUpdateValue: (v: string) => {
        row.key = v || ''
        const p = PRESETS.find(x => x.key === v)
        row.custom = !p
        if (p) { row.name = p.name; if (!row.url) row.url = `https://${p.domain}` }
      },
    }),
  },
  { title: 'URL', key: 'url', minWidth: 200, render: (row: SiteRow) => h(NInput, { value: row.url, size: 'small', placeholder: 'https://站点域名', onUpdateValue: (v: string) => { row.url = v } }) },
  {
    title: 'Cookie', key: 'cookie', minWidth: 220,
    render: (row: SiteRow) => h(NInput, { value: row.cookie, size: 'small', type: 'password', showPasswordOn: 'click', placeholder: '站点 Cookie（与托管凭据二选一）', onUpdateValue: (v: string) => { row.cookie = v } }),
  },
  {
    title: '托管凭据引用', key: 'credential_ref', minWidth: 160,
    render: (row: SiteRow) => h(NInput, { value: row.credential_ref, size: 'small', placeholder: 'cred_xxx（推荐）', onUpdateValue: (v: string) => { row.credential_ref = v } }),
  },
  {
    title: 'Token/API Key', key: 'token', minWidth: 140,
    render: (row: SiteRow) => h(NInput, { value: row.apikey || row.token, size: 'small', placeholder: 'mTeam/Rousi', onUpdateValue: (v: string) => { row.apikey = v; row.token = v } }),
  },
  { title: '超时(s)', key: 'timeout', width: 90, render: (row: SiteRow) => h('span', { style: 'font-size:12px' }, String(row.timeout || 20)) },
  {
    title: '签到', key: 'sign', width: 70,
    render: (row: SiteRow) => h('input', { type: 'checkbox', checked: signActive(row.id), disabled: allSign.value, onChange: (e: any) => toggleSign(row, e.target.checked) }),
  },
  {
    title: '登录保号', key: 'login', width: 90,
    render: (row: SiteRow) => h('input', { type: 'checkbox', checked: loginActive(row.id), disabled: allLogin.value, onChange: (e: any) => toggleLogin(row, e.target.checked) }),
  },
  {
    title: '', key: 'actions', width: 60,
    render: (row: SiteRow) => h(NButton, { size: 'tiny', type: 'error', ghost: true, onClick: () => removeSite(row) }, { default: () => '删' }),
  },
]

interface HistoryRow { key: string; date: string; type: string; site: string; status: string; message: string; ok: boolean }
const historyRows = computed<HistoryRow[]>(() => {
  const rows: HistoryRow[] = []
  const h = runtime.value?.history
  if (!h) return rows
  for (const [type, days] of [['sign', h.sign], ['login', h.login]] as const) {
    for (const [date, entry] of Object.entries(days || {})) {
      for (const r of entry.results || []) {
        rows.push({
          key: `${type}-${date}-${r.site_id}-${rows.length}`,
          date, type, site: r.site,
          status: r.success ? '签到成功' : r.logged_in ? '仅登录' : '失败',
          message: r.message, ok: r.success || r.logged_in,
        })
      }
    }
  }
  rows.sort((a, b) => b.date.localeCompare(a.date))
  return rows.slice(0, 200)
})

const historyColumns = [
  { title: '日期', key: 'date', width: 110 },
  { title: '类型', key: 'type', width: 70, render: (row: HistoryRow) => h(NTag, { size: 'small', type: row.type === 'sign' ? 'primary' : 'info' }, { default: () => row.type === 'sign' ? '签到' : '登录' }) },
  { title: '站点', key: 'site', width: 160 },
  { title: '状态', key: 'status', width: 90, render: (row: HistoryRow) => h(NTag, { size: 'small', type: row.ok ? 'success' : 'error' }, { default: () => row.status }) },
  { title: '消息', key: 'message' },
]

onMounted(refresh)
</script>

<style scoped>
.autosignin-page { display: flex; flex-direction: column; gap: 12px; }
.hero-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.hero-title { font-size: 18px; font-weight: 600; }
.hero-sub { color: var(--n-text-color-3); font-size: 12px; margin-top: 2px; }
.hero-actions { display: flex; gap: 8px; }
.live-banner { margin-top: 8px; font-size: 12px; color: #18a058; }
.live-banner.muted { color: var(--n-text-color-3); }
.card { margin-top: 4px; }
.hint { margin-left: 8px; color: var(--n-text-color-3); font-size: 12px; }
.meta-line { margin-top: 4px; color: var(--n-text-color-3); font-size: 12px; }
.error-text { color: #d03050; margin-left: 8px; }
.all-row { margin-bottom: 8px; font-size: 12px; color: var(--n-text-color-3); display: flex; gap: 12px; }
.all-label { margin-right: 4px; }
.cred-tip { margin-bottom: 8px; font-size: 12px; color: var(--n-text-color-3); line-height: 1.6; }
.cred-tip code { background: var(--n-action-color); padding: 0 4px; border-radius: 3px; }
</style>
