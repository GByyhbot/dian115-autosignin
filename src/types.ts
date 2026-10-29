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
export interface Preset { key: string; name: string; domain: string }

export const PRESETS: Preset[] = [
  { key: '52pt', name: '52PT', domain: '52pt.site' },
  { key: 'btschool', name: '学校（BTSchool）', domain: 'pt.btschool.club' },
  { key: 'chdbits', name: '彩虹岛（CHDBits）', domain: 'ptchdbits.co' },
  { key: 'haidan', name: '海胆之家（HaiDan）', domain: 'haidan.cc' },
  { key: 'hares', name: '白兔（Hares）', domain: 'club.hares.top' },
  { key: 'hdarea', name: '好大（HDArea）', domain: 'hdarea.club' },
  { key: 'hdchina', name: '瓷器（HDChina）', domain: 'hdchina.org' },
  { key: 'hdcity', name: '城市（HDCity）', domain: 'hdcity.city' },
  { key: 'hdsky', name: '天空（HDSky）', domain: 'hdsky.me' },
  { key: 'hdupt', name: 'HDUPT', domain: 'pt.hdupt.com' },
  { key: 'mteam', name: '馒头（M-Team）', domain: 'm-team' },
  { key: 'nexushd', name: 'NexusHD', domain: 'v6.nexushd.org' },
  { key: 'opencd', name: '皇后（OpenCD）', domain: 'open.cd' },
  { key: 'pterclub', name: '猫（PTerClub）', domain: 'pterclub.com' },
  { key: 'pttime', name: 'PT时间（PTTime）', domain: 'pttime.org' },
  { key: 'ptzone', name: 'PTZONE', domain: 'ptzone.xyz' },
  { key: 'rousi', name: 'Rousi Pro', domain: 'rousi.pro' },
  { key: 'tjupt', name: '北洋（TJUPT）', domain: 'tjupt.org' },
  { key: 'ttg', name: 'TTG', domain: 'totheglory.im' },
  { key: 'u2', name: 'U2', domain: 'u2.dmhy.org' },
  { key: 'yema', name: 'YemaPT', domain: 'yemapt.org' },
  { key: 'zhuque', name: '朱雀（ZHUQUE）', domain: 'zhuque.in' },
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
