export interface Point { x: number; y: number }
export interface Rect extends Point { width: number; height: number }
export interface ViewTransform extends Point { scale: number }
export type ResizeHandle = 'n' | 'ne' | 'e' | 'se' | 's' | 'sw' | 'w' | 'nw'

export const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value))
export function imagePoint(point: Point, view: ViewTransform): Point {
  return { x: (point.x - view.x) / view.scale, y: (point.y - view.y) / view.scale }
}
export function screenPoint(point: Point, view: ViewTransform): Point {
  return { x: point.x * view.scale + view.x, y: point.y * view.scale + view.y }
}
export function zoomAt(view: ViewTransform, anchor: Point, scale: number): ViewTransform {
  const point = imagePoint(anchor, view)
  return { scale, x: anchor.x - point.x * scale, y: anchor.y - point.y * scale }
}
export function fitTransform(width: number, height: number, viewportWidth: number, viewportHeight: number, padding = 24): ViewTransform {
  const scale = Math.max(0.00001, Math.min(Math.max(1, viewportWidth - padding * 2) / width, Math.max(1, viewportHeight - padding * 2) / height))
  return { scale, x: (viewportWidth - width * scale) / 2, y: (viewportHeight - height * scale) / 2 }
}
export function boundedRect(rect: Rect, width: number, height: number): Rect {
  const x = clamp(Math.round(Number.isFinite(rect.x) ? rect.x : 0), 0, width - 1)
  const y = clamp(Math.round(Number.isFinite(rect.y) ? rect.y : 0), 0, height - 1)
  return { x, y, width: clamp(Math.round(Number.isFinite(rect.width) ? rect.width : 1), 1, width - x), height: clamp(Math.round(Number.isFinite(rect.height) ? rect.height : 1), 1, height - y) }
}
export function drawRect(start: Point, end: Point, width: number, height: number): Rect {
  const left = clamp(Math.round(Math.min(start.x, end.x)), 0, width - 1)
  const top = clamp(Math.round(Math.min(start.y, end.y)), 0, height - 1)
  return { x: left, y: top, width: clamp(Math.round(Math.max(start.x, end.x)) - left, 1, width - left), height: clamp(Math.round(Math.max(start.y, end.y)) - top, 1, height - top) }
}
export function moveRect(rect: Rect, delta: Point, width: number, height: number): Rect {
  const safe = boundedRect(rect, width, height)
  return { ...safe, x: clamp(Math.round(safe.x + delta.x), 0, width - safe.width), y: clamp(Math.round(safe.y + delta.y), 0, height - safe.height) }
}
export function resizeRect(rect: Rect, handle: ResizeHandle, delta: Point, width: number, height: number): Rect {
  const safe = boundedRect(rect, width, height)
  let { x, y } = safe
  let right = x + safe.width, bottom = y + safe.height
  if (handle.includes('w')) x = clamp(Math.round(x + delta.x), 0, right - 1)
  if (handle.includes('e')) right = clamp(Math.round(right + delta.x), x + 1, width)
  if (handle.includes('n')) y = clamp(Math.round(y + delta.y), 0, bottom - 1)
  if (handle.includes('s')) bottom = clamp(Math.round(bottom + delta.y), y + 1, height)
  return { x, y, width: right - x, height: bottom - y }
}
