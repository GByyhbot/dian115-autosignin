import { readFileSync, statSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

// 复刻宿主 project-check 的 WASM 门槛：
//  - ABI：memory / dian115_alloc / dian115_handle 导出，host_call / host_read 导入
//  - 必须是 reactor（无 _start）
//  - 不得链接 encoding/json（宿主点名拒绝并保留编译结果）
//  - 模块体积上限 2 MiB（TinyGo -opt=z 约 0.6 MiB）
const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const wasmPath = join(root, 'build/runtime/plugin.wasm')
const module = new WebAssembly.Module(readFileSync(wasmPath))
const exports = WebAssembly.Module.exports(module).map(item => item.name)
// 宿主 5.x 实测（社区 douban115 工作插件）只接受这一精确导出集合；
// TinyGo c-shared 附带的 malloc/free/asyncify 等多余导出会被判 plugin_incompatible。
const ALLOWED_EXPORTS = new Set(['memory', '_initialize', 'dian115_alloc', 'dian115_handle'])
const extra = exports.filter(name => !ALLOWED_EXPORTS.has(name))
if (extra.length > 0) throw new Error(`Runtime exports non-contract symbols ${extra.join(', ')}; run scripts/build-runtime.mjs (strips extras)`)
const missing = [...ALLOWED_EXPORTS].filter(name => !exports.includes(name))
if (missing.length > 0) throw new Error(`Missing contract export: ${missing.join(', ')}`)
if (exports.includes('_start')) throw new Error('Runtime must be a reactor, not a command')
const imports = WebAssembly.Module.imports(module)
for (const name of ['host_call', 'host_read']) {
 if (!imports.some(item => item.module === 'dian115' && item.name === name)) throw new Error(`Missing broker import: ${name}`)
}

// 扫描链接标记（与宿主 project-check 相同的检测方式）。
const marker = readFileSync(wasmPath, 'latin1')
if (marker.includes('encoding/json')) {
 throw new Error('Runtime links encoding/json; use the pluginjson helper (host build requirement)')
}
if (marker.includes('go:buildid') && marker.includes('runtime.gc')) {
 throw new Error('Runtime was built with the Go standard compiler; the host only accepts TinyGo-style low-overhead toolchains')
}

const bytes = statSync(wasmPath).size
const cap = 2 << 20
if (bytes > cap) {
 throw new Error(`Runtime exceeds the 2 MiB module cap: ${bytes} bytes`)
}
console.log(`WASM reactor ABI: PASS (${bytes} bytes, under 2 MiB cap)`)
