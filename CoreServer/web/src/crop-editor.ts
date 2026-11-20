import type { Part } from './domain'
import { assetUrl } from './domain'
import { boundedRect, clamp, drawRect, fitTransform, imagePoint, moveRect, resizeRect, zoomAt } from './crop-geometry'
import type { Point, ResizeHandle, ViewTransform } from './crop-geometry'
import './crop-editor.scss'

export interface CropEditorOptions {
  asset: { id: string; name: string; url: string; width: number; height: number }
  parts: Part[]
  activeIndex: number
  onChange: (parts: Part[], activeIndex: number) => void
}
export interface CropEditor { update(parts: Part[], activeIndex: number): void; destroy(): void }
type Snapshot = { parts: Part[]; activeIndex: number }
type Gesture = { pointer: number; kind: 'pan' | 'draw' | 'move' | 'resize'; start: Point; view: ViewTransform; before: Snapshot; index: number; handle?: ResizeHandle }
const handles: ResizeHandle[] = ['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']
const copyParts = (parts: Part[]) => parts.map(part => ({ ...part }))
const sameParts = (a: Part[], b: Part[]) => JSON.stringify(a) === JSON.stringify(b)

/** Mount once per asset. update() synchronizes external list edits without replacing the viewport. */
export function mountCropEditor(host: HTMLElement, options: CropEditorOptions): CropEditor {
  const { asset } = options
  if (!(Number.isInteger(asset.width) && asset.width > 0 && Number.isInteger(asset.height) && asset.height > 0)) throw new Error('裁切图片尺寸无效。')
  const url = assetUrl(asset.url)
  const controller = new AbortController()
  const { signal } = controller
  const root = document.createElement('div')
  root.className = 'ce-editor'
  root.tabIndex = 0
  root.setAttribute('role', 'region')
  root.setAttribute('aria-label', `裁切编辑器：${asset.name}`)
  const toolbar = document.createElement('div')
  toolbar.className = 'ce-toolbar'
  toolbar.setAttribute('role', 'toolbar')
  toolbar.setAttribute('aria-label', '裁切工具')
  const viewport = document.createElement('div')
  viewport.className = 'ce-viewport'
  viewport.tabIndex = 0
  viewport.setAttribute('aria-label', '图片裁切画布，可滚轮缩放、空格拖动平移')
  const image = document.createElement('img')
  image.className = 'ce-source'
  image.alt = asset.name
  image.draggable = false
  const overlay = document.createElement('div')
  overlay.className = 'ce-overlay'
  const hint = document.createElement('div')
  hint.className = 'ce-hint'
  hint.textContent = '滚轮缩放 · Ctrl / 空格 / 中键拖动平移 · 选择后拖动边角调整 · Delete 删除'
  const status = document.createElement('span')
  status.className = 'ce-status'
  status.setAttribute('role', 'status')
  viewport.append(image, overlay)
  root.append(toolbar, viewport, hint)
  host.replaceChildren(root)
  host.classList.add('ce-host')

  const normalize = (items: Part[]) => items.map(p => ({ ...p, ...boundedRect(p, asset.width, asset.height) }))
  let parts = normalize(options.parts)
  let activeIndex = validIndex(options.activeIndex)
  let view: ViewTransform = { x: 0, y: 0, scale: 1 }
  let tool: 'select' | 'draw' | 'pan' = 'draw'
  let space = false, destroyed = false, fitted = false, fitMode = true
  let gesture: Gesture | undefined
  const undo: Snapshot[] = [], redo: Snapshot[] = []
  function validIndex(index: number) { return Number.isInteger(index) && index >= 0 && index < parts.length ? index : -1 }
  const snapshot = (): Snapshot => ({ parts: copyParts(parts), activeIndex })
  function emit() { options.onChange(copyParts(parts), activeIndex) }
  function remember(before: Snapshot) {
    if (sameParts(before.parts, parts)) return
    undo.push(before)
    if (undo.length > 100) undo.shift()
    redo.length = 0
  }
  function button(label: string, title: string, action: () => void) {
    const el = document.createElement('button')
    el.type = 'button'; el.textContent = label; el.title = title; el.setAttribute('aria-label', title)
    el.addEventListener('click', () => { if (!gesture) action() }, { signal })
    toolbar.append(el)
    return el
  }
  const selectButton = button('选择', '选择 / 移动矩形 (V)', () => setTool('select'))
  const drawButton = button('矩形', '绘制矩形 (R)', () => setTool('draw'))
  const panButton = button('平移', '平移画布 (H)', () => setTool('pan'))
  button('−', '缩小', () => zoom(view.scale / 1.25))
  button('+', '放大', () => zoom(view.scale * 1.25))
  button('适应', '适应画布', fit)
  button('100%', '原始尺寸 100%', () => zoom(1))
  const undoButton = button('撤销', '撤销 (Ctrl / ⌘ Z)', () => history(false))
  const redoButton = button('重做', '重做 (Ctrl Y / ⌘ Shift Z)', () => history(true))
  const deleteButton = button('删除', '删除选中的矩形', remove)
  toolbar.append(status)
  function setTool(next: typeof tool) { tool = next; render() }
  function size() { return { width: viewport.clientWidth, height: viewport.clientHeight } }
  function fit() {
    const { width, height } = size()
    if (!width || !height || gesture) return
    view = fitTransform(asset.width, asset.height, width, height)
    fitted = true; fitMode = true; render()
  }
  function zoom(scale: number, anchor?: Point) {
    if (gesture) return
    const { width, height } = size()
    view = zoomAt(view, anchor ?? { x: width / 2, y: height / 2 }, clamp(scale, Math.min(0.01, view.scale), 32))
    fitMode = false; render()
  }
  function history(forward: boolean) {
    if (gesture) return
    const source = forward ? redo : undo, destination = forward ? undo : redo
    const state = source.pop()
    if (!state) return
    destination.push(snapshot()); parts = copyParts(state.parts); activeIndex = state.activeIndex
    render(); emit()
  }
  function remove() {
    if (activeIndex < 0 || gesture) return
    const before = snapshot()
    parts.splice(activeIndex, 1); activeIndex = Math.min(activeIndex, parts.length - 1)
    remember(before); render(); emit()
  }
  function render() {
    if (destroyed) return
    image.style.width = `${asset.width}px`; image.style.height = `${asset.height}px`
    image.style.transform = `translate(${view.x}px, ${view.y}px) scale(${view.scale})`
    const fragment = document.createDocumentFragment()
    parts.forEach((part, index) => {
      const rect = document.createElement('div')
      rect.className = `ce-rect${index === activeIndex ? ' ce-selected' : ''}`
      rect.dataset.index = String(index)
      rect.style.cssText = `left:${view.x + part.x * view.scale}px;top:${view.y + part.y * view.scale}px;width:${part.width * view.scale}px;height:${part.height * view.scale}px`
      const label = document.createElement('span')
      label.className = 'ce-rect-label'; label.textContent = `${index + 1} · ${part.name}`
      rect.append(label)
      if (index === activeIndex) handles.forEach(handle => {
        const node = document.createElement('span')
        node.className = `ce-handle ce-handle-${handle}`; node.dataset.handle = handle
        rect.append(node)
      })
      fragment.append(rect)
    })
    overlay.replaceChildren(fragment)
    viewport.dataset.tool = gesture?.kind === 'pan' || space ? 'pan' : tool
    viewport.classList.toggle('ce-dragging', !!gesture)
    ;[[selectButton, 'select'], [drawButton, 'draw'], [panButton, 'pan']].forEach(([button, name]) => (button as HTMLButtonElement).setAttribute('aria-pressed', String(tool === name)))
    undoButton.disabled = undo.length === 0 || !!gesture; redoButton.disabled = redo.length === 0 || !!gesture
    deleteButton.disabled = activeIndex < 0 || !!gesture
    status.textContent = `${Math.round(view.scale * 100)}% · ${parts.length} 个部件`
  }
  function local(event: PointerEvent | WheelEvent): Point {
    const bounds = viewport.getBoundingClientRect()
    return { x: event.clientX - bounds.left - viewport.clientLeft, y: event.clientY - bounds.top - viewport.clientTop }
  }
  function finish(cancel = false) {
    const current = gesture
    if (!current) return
    gesture = undefined
    if (cancel) { parts = copyParts(current.before.parts); activeIndex = current.before.activeIndex; view = current.view }
    else if (current.kind !== 'pan') remember(current.before)
    if (viewport.hasPointerCapture(current.pointer)) viewport.releasePointerCapture(current.pointer)
    render()
    if (current.kind !== 'pan') emit()
  }
  viewport.addEventListener('pointerdown', event => {
    if (gesture || (event.button !== 0 && event.button !== 1)) return
    event.preventDefault(); viewport.focus({ preventScroll: true })
    const start = local(event), before = snapshot()
    const target = event.target as HTMLElement
    const rect = target.closest<HTMLElement>('.ce-rect')
    const index = rect ? Number(rect.dataset.index) : -1
    let kind: Gesture['kind']
    let handle: ResizeHandle | undefined
    if (event.button === 1 || event.ctrlKey || space || tool === 'pan') { kind = 'pan'; fitMode = false }
    else if (index >= 0) {
      activeIndex = index; handle = target.dataset.handle as ResizeHandle | undefined
      kind = handle ? 'resize' : 'move'
    } else if (tool === 'draw') {
      const point = imagePoint(start, view)
      if (point.x < 0 || point.y < 0 || point.x > asset.width || point.y > asset.height) return
      let number = parts.length + 1
      while (parts.some(p => p.name === `部件 ${number}`)) number++
      activeIndex = parts.length
      parts.push({ name: `部件 ${number}`, ...drawRect(point, point, asset.width, asset.height) })
      kind = 'draw'
    } else { activeIndex = -1; render(); emit(); return }
    gesture = { pointer: event.pointerId, kind, start, before, view: { ...view }, index: activeIndex, handle }
    viewport.setPointerCapture(event.pointerId)
    render()
    if (kind !== 'pan') emit()
  }, { signal })
  viewport.addEventListener('pointermove', event => {
    const current = gesture
    if (!current || current.pointer !== event.pointerId) return
    const point = local(event)
    const delta = { x: (point.x - current.start.x) / current.view.scale, y: (point.y - current.start.y) / current.view.scale }
    if (current.kind === 'pan') view = { ...view, x: current.view.x + point.x - current.start.x, y: current.view.y + point.y - current.start.y }
    else {
      const part = parts[current.index]
      if (!part) return
      const original = current.before.parts[current.index]
      const geometry = current.kind === 'draw' ? drawRect(imagePoint(current.start, current.view), imagePoint(point, current.view), asset.width, asset.height)
        : current.kind === 'resize' ? resizeRect(original, current.handle!, delta, asset.width, asset.height)
        : moveRect(original, delta, asset.width, asset.height)
      parts[current.index] = { ...part, ...geometry }
    }
    render()
    if (current.kind !== 'pan') emit()
  }, { signal })
  viewport.addEventListener('pointerup', event => { if (gesture?.pointer === event.pointerId) finish() }, { signal })
  viewport.addEventListener('pointercancel', event => { if (gesture?.pointer === event.pointerId) finish(true) }, { signal })
  viewport.addEventListener('lostpointercapture', event => { if (gesture?.pointer === event.pointerId) finish(true) }, { signal })
  viewport.addEventListener('wheel', event => { event.preventDefault(); zoom(view.scale * Math.exp(-clamp(event.deltaY * (event.deltaMode === 1 ? 16 : event.deltaMode === 2 ? viewport.clientHeight : 1), -400, 400) * 0.002), local(event)) }, { signal, passive: false })
  viewport.addEventListener('auxclick', event => { if (event.button === 1) event.preventDefault() }, { signal })
  function editable(target: EventTarget | null) { return target instanceof HTMLElement && !!target.closest('input, textarea, select, [contenteditable]:not([contenteditable="false"])') }
  root.addEventListener('keydown', event => {
    if (editable(event.target)) return
    const key = event.key.toLowerCase()
    if (key === 'escape' && gesture) { event.preventDefault(); finish(true); return }
    if (event.code === 'Space') { event.preventDefault(); space = true; render(); return }
    if (gesture) return
    if ((event.ctrlKey || event.metaKey) && !event.altKey && (key === 'z' || key === 'y')) {
      event.preventDefault(); history(key === 'y' || event.shiftKey); return
    }
    if (event.ctrlKey || event.metaKey || event.altKey) return
    if (key === 'delete' || key === 'backspace') { event.preventDefault(); remove() }
    else if (key === 'v') setTool('select')
    else if (key === 'r') setTool('draw')
    else if (key === 'h') setTool('pan')
    else if (key === '+' || key === '=') { event.preventDefault(); zoom(view.scale * 1.25) }
    else if (key === '-') { event.preventDefault(); zoom(view.scale / 1.25) }
    else if (key === '0') fit()
  }, { signal })
  root.addEventListener('keyup', event => { if (event.code === 'Space') { space = false; render() } }, { signal })
  root.addEventListener('focusout', event => { if (!root.contains(event.relatedTarget as Node | null)) { space = false; render() } }, { signal })
  window.addEventListener('blur', () => { space = false; finish(true); render() }, { signal })
  image.addEventListener('load', fit, { signal })
  image.addEventListener('error', () => { hint.textContent = '图片加载失败，请检查素材地址或重新选择素材。' }, { signal })
  const observer = new ResizeObserver(() => { if (!gesture && (!fitted || fitMode)) fit() })
  observer.observe(viewport)
  image.src = url
  fit(); render()
  return {
    update(nextParts, nextIndex) {
      if (destroyed || gesture) return
      const normalized = normalize(nextParts)
      if (!sameParts(parts, normalized)) {
        const before = snapshot(); parts = normalized; remember(before)
      }
      activeIndex = validIndex(nextIndex)
      render()
    },
    destroy() {
      if (destroyed) return
      // Do not publish state while an owner is switching assets/unmounting.
      const pointer = gesture?.pointer
      gesture = undefined; destroyed = true
      if (pointer !== undefined && viewport.hasPointerCapture(pointer)) viewport.releasePointerCapture(pointer)
      controller.abort(); observer.disconnect(); root.remove(); host.classList.remove('ce-host')
    },
  }
}
