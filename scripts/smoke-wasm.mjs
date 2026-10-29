// Real reactor + in-memory Host Call stub. This is not a DIAN115 host acceptance test.
import { WASI } from 'node:wasi'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'
import assert from 'node:assert/strict'

const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const module = await WebAssembly.compile(readFileSync(join(root, 'build/runtime/plugin.wasm')))
const docs = new Map()
let revision = 0
let instance
let hostResponse
const bytes = () => new Uint8Array(instance.exports.memory.buffer)
const encode = value => Buffer.from(JSON.stringify(value))
const wasi = new WASI({ version: 'preview1', args: [], env: {}, preopens: {} })
const imports = {
  wasi_snapshot_preview1: wasi.wasiImport,
  dian115: {
    host_call(ptr, size) {
      const rpc = JSON.parse(Buffer.from(bytes().slice(ptr, ptr + size)).toString())
      if (rpc.method === 'host.capabilities') {
        hostResponse = encode({ result: { features: { http_sessions_v1: true, http_sites_v1: true, credential_fields_v1: true } } })
        return hostResponse.length
      }
      assert.equal(rpc.method, 'host.call')
      const { method, path, headers = {}, body_base64 } = rpc.params
      if (path === '/api/plugin-runtime/http/cookiecloud' && method === 'GET') {
        hostResponse = encode({ result: { status: 200, body_base64: encode({ data: { revision: '', domains: [] } }).toString('base64') } })
        return hostResponse.length
      }
      assert.ok(path.startsWith('/api/plugin-runtime/storage/'), 'unexpected non-storage side effect')
      let response
      if (method === 'GET') {
        const doc = docs.get(path)
        response = doc ? { status: 200, headers: { etag: [doc.etag] }, body_base64: encode({ data: { value: doc.value } }).toString('base64') } : { status: 404 }
      } else if (method === 'PUT') {
        const previous = docs.get(path)
        if (headers['if-match'] && previous?.etag !== headers['if-match']) response = { status: 412 }
        else {
          assert.ok(headers['idempotency-key'].length >= 16)
          const { value } = JSON.parse(Buffer.from(body_base64, 'base64').toString())
          const etag = `"pkv_${++revision}"`
          docs.set(path, { value, etag }); response = { status: 200, headers: { etag: [etag] } }
        }
      } else throw new Error(`unexpected method ${method}`)
      hostResponse = encode({ result: response })
      return hostResponse.length
    },
    host_read(ptr, capacity) {
      assert.ok(capacity >= hostResponse.length)
      bytes().set(hostResponse, ptr)
      return hostResponse.length
    },
  },
}
instance = await WebAssembly.instantiate(module, imports)
wasi.initialize(instance)
function rpc(method, params) {
  const request = encode({ method, params })
  const ptr = instance.exports.dian115_alloc(request.length)
  bytes().set(request, ptr)
  const result = instance.exports.dian115_handle(ptr, request.length)
  const offset = Number(result >> 32n), size = Number(result & 0xffffffffn)
  const response = JSON.parse(Buffer.from(bytes().slice(offset, offset + size)).toString())
  assert.equal(response.error, undefined, JSON.stringify(response.error))
  assert.ok(size <= 256 * 1024)
  return response.result
}
let invocation = 0
const invoke = (op, payload) => rpc('runtime.invoke', { envelope: { op, invocation_id: `smoke-${++invocation}`, payload } })
assert.equal(rpc('runtime.initialize', { protocol: 'dian115:wasm@1' }).ready, true)
const initial = invoke('state', {})
assert.equal(initial.state.live_execution, true)
assert.ok(Array.isArray(initial.state.presets) && initial.state.presets.length >= 160, 'preset library populated: ' + (initial.state.presets || []).length)
const presetGroups = new Set(initial.state.presets.map(p => p.group))
assert.ok(presetGroups.has('major') && presetGroups.has('nexus'), 'preset groups present')
assert.equal(invoke('state', { if_none_match: initial.etag }).not_modified, true)
const emptyConfig = {
  schema_version: 1, enabled: false, cron: '', notify: false, queue_cnt: 5,
  sign_sites: [], login_sites: [], retry_keyword: '', sites: [],
}
assert.equal(invoke('action', { id: 'save-config', input: { config: emptyConfig, revision: '' } }).status, 'succeeded')
assert.equal(invoke('action', { id: 'save-config', input: { config: emptyConfig, revision: '' } }).status, 'failed')
const saved = invoke('state', {})
assert.equal(saved.state.config.enabled, false)
assert.equal(invoke('action', { id: 'sign-now', input: { type: 'sign' } }).report.all, 0)
assert.equal(invoke('action', { id: 'clear-history', input: {} }).status, 'succeeded')
assert.equal(invoke('job', { id: 'unknown' }).status, 'skipped')
assert.equal(rpc('runtime.shutdown', {}).stopping, true)
console.log('Real WASM smoke: PASS (mock Broker; actual host/resident lifecycle not covered)')
