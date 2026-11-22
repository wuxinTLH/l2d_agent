import { isIP } from 'node:net'

export const DEFAULT_STUDIO_URL = 'http://127.0.0.1:17823'
export const METHODS = Object.freeze(['GetDocuments', 'GetCurrentModelUID', 'GetParameters', 'GetParameterValues', 'SetParameterValues', 'ClearParameterValues'])
const MAX_BODY = 1024 * 1024

export function localURL(value, protocol, rootOnly = false) {
  let url
  try { url = new URL(value) } catch { throw new Error('Expected a loopback URL') }
  const host = url.hostname.replace(/^\[|\]$/g, '')
  const loopback = host === 'localhost' || host === '::1' || (isIP(host) === 4 && host.startsWith('127.'))
  if (!loopback || url.protocol !== protocol || !url.port || url.username || url.password || url.search || url.hash || (rootOnly && url.pathname !== '/')) {
    throw new Error(`Expected a loopback ${protocol} URL with explicit non-default port and no credentials, query or fragment${rootOnly ? ' or path' : ''}`)
  }
  return url
}

export function createClient({ studioUrl = DEFAULT_STUDIO_URL, timeoutMs = 30_000 } = {}, fetchImpl = fetch) {
  const base = localURL(studioUrl, 'http:', true)
  if (!Number.isInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > 600_000) throw new Error('timeoutMs must be 1..600000')
  return async function request(path, body, signal) {
    const combined = signal ? AbortSignal.any([signal, AbortSignal.timeout(timeoutMs)]) : AbortSignal.timeout(timeoutMs)
    let response
    try {
      response = await fetchImpl(new URL(path, base), {
        method: body === undefined ? 'GET' : 'POST',
        redirect: 'error', signal: combined,
        headers: { Accept: 'application/json', ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
        ...(body === undefined ? {} : { body: JSON.stringify(body) }),
      })
    } catch {
      if (combined.aborted) throw new Error('Studio request cancelled or timed out; a mutation may already have taken effect. Inspect before retrying.')
      throw new Error('Studio request failed; check localhost studio health. A mutation may already have taken effect; inspect before retrying.')
    }
    // Do not echo arbitrary HTTP bodies: proxy errors can contain credentials or HTML.
    if (!response.ok) {
      await response.body?.cancel()
      throw new Error(`Studio returned HTTP ${response.status}; inspect the studio UI/server. No automatic retry was attempted.`)
    }
    const type = response.headers.get('content-type') ?? ''
    if (!type.toLowerCase().includes('application/json')) {
      await response.body?.cancel()
      throw new Error('Studio returned non-JSON content')
    }
    const reader = response.body?.getReader()
    if (!reader) throw new Error('Studio returned an empty response')
    const chunks = []
    let total = 0
    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        total += value.byteLength
        if (total > MAX_BODY) throw new Error('Studio JSON response exceeds 1 MiB')
        chunks.push(value)
      }
    } catch (error) {
      await reader.cancel().catch(() => {})
      if (combined.aborted) throw new Error('Studio response cancelled or timed out; inspect mutation state before retrying')
      throw error
    } finally { reader.releaseLock() }
    let value
    try { value = JSON.parse(Buffer.concat(chunks).toString('utf8')) } catch { throw new Error('Studio returned invalid JSON') }
    if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error('Studio returned a non-object JSON response')
    return value
  }
}

const object = value => value !== null && typeof value === 'object' && !Array.isArray(value)
function fields(value, allowed, label) {
  if (!object(value) || Object.keys(value).some(key => !allowed.includes(key))) throw new Error(`Invalid ${label} fields`)
}
function uid(value) {
  if (typeof value !== 'string' || !value.trim() || Buffer.byteLength(value) > 1024 || /[\0\r\n]/.test(value)) throw new Error('Invalid Cubism UID or parameter Id')
}
export function commandBody(method, data = {}) {
  if (!METHODS.includes(method)) throw new Error('Unsupported Cubism method; no arbitrary evaluation or mesh editing is available')
  const allowed = {
    GetDocuments: [], GetCurrentModelUID: [], GetParameters: ['ModelUID', 'DocumentUID'],
    GetParameterValues: ['ModelUID', 'Ids'], SetParameterValues: ['ModelUID', 'Parameters'], ClearParameterValues: ['ModelUID'],
  }[method]
  fields(data, allowed, 'command data')
  for (const key of ['ModelUID', 'DocumentUID']) if (data[key] !== undefined) uid(data[key])
  if (method === 'GetParameters' && !data.ModelUID && !data.DocumentUID) throw new Error('GetParameters requires ModelUID or DocumentUID')
  if (['GetParameterValues', 'SetParameterValues', 'ClearParameterValues'].includes(method)) uid(data.ModelUID)
  if (data.Ids !== undefined) {
    if (!Array.isArray(data.Ids) || data.Ids.length > 1024) throw new Error('Ids must contain at most 1024 entries')
    data.Ids.forEach(uid)
  }
  if (method === 'SetParameterValues') {
    if (!Array.isArray(data.Parameters) || data.Parameters.length < 1 || data.Parameters.length > 1024) throw new Error('Parameters must contain 1..1024 entries')
    const ids = new Set()
    for (const parameter of data.Parameters) {
      fields(parameter, ['Id', 'Value'], 'parameter'); uid(parameter.Id)
      if (ids.has(parameter.Id) || !Number.isFinite(parameter.Value) || Math.abs(parameter.Value) > 1e6) throw new Error('Parameters require unique Ids and finite numeric Values within +/-1000000')
      ids.add(parameter.Id)
    }
  }
  if (Buffer.byteLength(JSON.stringify(data)) > 256 * 1024) throw new Error('Command data exceeds 256 KiB')
  return { method, data }
}

export function psdBody(args) {
  fields(args, ['width', 'height', 'layers'], 'PSD request')
  const size = (w, h) => Number.isInteger(w) && Number.isInteger(h) && w >= 1 && h >= 1 && w <= 8192 && h <= 8192 && w * h <= 40_000_000
  if (!size(args.width, args.height)) throw new Error('PSD canvas must be 1..8192 per axis and at most 40 megapixels')
  if (!Array.isArray(args.layers) || args.layers.length < 1 || args.layers.length > 64) throw new Error('PSD requires 1..64 layers')
  for (const layer of args.layers) {
    fields(layer, ['assetId', 'name', 'group', 'x', 'y', 'width', 'height'], 'PSD layer')
    if (typeof layer.assetId !== 'string' || !/^[a-f0-9]{24}$/.test(layer.assetId)) throw new Error('Invalid assetId')
    for (const key of ['name', 'group']) if (layer[key] !== undefined && (typeof layer[key] !== 'string' || Buffer.byteLength(layer[key]) > 4096 || layer[key].includes('\0'))) throw new Error('Invalid PSD layer name/group')
    for (const key of ['x', 'y']) if (!Number.isInteger(layer[key] ?? 0) || Math.abs(layer[key] ?? 0) > 8192) throw new Error('PSD layer offsets must be integers within +/-8192')
    if (((layer.width ?? 0) !== 0 || (layer.height ?? 0) !== 0) && !size(layer.width, layer.height)) throw new Error('PSD layer resize requires both positive dimensions within canvas limits')
  }
  return args
}
