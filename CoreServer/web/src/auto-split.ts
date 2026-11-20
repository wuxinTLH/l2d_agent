import type { Asset } from './domain'
import { assetUrl } from './domain'
import './auto-split.scss'

export interface AutoSplitResponse {
  id: string
  parts: Asset[]
  failures: { name: string; error: string }[]
  manifestUrl: string
  note: string
}

export interface AutoSplitDeps {
  getSelected: () => Asset | undefined
  post: <T>(path: string, body: unknown) => Promise<T>
  notify: (s: string, error?: boolean) => void
  onComplete: (result: AutoSplitResponse, source: Asset) => void
}

const defaultParts = ['左眼睛', '右眼睛', '头（不含眼睛及嘴及耳朵及头发）', '左手', '右手', '嘴', '身体（不含头及手及头发）', '左耳朵', '右耳朵', '头发']

/** Mount once; call update() after the parent changes the selected source. Never selects outputs. */
export function mountAutoSplit(host: HTMLElement, deps: AutoSplitDeps): { update(): void } {
  function element<K extends keyof HTMLElementTagNameMap>(tag: K, className: string, text?: string) {
    const node = document.createElement(tag)
    node.className = className
    if (text !== undefined) node.textContent = text
    return node
  }
  const root = element('section', 'asp-panel')
  root.setAttribute('aria-label', '付费 API 自动拆分')
  const heading = element('header', 'asp-heading')
  heading.append(element('span', 'asp-kicker', 'AI PART ISOLATION'), element('h2', 'asp-title', '一键 API 拆分'), element('p', 'asp-copy', '选择原图 → 填写部件 → 生成并检查透明 PNG'))
  const sourceName = element('p', 'asp-source')
  const warning = element('p', 'asp-warning', 'AI 会尝试隔离部件、移除背景并补全被遮挡区域。这是生成式重绘，不是精确像素裁切；可能改变细节、产生不透明背景或失败。结果需人工检查，不会生成 moc3，也不会自动绑定 Live2D。')
  const label = element('label', 'asp-label', '部件名称 · 1–12 个')
  const names = element('textarea', 'asp-names')
  names.rows = 3
  names.value = defaultParts.join(', ')
  names.spellcheck = false
  names.setAttribute('aria-label', '部件名称，逗号或换行分隔，1 至 12 个')
  label.append(names)
  const hint = element('p', 'asp-hint', '超精细拆分默认 10 个部件，左右以角色自身为准。使用逗号（中英文均可）或换行分隔；每个名称单独请求一张图片（默认 10 次可能计费调用）。保留细线、轮廓和透明边缘的实际质量取决于接口，请检查结果。')
  const validation = element('p', 'asp-validation')
  validation.setAttribute('aria-live', 'polite')
  const fees = element('p', 'asp-fees')
  const start = element('button', 'asp-start')
  start.type = 'button'
  const status = element('p', 'asp-status', '等待操作。请先在素材库选择要拆分的原图。')
  status.setAttribute('role', 'status')
  status.setAttribute('aria-live', 'polite')
  const results = element('section', 'asp-results')
  results.hidden = true
  const resultTitle = element('h3', 'asp-result-title')
  const resultNote = element('p', 'asp-copy')
  const backgroundControls = element('div', 'asp-background-controls')
  backgroundControls.setAttribute('role', 'group')
  backgroundControls.setAttribute('aria-label', '透明图片预览背景，仅本地显示')
  backgroundControls.append(element('span', 'asp-hint', '预览背景 · 不修改图片'))
  const backgroundButtons: HTMLButtonElement[] = []
  for (const [mode, title] of [['checker', '棋盘格'], ['dark', '深色'], ['light', '浅色']] as const) {
    const button = element('button', 'asp-background-button', title)
    button.type = 'button'
    button.setAttribute('aria-pressed', String(mode === 'checker'))
    button.addEventListener('click', () => {
      if (busy) return
      root.dataset.preview = mode
      backgroundButtons.forEach(item => item.setAttribute('aria-pressed', String(item === button)))
    })
    backgroundButtons.push(button)
    backgroundControls.append(button)
  }
  const grid = element('div', 'asp-grid')
  const failureSection = element('section', 'asp-failures')
  const manifest = element('div', 'asp-manifest')
  results.append(resultTitle, resultNote, backgroundControls, grid, failureSection, manifest)
  root.append(heading, sourceName, warning, label, hint, validation, fees, start, status, results)
  root.dataset.preview = 'checker'
  host.append(root)
  let busy = false
  let runningSource: Asset | undefined

  function parseNames() {
    return names.value.split(/[,，\r\n]+/).map(name => name.trim()).filter(Boolean)
  }
  function invalidReason(parts: string[]) {
    if (parts.length < 1 || parts.length > 12) return '请填写 1–12 个部件名称。'
    if (new Set(parts).size !== parts.length) return '部件名称不能重复，请修改后再开始。'
    return ''
  }
  function update() {
    const selected = runningSource ?? deps.getSelected()
    const parts = parseNames()
    const error = invalidReason(parts)
    sourceName.textContent = selected ? `${busy ? '本次锁定原图' : '当前选中原图'}：${selected.name} · ${selected.width} × ${selected.height} px` : '当前未选择原图：请先在素材库选中一张图片。'
    validation.textContent = error
    names.setAttribute('aria-invalid', String(Boolean(error)))
    fees.textContent = `本次将发起 ${parts.length} 次独立的付费图像编辑 API 请求。后端每个部件都会重新上传同一张原始图片一次，不以上一个部件为输入；费用按服务商实际规则计算，失败也可能计费。不会自动重试。点击即开始，不再弹出确认。`
    start.textContent = busy ? `正在拆分 · ${parts.length} 次请求` : `一键 API 拆分 · ${parts.length} 次请求`
    start.disabled = busy || !selected || Boolean(error)
    names.disabled = busy
    backgroundButtons.forEach(button => { button.disabled = busy })
    root.setAttribute('aria-busy', String(busy))
  }
  function feedback(message: string, error = false) {
    status.textContent = message
    status.classList.toggle('asp-error', error)
    deps.notify(message, error)
  }
  function downloadLink(url: string, text: string, filename?: string) {
    const link = element('a', 'asp-download', text)
    link.href = assetUrl(url)
    link.download = filename ?? ''
    link.target = '_blank'
    link.rel = 'noopener noreferrer'
    return link
  }
  function renderResult(result: AutoSplitResponse, source: Asset) {
    results.hidden = false
    resultTitle.textContent = `「${source.name}」的拆分结果 · ${result.parts.length} 成功 / ${result.failures.length} 失败`
    resultNote.textContent = result.note || '请检查每张输出的透明度、轮廓及补全细节，再继续制作模型。'
    grid.replaceChildren()
    for (const part of result.parts) {
      const card = element('article', 'asp-part')
      const preview = element('div', 'asp-preview')
      const image = element('img', 'asp-image')
      image.alt = part.name
      image.loading = 'lazy'
      image.decoding = 'async'
      const imageStatus = element('p', 'asp-image-status')
      image.addEventListener('error', () => { imageStatus.textContent = '预览加载失败，可尝试下载原文件。' })
      preview.append(image)
      card.append(preview, element('h4', 'asp-part-name', part.name), element('p', 'asp-dimensions', `${part.width} × ${part.height} px · 完整适应显示`), imageStatus)
      try {
        image.src = assetUrl(part.url)
        card.append(downloadLink(part.url, '下载部件图片', part.name))
      } catch {
        imageStatus.textContent = '服务器返回了无效图片地址，无法预览或下载。'
      }
      grid.append(card)
    }
    failureSection.replaceChildren()
    failureSection.hidden = result.failures.length === 0
    if (result.failures.length) {
      failureSection.append(element('h4', 'asp-failure-title', '未成功的部件（未自动重试）'))
      const list = element('ul', 'asp-failure-list')
      for (const failure of result.failures) list.append(element('li', 'asp-failure', `${failure.name}：${failure.error}`))
      failureSection.append(list, element('p', 'asp-hint', '如需重试，请先核对服务商计费和已有输出，再手动修改上方部件列表。重新点击会产生新的付费请求。'))
    }
    manifest.replaceChildren()
    if (result.manifestUrl) {
      try { manifest.append(downloadLink(result.manifestUrl, '下载拆分清单 JSON')) }
      catch { manifest.append(element('p', 'asp-error', '拆分清单地址无效。')) }
    }
    manifest.append(element('p', 'asp-hint', `任务 ID：${result.id} · 保留原图选择，不会自动选中新输出。`))
  }
  names.addEventListener('input', update)
  start.addEventListener('click', async () => {
    if (busy) return
    const selected = deps.getSelected()
    const parts = parseNames()
    const error = invalidReason(parts)
    if (!selected || error) {
      update()
      feedback(error || '请先选择要拆分的原图。', true)
      return
    }
    // Snapshot before awaiting: parent selection changes must never change this paid operation.
    const source = { ...selected }
    runningSource = source
    busy = true
    update()
    status.classList.remove('asp-error')
    status.textContent = `正在为「${source.name}」请求 ${parts.length} 个部件，可能需要数分钟。请勿重复提交；没有自动重试。`
    let response: AutoSplitResponse
    try {
      response = await deps.post<AutoSplitResponse>('/oc/auto-split', { assetId: source.id, parts })
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '无法取得服务器响应。'
      feedback(`拆分请求未完成：${message} 请求可能已被服务商执行或计费，请先检查素材库与服务商记录，勿盲目重复提交。未自动重试。`, true)
      busy = false
      runningSource = undefined
      update()
      return
    }
    try {
      renderResult(response, source)
      feedback(`拆分完成：${response.parts.length} 个成功，${response.failures.length} 个失败。请检查下方结果；未自动重试，也未切换原图选择。`, response.failures.length > 0)
      try { deps.onComplete(response, source) }
      catch (cause) {
        const message = cause instanceof Error ? cause.message : '未知刷新错误'
        feedback(`拆分结果已收到，但素材库刷新回调失败：${message}。请保留当前结果并手动刷新素材库，不要重新提交付费拆分。`, true)
      }
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '响应格式异常'
      feedback(`服务器已响应，但结果展示失败：${message}。请检查素材库，不要直接重复付费请求。`, true)
    } finally {
      busy = false
      runningSource = undefined
      update()
    }
  })
  update()
  return { update }
}
