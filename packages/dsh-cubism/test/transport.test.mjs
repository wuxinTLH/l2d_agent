import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { once } from 'node:events'
import { createClient, localURL, commandBody, psdBody, METHODS } from '../src/transport.mjs'
import { createTools } from '../src/tools.mjs'

const json = value => new Response(JSON.stringify(value), { headers: { 'content-type': 'application/json' } })
const id = 'a'.repeat(24)

test('only credential-free explicit-port loopback origins; prevent redirect SSRF', () => {
  for (const url of ['http://127.0.0.1:17823', 'http://localhost:1234', 'http://[::1]:1234']) assert.ok(localURL(url, 'http:', true))
  for (const url of ['http://example.com:1234', 'https://localhost:1234', 'http://localhost:1234/path', 'http://user:secret@localhost:1234', 'http://localhost:1234/?key=secret', 'http://localhost']) assert.throws(() => createClient({ studioUrl: url }))
  assert.throws(() => localURL('ws://remote:22033', 'ws:'))
})

test('the eight studio tools issue exact routes/bodies and preserve truthful response', async () => {
  const calls = []
  const signal = new AbortController().signal
  const tools = createTools({}, async (url, options) => { calls.push([url.pathname, options]); return json({ connected: false, approved: false, note: 'not ready' }) })
    .filter(tool => !['cubism_build_eye_rig', 'cubism_build_face_rig'].includes(tool.name))
  assert.equal(tools.length, 8)
  const args = [{}, {}, {}, {}, { method: 'GetDocuments' }, {}, { width: 32, height: 32, layers: [{ assetId: id, name: 'eye', group: 'face', x: -1, y: 2 }] }, { exportId: id }]
  const paths = ['/api/health', '/api/l2d/cubism/discover', '/api/l2d/cubism/launch', '/api/l2d/cubism/connect', '/api/l2d/cubism/command', '/api/l2d/cubism/disconnect', '/api/oc/export-psd', '/api/l2d/cubism/open-psd']
  for (const [i, tool] of tools.entries()) {
    const out = await tool.execute(args[i], { signal })
    assert.equal(out.approved, false)
    assert.equal(calls[i][0], paths[i])
    assert.equal(calls[i][1].method, i === 0 ? 'GET' : 'POST')
    assert.equal(calls[i][1].redirect, 'error')
    assert.ok(calls[i][1].signal instanceof AbortSignal)
    assert.equal(tool.output.render(args[i], out)[0].type, 'text')
  }
  assert.equal(calls[0][1].body, undefined)
  assert.deepEqual(JSON.parse(calls[4][1].body), { method: 'GetDocuments', data: {} })
  assert.deepEqual(JSON.parse(calls[6][1].body), args[6])
  assert.deepEqual(JSON.parse(calls[7][1].body), args[7])
})

test('command allowlist and method-specific fields block arbitrary execution', () => {
  assert.equal(METHODS.length, 6)
  assert.deepEqual(commandBody('GetParameters', { DocumentUID: 'doc' }).data, { DocumentUID: 'doc' })
  assert.doesNotThrow(() => commandBody('GetParameterValues', { ModelUID: 'model', Ids: ['ParamAngleX'] }))
  assert.doesNotThrow(() => commandBody('ClearParameterValues', { ModelUID: 'model' }))
  assert.doesNotThrow(() => commandBody('SetParameterValues', { ModelUID: 'model', Parameters: [{ Id: 'ParamAngleX', Value: 0.5 }] }))
  for (const [method, data] of [['Eval', {}], ['GetDocuments', { token: 'hidden' }], ['GetParameters', {}], ['GetParameterValues', {}], ['SetParameterValues', { ModelUID: 'm', Parameters: [{ Id: 'p', Value: Infinity }] }], ['SetParameterValues', { ModelUID: 'm', Parameters: [{ Id: 'p', Value: 1 }, { Id: 'p', Value: 2 }] }], ['ClearParameterValues', { ModelUID: 'm\n' }]]) assert.throws(() => commandBody(method, data))
})

test('PSD validates geometry, IDs, resize pairs and unknown layer keys', () => {
  const valid = { width: 512, height: 512, layers: [{ assetId: id }] }
  assert.equal(psdBody(valid), valid)
  for (const args of [{ ...valid, width: 8193 }, { ...valid, width: 8192, height: 8192 }, { ...valid, layers: [] }, { ...valid, layers: [{ assetId: '../secret' }] }, { ...valid, layers: [{ assetId: id, x: 8193 }] }, { ...valid, layers: [{ assetId: id, width: 32 }] }, { ...valid, layers: [{ assetId: id, drawOrder: 1 }] }]) assert.throws(() => psdBody(args))
})

test('transport fails closed on HTTP, non-JSON, malformed and oversized bodies; no retries or error-body leakage', async () => {
  let count = 0
  const request = createClient({}, async () => { count++; return new Response('secret=do-not-print', { status: 400 }) })
  await assert.rejects(request('/api/health'), error => error.message.includes('HTTP 400') && !error.message.includes('do-not-print'))
  assert.equal(count, 1)
  for (const response of [new Response('<html/>'), new Response('{bad', { headers: { 'content-type': 'application/json' } }), json(null), json('x'), json({ big: 'x'.repeat(1024 * 1024) })]) {
    await assert.rejects(createClient({}, async () => response)('/api/health'))
  }
})

test('real localhost HTTP transport sends JSON and refuses redirects', async t => {
  const server = createServer((req, res) => {
    if (req.url === '/redirect') { res.writeHead(302, { location: 'http://example.com/' }); res.end(); return }
    let body = ''
    req.on('data', chunk => { body += chunk })
    req.on('end', () => { res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify({ method: req.method, body })) })
  })
  server.listen(0, '127.0.0.1'); await once(server, 'listening')
  t.after(() => { server.closeAllConnections(); server.close() })
  const request = createClient({ studioUrl: `http://127.0.0.1:${server.address().port}` })
  assert.deepEqual(await request('/api/test', { hello: 'world' }), { method: 'POST', body: '{"hello":"world"}' })
  await assert.rejects(request('/redirect'), /Studio request failed/)
})

test('caller cancellation and timeout reach transport; never retry ambiguous mutations', async () => {
  const abort = new AbortController(); abort.abort()
  await assert.rejects(createClient({}, async (_url, options) => { options.signal.throwIfAborted() })('/api/test', {}, abort.signal), /mutation may already have taken effect/)
  await assert.rejects(createClient({ timeoutMs: 5 }, async (_url, options) => {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(resolve, 100)
      options.signal.addEventListener('abort', () => { clearTimeout(timer); reject(new Error('aborted')) }, { once: true })
    })
  })('/api/test', {}), /timed out/)
})
