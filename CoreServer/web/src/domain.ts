export interface Asset { id: string; name: string; url: string; width: number; height: number; createdAt: string; processingWarning?:string; sourcePlacement?: {assetId?:string;x:number;y:number;canvasWidth:number;canvasHeight:number;verifiedScale:boolean} }
export interface Part { name: string; x: number; y: number; width: number; height: number }
export function validateParts(parts: Part[], width: number, height: number): string | undefined {
  if (!parts.length) return '请先添加至少一个裁切部件。'
  const names = new Set<string>()
  for (const p of parts) {
    if (!p.name.trim()) return '部件名称不能为空。'
    if (names.has(p.name.trim())) return '部件名称不能重复。'
    names.add(p.name.trim())
    if (![p.x, p.y, p.width, p.height].every(Number.isInteger)) return '裁切坐标必须为整数像素。'
    if (p.x < 0 || p.y < 0 || p.width < 1 || p.height < 1 || p.x + p.width > width || p.y + p.height > height) return `「${p.name}」超出原图边界或尺寸无效。`
  }
}
export function pixelRect(start: {x:number;y:number}, end: {x:number;y:number}, width: number, height: number): Omit<Part, 'name'> {
  const clamp = (v:number, max:number) => Math.max(0, Math.min(Math.round(v), max))
  const x = Math.min(clamp(start.x, width - 1), clamp(end.x, width - 1))
  const y = Math.min(clamp(start.y, height - 1), clamp(end.y, height - 1))
  return { x, y, width: Math.max(1, Math.min(width - x, Math.abs(clamp(end.x, width) - clamp(start.x, width)))), height: Math.max(1, Math.min(height - y, Math.abs(clamp(end.y, height) - clamp(start.y, height)))) }
}
export function assetUrl(value: string): string {
  const url = new URL(value, window.location.origin)
  if (!['http:', 'https:'].includes(url.protocol)) throw new Error('服务器返回了不支持的资源 URL。')
  return url.href
}
