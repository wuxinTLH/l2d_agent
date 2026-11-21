import type { Asset } from './domain'

export interface PsdLayer { assetId:string; name:string; x:number; y:number; group?:string; width?:number; height?:number }
export interface PsdDraft { width:number; height:number; layers:PsdLayer[] }
export interface PsdExport { id:string; url:string; width:number; height:number; layerCount:number }
// Layers are top-to-bottom. Rank only new drafts, never a user's edited stack.
export function createPsdDraft(assets:Asset[]):PsdDraft {
  const sourceIds = new Set(assets.map(a=>a.sourcePlacement?.assetId).filter(Boolean))
  const rank = (asset:Asset):number => {
    if (sourceIds.has(asset.id)) return 4
    // A derived part can inherit the source's name; provenance takes precedence.
    if (!asset.sourcePlacement && /^generated[\s_-]+oc(?:$|[\s_(-])/i.test(asset.name.trim())) return 4
    const name = asset.name.toLowerCase()
    if (/\b(mouth|eyes?|eyebrows?|brows?)\b|嘴|口部|眼|眉/.test(name.replace(/[_-]/g,' '))) return 0
    if (/\bhair\b|头发|頭髮|发丝|髮|刘海|劉海/.test(name.replace(/[_-]/g,' '))) return 1
    if (/\b(head|body|torso|face)\b|头|頭|身体|身體|躯干|軀幹|脸|臉/.test(name.replace(/[_-]/g,' '))) return 2
    return 3
  }
  const ordered = assets.map((asset,index)=>({asset,index,rank:rank(asset)})).sort((a,b)=>a.rank-b.rank || a.index-b.index)
  return {width:Math.max(1,...assets.map(a=>a.sourcePlacement?.canvasWidth ?? a.width)),height:Math.max(1,...assets.map(a=>a.sourcePlacement?.canvasHeight ?? a.height)),layers:ordered.map(({asset:a})=>({assetId:a.id,name:a.name,x:a.sourcePlacement?.x ?? 0,y:a.sourcePlacement?.y ?? 0}))}
}
// Advisory only: legacy assets without provenance remain exportable. Callers can
// show this before export without treating guessed geometry as verified.
export function psdDraftWarnings(draft:PsdDraft, assets:Asset[]):string[] {
  const selected = new Set(draft.layers.map(layer=>layer.assetId))
  return [...assets.filter(a=>selected.has(a.id)&&a.processingWarning).map(a=>`「${a.name}」${a.processingWarning}`),...assets.filter(a=>selected.has(a.id) && a.sourcePlacement?.verifiedScale === false)
    .map(a=>`「${a.name}」尚未验证原图比例与位置，可能出现部件过大或错位；导出前请与原图核对。可手动调整输出宽高及坐标。`)]
}
export function validatePsdDraft(draft:PsdDraft, assets:Asset[]):string | undefined {
  if (!draft.layers.length || draft.layers.length > 64) return '请选择 1–64 个素材作为图层。'
  if (![draft.width,draft.height].every(n=>Number.isSafeInteger(n) && n > 0 && n <= 8192) || draft.width*draft.height > 40_000_000) return '画布宽高须为 1–8192 整数像素，且不超过 4000 万像素。'
  for (const layer of draft.layers) {
    if (!assets.some(a=>a.id === layer.assetId)) return '部分素材已不存在，请关闭并重新选择图层。'
    if ((layer.width ?? 0)!==0 || (layer.height ?? 0)!==0) { if (![layer.width,layer.height].every(n=>Number.isSafeInteger(n)&&Number(n)>0&&Number(n)<=8192) || Number(layer.width)*Number(layer.height)>40_000_000) return '图层宽高须同时为 1–8192 整数像素且不超过 4000 万像素。' }
    if (!layer.name.trim() || layer.name.trim().length > 100) return '图层名称须为 1–100 个字符。'
    if (![layer.x,layer.y].every(n=>Number.isSafeInteger(n) && Math.abs(n) <= 8192)) return '图层坐标必须为 -8192 至 8192 的整数像素。'
  }
}
// Download only the known server-local export route; never trust arbitrary response URLs.
export function psdDownloadUrl(result:PsdExport):string {
  if (!/^[a-zA-Z0-9_-]+$/.test(result.id) || result.url !== `/exports/${result.id}.psd`) throw new Error('服务返回了无效的 PSD 下载地址。')
  return result.url
}
export async function deleteSequential(ids:string[], remove:(id:string)=>Promise<unknown>) {
  const deleted:string[] = [], failed:{id:string;message:string}[] = []
  for (const id of ids) {
    try { await remove(id); deleted.push(id) }
    catch (error) { failed.push({id,message:error instanceof Error ? error.message : '删除失败'}) }
  }
  return {deleted,failed}
}
