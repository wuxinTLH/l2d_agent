import test from 'node:test'
import assert from 'node:assert/strict'
import { boundedRect, drawRect, fitTransform, imagePoint, moveRect, resizeRect, screenPoint, zoomAt } from '../src/crop-geometry.ts'
import type { ResizeHandle } from '../src/crop-geometry.ts'

test('screen/image transforms round-trip after pan and zoom', () => {
  const view = { x: -45, y: 83, scale: 2.5 }
  const point = { x: 156, y: 42 }
  assert.deepEqual(imagePoint(screenPoint(point, view), view), point)
})
test('pointer anchored zoom preserves the image point under cursor', () => {
  const view = { x: -45, y: 83, scale: 0.35 }, anchor = { x: 240, y: 170 }
  const before = imagePoint(anchor, view)
  const after = imagePoint(anchor, zoomAt(view, anchor, 1.75))
  assert.ok(Math.abs(before.x - after.x) < 1e-10)
  assert.ok(Math.abs(before.y - after.y) < 1e-10)
})
test('fit centers the full image with padding for portrait and landscape', () => {
  for (const [width, height] of [[1000, 2000], [2000, 1000]]) {
    const view = fitTransform(width, height, 600, 400)
    assert.ok(view.x >= 24 && view.y >= 24)
    assert.ok(view.x + width * view.scale <= 576)
    assert.ok(view.y + height * view.scale <= 376)
    assert.equal(view.x * 2 + width * view.scale, 600)
    assert.equal(view.y * 2 + height * view.scale, 400)
  }
})
test('reverse rectangle drawing clamps and rounds to original image bounds', () => {
  assert.deepEqual(drawRect({ x: 90.4, y: 195 }, { x: -10, y: -30 }, 100, 200), { x: 0, y: 0, width: 90, height: 195 })
  assert.deepEqual(drawRect({ x: 100, y: 200 }, { x: 100, y: 200 }, 100, 200), { x: 99, y: 199, width: 1, height: 1 })
})
test('moving preserves size while clamping at every edge', () => {
  const rect = { x: 10, y: 20, width: 30, height: 40 }
  assert.deepEqual(moveRect(rect, { x: -100, y: 1000 }, 100, 200), { x: 0, y: 160, width: 30, height: 40 })
  assert.deepEqual(moveRect(rect, { x: 1000, y: -1000 }, 100, 200), { x: 70, y: 0, width: 30, height: 40 })
})
test('all eight resize handles keep positive integer dimensions and stay bounded', () => {
  const rect = { x: 10, y: 20, width: 30, height: 40 }
  const handles: ResizeHandle[] = ['n', 'ne', 'e', 'se', 's', 'sw', 'w', 'nw']
  for (const handle of handles) for (const sign of [-1, 1]) {
    const result = resizeRect(rect, handle, { x: sign * 1000.8, y: sign * 1000.1 }, 100, 200)
    assert.ok(Object.values(result).every(Number.isInteger))
    assert.ok(result.x >= 0 && result.y >= 0 && result.width >= 1 && result.height >= 1)
    assert.ok(result.x + result.width <= 100 && result.y + result.height <= 200)
    if (!handle.includes('w')) assert.equal(result.x, rect.x)
    if (!handle.includes('n')) assert.equal(result.y, rect.y)
    if (!handle.includes('e')) assert.equal(result.x + result.width, rect.x + rect.width)
    if (!handle.includes('s')) assert.equal(result.y + result.height, rect.y + rect.height)
  }
})
test('external malformed geometry is normalized for rendering', () => {
  assert.deepEqual(boundedRect({ x: NaN, y: -5, width: Infinity, height: 300 }, 100, 200), { x: 0, y: 0, width: 1, height: 200 })
})
