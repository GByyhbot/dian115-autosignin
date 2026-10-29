export interface SiteRow {
  id: string
  key: string // 内置处理器预设键，"" 表示通用
  name: string
  url: string
  cookie: string
  credential_ref: string
  session: string // 受管 Cookie 会话名（5.0.54+），空则按站点 ID 自动生成
  ua: string
  timeout: number
  apikey: string // Rousi Pro API Key
  token: string // mTeam / Rousi 兼容 Authorization
  custom: boolean
}
export interface Config {
  schema_version: number
  enabled: boolean
  cron: string // "0 8 * * *" | "2.3/9-23" | "2.3" | ""（随机）
  notify: boolean
  queue_cnt: number
  sign_sites: string[]
  login_sites: string[]
  retry_keyword: string
  sites: SiteRow[]
}
export interface HistoryResult { site: string; site_id: string; success: boolean; logged_in: boolean; message: string }
export interface DayEntry { do: string[]; retry: string[]; results: HistoryResult[] }
export interface History { sign: Record<string, DayEntry>; login: Record<string, DayEntry> }
export interface RuntimeState {
  last_run: string
  last_cron_key: string
  last_error: string
  random_date: string
  random_times: string[]
  history: History
}
export interface Heartbeat { at: string; enabled_sites: number; status: string }
export interface CookieCloudDomain { domain: string; cookie_count: number }
export interface CookieCloudOverview { revision?: string; domains?: CookieCloudDomain[] }
export interface State {
  config: Config
  config_revision: string
  mode: string
  live_execution: boolean
  version: string
  heartbeat?: Heartbeat
  runtime?: RuntimeState
  features?: Record<string, boolean>
  cookiecloud?: CookieCloudOverview
  presets?: Preset[]
}
export interface Report {
  type: string
  all: number
  done: number
  next: number
  success: number
  logged_in: number
  failed: number
  message: string
  results: HistoryResult[]
}
export interface Result { status: 'succeeded' | 'failed' | 'accepted' | 'skipped'; message?: string; report?: Report; imported?: number; total_cookies?: number; cookiecloud?: CookieCloudOverview }
export interface Bridge {
  getState(view?: string): Promise<{ state?: State; etag?: string; not_modified?: boolean }>
  invokeAction(action: string, input?: unknown): Promise<{ result?: Result }>
  refresh(): Promise<State>
}
export interface Preset { key: string; name: string; domain: string; homepage?: string; group: 'major' | 'nexus' }

// 兜底列表（后端 state.presets 未下发时使用）；完整 166 站由 WASM 运行时下发。
export const PRESETS: Preset[] = [
  { key: '52pt', name: '52PT', domain: '52pt.site', homepage: 'https://52pt.site/', group: 'major' },
  { key: 'btschool', name: '学校（BTSchool）', domain: 'pt.btschool.club', homepage: 'https://pt.btschool.club/', group: 'major' },
  { key: 'chdbits', name: '彩虹岛（CHDBits）', domain: 'ptchdbits.co', homepage: 'https://ptchdbits.co/', group: 'major' },
  { key: 'haidan', name: '海胆之家（HaiDan）', domain: 'haidan.cc', homepage: 'https://haidan.cc/', group: 'major' },
  { key: 'hares', name: '白兔（Hares）', domain: 'club.hares.top', homepage: 'https://club.hares.top/', group: 'major' },
  { key: 'hdarea', name: '好大（HDArea）', domain: 'hdarea.club', homepage: 'https://hdarea.club/', group: 'major' },
  { key: 'hdchina', name: '瓷器（HDChina）', domain: 'hdchina.org', homepage: 'https://hdchina.org/', group: 'major' },
  { key: 'hdcity', name: '城市（HDCity）', domain: 'hdcity.city', homepage: 'https://hdcity.city/', group: 'major' },
  { key: 'hdsky', name: '天空（HDSky）', domain: 'hdsky.me', homepage: 'https://hdsky.me/', group: 'major' },
  { key: 'hdupt', name: 'HDUPT', domain: 'pt.hdupt.com', homepage: 'https://pt.hdupt.com/', group: 'major' },
  { key: 'mteam', name: '馒头（M-Team）', domain: 'm-team', group: 'major' },
  { key: 'nexushd', name: 'NexusHD', domain: 'v6.nexushd.org', homepage: 'https://v6.nexushd.org/', group: 'major' },
  { key: 'opencd', name: '皇后（OpenCD）', domain: 'open.cd', homepage: 'https://open.cd/', group: 'major' },
  { key: 'pterclub', name: '猫（PTerClub）', domain: 'pterclub.com', homepage: 'https://pterclub.com/', group: 'major' },
  { key: 'pttime', name: 'PT时间（PTTime）', domain: 'pttime.org', homepage: 'https://pttime.org/', group: 'major' },
  { key: 'ptzone', name: 'PTZONE', domain: 'ptzone.xyz', homepage: 'https://ptzone.xyz/', group: 'major' },
  { key: 'rousi', name: 'Rousi Pro', domain: 'rousi.pro', group: 'major' },
  { key: 'tjupt', name: '北洋（TJUPT）', domain: 'tjupt.org', homepage: 'https://tjupt.org/', group: 'major' },
  { key: 'ttg', name: 'TTG', domain: 'totheglory.im', group: 'major' },
  { key: 'u2', name: 'U2', domain: 'u2.dmhy.org', homepage: 'https://u2.dmhy.org/', group: 'major' },
  { key: 'yema', name: 'YemaPT', domain: 'yemapt.org', group: 'major' },
  { key: 'zhuque', name: '朱雀（ZHUQUE）', domain: 'zhuque.in', homepage: 'https://zhuque.in/', group: 'major' },
]

export const emptyConfig = (): Config => ({
  schema_version: 1,
  enabled: false,
  cron: '',
  notify: true,
  queue_cnt: 5,
  sign_sites: [],
  login_sites: [],
  retry_keyword: '',
  sites: [],
})
