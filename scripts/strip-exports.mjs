// strip-extra-exports: 把 TinyGo c-shared 构建产物的导出集合收敛为宿主契约
// 精确允许的 [memory, _initialize, dian115_alloc, dian115_handle]。
// 宿主 5.x 实测（社区插件 douban115）只接受这一导出集合；TinyGo 默认 asyncify
// 调度器会额外导出 malloc/free/calloc/realloc/fmin*/fmax*/asyncify_*，
// 宿主按 ABI 白名单把多余导出判定为 plugin_incompatible。
// 只删除导出条目（名字→函数索引映射），不改动函数体与内部调用，wazero 行为不变。
import { readFileSync, writeFileSync } from 'node:fs'

const ALLOWED = new Set(['memory', '_initialize', 'dian115_alloc', 'dian115_handle'])

function lebDecode(buf, off) {
  let result = 0
  let shift = 0
  while (true) {
    const byte = buf[off]
    off += 1
    result |= (byte & 0x7f) << shift
    if ((byte & 0x80) === 0) break
    shift += 7
  }
  return [result >>> 0, off]
}

function lebEncode(value) {
  const out = []
  let v = value >>> 0
  do {
    let byte = v & 0x7f
    v >>>= 7
    if (v !== 0) byte |= 0x80
    out.push(byte)
  } while (v !== 0)
  return out
}

function encodeString(s) {
  const bytes = Buffer.from(s, 'latin1')
  return Buffer.concat([Buffer.from(lebEncode(bytes.length)), bytes])
}

function parseSections(wasm) {
  const sections = []
  let off = 8
  while (off < wasm.length) {
    const id = wasm[off]
    off += 1
    const [size, payloadStart] = lebDecode(wasm, off)
    off = payloadStart + size
    sections.push({ id, payload: wasm.subarray(payloadStart, off) })
  }
  return sections
}

function stripExtraExports(wasm) {
  const header = wasm.subarray(0, 8)
  const sections = parseSections(wasm)
  let removed = 0
  const rebuilt = [header]
  for (const { id, payload } of sections) {
    if (id === 7) {
      const [count, p0] = lebDecode(payload, 0)
      let p = p0
      const entries = []
      for (let i = 0; i < count; i++) {
        const [nameLen, a] = lebDecode(payload, p)
        const name = payload.subarray(a, a + nameLen).toString('latin1')
        p = a + nameLen
        const kind = payload[p]
        p += 1
        const [idx, b] = lebDecode(payload, p)
        p = b
        if (ALLOWED.has(name)) {
          entries.push({ name, kind, idx })
        } else {
          removed += 1
        }
      }
      const newPayload = Buffer.concat([
        Buffer.from(lebEncode(entries.length)),
        ...entries.map((e) => Buffer.concat([encodeString(e.name), Buffer.from([e.kind]), Buffer.from(lebEncode(e.idx))])),
      ])
      rebuilt.push(Buffer.concat([Buffer.from([7]), Buffer.from(lebEncode(newPayload.length)), newPayload]))
    } else {
      rebuilt.push(Buffer.concat([Buffer.from([id]), Buffer.from(lebEncode(payload.length)), payload]))
    }
  }
  return { wasm: Buffer.concat(rebuilt), removed }
}

export function stripExportsFromFile(inputPath, outputPath) {
  const wasm = readFileSync(inputPath)
  const { wasm: stripped, removed } = stripExtraExports(wasm)
  if (removed === 0 && Buffer.compare(stripped, wasm) === 0) {
    writeFileSync(outputPath, wasm)
  } else {
    writeFileSync(outputPath, stripped)
  }
  return removed
}

if (import.meta.url === `file://${process.argv[1].replace(/\\/g, '/')}`) {
  const [input, output] = process.argv.slice(2)
  const removed = stripExportsFromFile(input, output || input)
  process.stdout.write(`stripped ${removed} extra export(s)\n`)
}
