import { mkdirSync } from 'node:fs'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

// 宿主只接受低占用工具链构建的模块（TinyGo），并拒绝链接 encoding/json 的
// 模块。构建前设置：TINYGO（tinygo 可执行文件）、WASMOPT（wasm-opt，可选，
// TinyGo 依赖它做 -opt=z）、GOROOT（TinyGo 从 GOROOT 源码编译标准库）。
const root = join(dirname(fileURLToPath(import.meta.url)), '..')
const output = join(root, 'build', 'runtime', 'plugin.wasm')
mkdirSync(dirname(output), { recursive: true })
const tinygo = process.env.TINYGO || 'tinygo'
const result = spawnSync(tinygo, ['build', '-target=wasi', '-buildmode=c-shared', '-opt=z', '-no-debug', '-o', output, './runtime'], {
  cwd: root,
  env: { ...process.env },
  encoding: 'utf8',
  stdio: 'inherit',
})

if (result.error) throw result.error
if (result.status !== 0) throw new Error(`TinyGo runtime build failed with exit code ${result.status}`)
process.stdout.write(`Built TinyGo WASM runtime: ${output}\n`)
