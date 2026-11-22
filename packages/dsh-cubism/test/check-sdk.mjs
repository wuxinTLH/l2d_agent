// Read-only compatibility probe: resolve SDK peers from an explicit installed DSH.
// Does not install dependencies, enable the plugin or touch the profile.
import { registerHooks, createRequire } from 'node:module'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import assert from 'node:assert/strict'

const root = process.argv[2]
if (!root) throw new Error('Usage: npm run check:sdk -- <installed-DSH-directory>')
const sdk = createRequire(pathToFileURL(resolve(root, 'package.json')))
const peers = new Map(['@deepseek-ai/dsh-tools', '@deepseek-ai/schemastery'].map(name => [name, pathToFileURL(sdk.resolve(name)).href]))
registerHooks({ resolve(specifier, context, nextResolve) {
  if (peers.has(specifier)) return { url: peers.get(specifier), shortCircuit: true }
  return nextResolve(specifier, context)
} })
const plugin = await import('../src/index.mjs')
assert.equal(plugin.name, 'cubism')
assert.deepEqual(plugin.inject, ['tools'])
const registered = []
const config = plugin.Config({})
assert.equal(config.studioUrl, 'http://127.0.0.1:17823')
assert.equal(config.timeoutMs, 30000)
const originalFetch = globalThis.fetch
globalThis.fetch = async () => new Response('{"status":"ok"}', { headers: { 'content-type': 'application/json' } })
try { plugin.apply({ tools: { register: definition => registered.push(definition) } }, config) }
finally { globalThis.fetch = originalFetch }
assert.equal(registered.length, 10)
assert.equal(new Set(registered.map(tool => tool.name)).size, 10)
for (const name of ['cubism_build_eye_rig', 'cubism_build_face_rig']) {
  assert.equal(registered.find(tool => tool.name === name).timeoutMs, 90_000)
  await assert.rejects(registered.find(tool => tool.name === name).execute({ recipe: 'override' }, {}), /takes no arguments/)
}
for (const tool of registered) {
  assert.equal(typeof tool.execute, 'function')
  assert.ok(tool.parameters)
  assert.ok(tool.output)
}
const health = registered.find(tool => tool.name === 'cubism_health')
assert.deepEqual(await health.execute({}, { signal: new AbortController().signal }), { status: 'ok' })
const command = registered.find(tool => tool.name === 'cubism_command')
await assert.rejects(command.execute({ method: 'Eval', data: {} }, { signal: new AbortController().signal }))
const dshVersion = sdk('./package.json').version
const toolsVersion = sdk('@deepseek-ai/dsh-tools/package.json').version
console.log(`PASS: ${registered.length} tools registered with real DSH ${dshVersion}, dsh-tools ${toolsVersion}; no profile activation or Editor calls.`)
