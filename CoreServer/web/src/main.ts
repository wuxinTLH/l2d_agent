import './tailwind.css'
import './style.scss'
import { assetUrl, validateParts, type Asset, type Part } from './domain'
import { mountCropEditor } from './crop-editor'
import { mountLiveWorkflow } from './live-workflow'
import { mountAutoSplit } from './auto-split'
let autoSplitPanel: ReturnType<typeof mountAutoSplit> | undefined
let cropEditor: ReturnType<typeof mountCropEditor> | undefined
let cropAssetId = ''

const icons: Record<string,string> = {
  spark: '<path d="m12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5Z"/>',
  grid: '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
  crop: '<path d="M6 3v15h15M3 6h15v15"/>',
  cube: '<path d="m12 3 9 5v8l-9 5-9-5V8Zm0 9 9-4M12 12 3 8m9 4v9"/>',
  settings: '<path d="M4 7h16M4 17h16"/><circle cx="9" cy="7" r="3"/><circle cx="15" cy="17" r="3"/>',
  upload: '<path d="M12 16V3m-5 5 5-5 5 5M4 15v6h16v-6"/>',
  arrow: '<path d="M5 12h14m-5-5 5 5-5 5"/>',
  image: '<rect x="3" y="3" width="18" height="18" rx="3"/><circle cx="8" cy="8" r="1.5"/><path d="m3 17 6-6 4 4 3-3 5 5"/>',
  plus: '<path d="M12 5v14M5 12h14"/>',
  refresh: '<path d="M20 7v5h-5M4 17v-5h5M6 6a8 8 0 0 1 13 3M5 15a8 8 0 0 0 13 3"/>',
  close: '<path d="m6 6 12 12M6 18 18 6"/>',
  check: '<path d="m5 12 4 4L19 6"/>',
}
const icon = (name:string) => `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${icons[name] ?? icons.spark}</svg>`
const esc = (value:unknown) => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]!))
const $ = <T extends HTMLElement = HTMLElement>(selector:string) => document.querySelector<T>(selector)!
interface Config { imageBaseUrl:string; imageModel:string; hasImageKey:boolean; cubismExecutable?:string; viewerExecutable:string; exapiUrl:string }
interface ViewerStatus {configured:boolean; exapiUrl:string; capabilities:unknown[]; note:string}
let assets:Asset[] = []
let selected:Asset | undefined
let parts:Part[] = []
let activePart = -1
let mode:'create'|'split'|'live2d' = 'create'
let config:Config | undefined
let connected = false
let noticeTimer:ReturnType<typeof setTimeout>

$('#app').innerHTML = `
<a href="#main" class="skip-link">跳至工作区</a>
<aside class="rail"><a class="brand" href="#" aria-label="人人皆可建模首页" style="font-size:14px;letter-spacing:1px;line-height:1.7;flex-wrap:wrap;text-align:center">人人皆可<br><span>建模</span><i></i></a><div class="rail-divider"></div>
<nav aria-label="工作区"><button class="nav-item active" data-mode="create" aria-current="page">${icon('spark')}<span>角色创作</span></button><button class="nav-item" data-mode="split">${icon('crop')}<span>部件拆分</span></button><button class="nav-item" data-mode="live2d">${icon('cube')}<span>Live2D</span></button></nav>
<button class="nav-item rail-settings" id="open-config">${icon('settings')}<span>设置</span></button><div class="rail-version">LOCAL<br><b>STUDIO / 01</b></div></aside>
<div class="shell"><header class="topbar"><div class="breadcrumb">工作室 <span>/</span> <strong id="crumb">角色创作</strong><span class="local-tag">本地工作流</span></div><div class="top-actions"><span id="connection" class="connection"><i></i>检测服务中</span><button class="icon-button" id="refresh" aria-label="刷新服务与资源">${icon('refresh')}</button></div></header>
<main id="main"><div class="page-heading"><div><div class="eyebrow">YOUR IMAGINATION, BROUGHT TO LIFE</div><h1 id="page-title">让想象，有自己的模样<span>。</span></h1><p id="page-subtitle">从一句灵感开始，创造属于你的原创角色。</p></div><div class="project-badge"><span class="small-square"></span> OC PROJECT <span class="muted">/ 本地素材</span></div></div>
<div class="workflow-tabs" aria-label="创作流程"><button class="active" data-mode="create"><span>01</span>角色创作</button><div></div><button data-mode="split"><span>02</span>部件拆分</button><div></div><button data-mode="live2d"><span>03</span>Live2D 工作流</button><span class="workflow-note">灵感 → 图像 → 部件 → 模型</span></div>
<section id="create-view" class="workspace-grid"><section class="panel creator"><div class="panel-heading"><h2>${icon('spark')}灵感画板</h2><span class="tiny-label">CREATE</span></div>
<form id="generate-form"><label for="prompt" class="field-label">角色描述 <span>必填</span></label><textarea id="prompt" name="prompt" rows="7" maxlength="16000" required placeholder="描绘你心中的角色…&#10;&#10;例如：银白色短发的森林精灵，翡翠色眼睛，穿着带有植物纹样的斗篷，正面全身立绘，纯色背景。"></textarea><div class="prompt-hint"><span>越具体的描述，越接近你的想象</span><span id="prompt-count">0 / 16000</span></div>
<div class="suggestion-row"><button type="button" data-prompt="正面全身立绘，双手自然展开，完整展示四肢，纯色背景">全身立绘 +</button><button type="button" data-prompt="干净的线稿，平涂色彩，清晰的部件边界">便于拆分 +</button></div>
<label for="negative" class="field-label">排除内容 <span>可选</span></label><textarea id="negative" rows="2" maxlength="16000" placeholder="不希望出现的元素，如：文字、水印、复杂背景"></textarea>
<label for="size" class="field-label">画布尺寸</label><select id="size"><option value="1024x1536">1024 × 1536 · 竖版立绘</option><option value="1024x1024">1024 × 1024 · 方形画布</option><option value="1536x1024">1536 × 1024 · 横版画布</option></select>
<button class="primary-button generate-button" type="submit">${icon('spark')}生成角色<span class="button-tail">${icon('arrow')}</span></button><p class="under-button">通过你配置的图像 API 生成 · 结果保存至本地</p></form>
<div class="or-divider"><span>或从已有角色开始</span></div><label class="import-zone" for="import-file" tabindex="0" role="button" aria-controls="import-file">${icon('upload')}<span><strong>导入角色图片</strong><small>PNG / JPG · 保留原始尺寸</small></span><span>↗</span></label><input class="sr-only" id="import-file" type="file" accept="image/png,image/jpeg"><div id="import-status" class="helper" role="status"></div></section>
<section class="panel preview-panel"><div class="panel-heading"><h2>${icon('image')}角色预览</h2><span id="preview-meta" class="tiny-label">NO ASSET SELECTED</span></div><div id="preview" class="preview checker"><div class="empty-preview"><div class="empty-orbit"><span>✧</span><i></i><b></b></div><h3>下一个角色，正等待诞生</h3><p>输入描述生成角色，或导入一张已有图片<br>你的创作会在这里呈现</p><div class="empty-tags"><span>ORIGINAL CHARACTER</span><span>无限可能</span></div></div></div><div class="preview-footer"><div><span class="status-dot"></span><span id="selected-name">尚未选择素材</span></div><div class="flex gap-2"><a id="download-source" class="text-button hidden" target="_blank" rel="noopener">打开原图 ↗</a><button id="go-split" class="text-button" disabled>前往拆分 ${icon('arrow')}</button></div></div>
<div class="edit-area"><label for="edit-prompt" class="field-label">基于当前图片编辑 <span>图生图</span></label><div class="edit-input-row"><input id="edit-prompt" placeholder="例如：将斗篷改为深绿色，保持角色其余特征" maxlength="16000"><button id="edit-asset" class="secondary-button" disabled>提交编辑 ${icon('arrow')}</button></div><p class="helper">使用当前素材的 sourceId；遮挡部位需另行补绘，不会自动还原。</p></div></section></section>
<section id="split-view" class="split-layout hidden"><div id="auto-split" style="grid-column:1/-1;min-width:0"></div><section class="panel"><div class="panel-heading"><h2>${icon('crop')}手动裁切画布</h2><span class="tiny-label">PIXEL COORDINATES</span></div><div class="info-strip">绘制框后可选中移动、拖动边角调整。滚轮缩放，按住 Ctrl / 空格或中键拖动画布；坐标始终对应原图像素。仅矩形裁切，不执行语义分割。</div><div class="crop-stage checker" id="crop-stage"><p class="empty-copy">请先在角色创作中生成或选择素材</p></div><div class="preview-footer"><span id="crop-meta">尚未选择素材</span><span>坐标原点：左上角 · 单位 px</span></div></section><section class="panel parts-panel"><div class="panel-heading"><h2>部件列表 <span id="parts-count" class="count">0</span></h2><button class="icon-button" id="add-part" aria-label="新增裁切部件">${icon('plus')}</button></div><div id="parts-list" class="parts-list"><p class="empty-copy">添加矩形，开始整理部件。</p></div><div class="parts-actions"><p class="warning-copy">裁切不会补全被遮挡的内容。用于 Live2D 的独立图层通常还需去背景、修边和补绘。</p><button id="split-submit" class="primary-button" disabled>${icon('crop')}导出裁切部件</button></div></section><section class="panel split-output"><div class="panel-heading"><h2>拆分输出</h2><a id="manifest" class="text-button hidden" target="_blank" rel="noopener">下载 manifest ↗</a></div><div id="split-results" class="output-grid"><p class="empty-copy">完成后在这里查看真实导出的部件。</p></div></section></section>
<section id="live2d-view" class="live-layout hidden"><div id="live-workflow" style="grid-column:1/-1;min-width:0"></div><section class="panel"><div class="panel-heading"><h2>${icon('cube')}可选播放 · Live2DViewerEX 连接</h2><span class="tiny-label">LOCAL BRIDGE</span></div><div class="viewer-status"><div class="viewer-symbol">${icon('cube')}</div><div><h3 id="viewer-title">尚未检测</h3><p id="viewer-note">仅在需要可选播放时配置 ViewerEX 与 EXAPI；它们不是 Cubism 编辑器接口。</p></div></div><div class="flex flex-wrap gap-3"><button id="viewer-refresh" class="secondary-button">${icon('refresh')}检查配置</button><button id="viewer-launch" class="primary-button">启动 ViewerEX ${icon('arrow')}</button></div><pre id="viewer-details" class="json-output">等待检测…</pre><div class="info-strip">本工作室不生成 .moc3，也不提供自动绑定。请先使用 Cubism Editor 制作可运行的 Live2D 模型。</div></section><section class="panel"><div class="panel-heading"><h2>本地模型检查</h2><span class="tiny-label">INSPECT</span></div><form id="inspect-form"><label class="field-label" for="model-path">Cubism .model3.json 文件绝对路径</label><input id="model-path" placeholder="D:/Models/MyCharacter/model.model3.json" required><p class="helper">路径由本地 Go 服务读取，不会从浏览器上传整个目录。</p><button type="submit" class="secondary-button">检查模型结构 ${icon('arrow')}</button></form><pre id="inspect-output" class="json-output">尚未检查模型。</pre></section><section class="panel command-panel"><div class="panel-heading"><h2>可选播放 · ViewerEX EXAPI 原始命令</h2><span class="tiny-label">ADVANCED</span></div><p class="helper">这是可选 ViewerEX 播放接口，不是 Cubism 编辑器接口。仅转发你输入的 JSON 对象。可使用注册回显消息 <code>{"msg":10000,"msgId":1}</code> 测试。只有 msg 10000 等待关联回显；其他命令仅返回 sent / verified:false，表示已写入连接而非动作成功。超时也可能已送达，请勿盲目重试。</p><form id="command-form"><label class="field-label" for="command-json">消息对象 JSON</label><textarea id="command-json" rows="6" spellcheck="false" placeholder="在此粘贴符合你的 EXAPI 版本的消息对象" required></textarea><button class="secondary-button" type="submit">发送命令 ${icon('arrow')}</button></form><pre id="command-output" class="json-output">尚未发送命令。</pre></section></section>
<section class="panel library"><div class="panel-heading"><h2>${icon('grid')}本地素材库 <span id="asset-count" class="count">0</span></h2><button class="text-button" id="refresh-assets">刷新 ${icon('refresh')}</button></div><div id="asset-list" class="asset-list"><p class="empty-copy">正在读取本地素材…</p></div></section><footer class="page-footer"><span><span class="footer-logo">人人皆可建模</span> 让每个角色，从灵感走向鲜活。</span><span>LOCAL FIRST <i>·</i> OC & LIVE2D WORKSPACE</span></footer></main></div>
<dialog id="config-dialog"><form id="config-form"><div class="panel-heading"><h2>${icon('settings')}工作室设置</h2><button id="close-config" type="button" class="icon-button" aria-label="关闭设置">${icon('close')}</button></div><p class="helper">配置由本机服务保存。密钥仅在提交时发送，不会存入浏览器存储或重新显示。</p><p class="warning-copy">注意：API Key 由后端以明文保存在本地配置文件中。请保护配置文件与机器访问权限，不要提交到版本库。</p><h3>图像生成 API</h3><label class="field-label" for="image-base">API Base URL</label><input id="image-base" type="url" placeholder="https://api.example.com/v1" required><label class="field-label" for="image-model">模型名称</label><input id="image-model" placeholder="填写服务支持的模型标识" required><label class="field-label" for="image-key">API Key <span id="key-state">尚未读取</span></label><input id="image-key" type="password" autocomplete="new-password" placeholder="留空保留已保存的密钥"><h3>Live2D Cubism Editor · 模型制作</h3><p class="helper"><a href="https://www.live2d.com/zh-CHS/cubism/download/editor/" target="_blank" rel="noopener noreferrer">官方下载 Cubism Editor ↗</a> · 默认查找项目根目录的 Live2DCubism 文件夹。请将官方安装完成后的完整编辑器和依赖放入该文件夹，或填写真实安装目录中的编辑器可执行文件路径。安装包不等于编辑器，不能将安装程序作为启动路径；本项目不附带软件本体。</p><label class="field-label" for="cubism-executable">Cubism Editor 可执行文件绝对路径（可选）</label><input id="cubism-executable" placeholder="填写已安装的 Cubism Editor 程序绝对路径"><p class="helper">路径发现仅列出候选，请核对后手动填写并保存。启动编辑器不等于自动制作网格、绑定或生成 moc3。</p><h3>可选播放 · Live2DViewerEX / EXAPI</h3><p class="helper">仅用于播放已有模型，不是 Cubism 编辑器接口；不使用桌面播放时可留空。<a href="https://store.steampowered.com/app/616720/Live2DViewerEX/" target="_blank" rel="noopener noreferrer">可选：通过 Steam 获取 ViewerEX ↗</a> · 需登录并取得授权。可使用 Steam 原安装路径；若使用项目的 L2dViewerEX 文件夹，请放入官方取得的完整程序和依赖。本项目不附带软件本体。</p><label class="field-label" for="viewer-executable">ViewerEX 可执行文件绝对路径（可选）</label><input id="viewer-executable" placeholder="D:/Live2DViewerEX/Live2DViewerEX.exe"><label class="field-label" for="exapi-url">EXAPI 地址</label><input id="exapi-url" type="url" placeholder="ws://127.0.0.1:10086/api"><div class="dialog-footer"><span id="config-state" class="helper" role="status"></span><button class="primary-button" id="save-config" type="submit">保存配置 ${icon('check')}</button></div></form></dialog><div id="toast" class="toast hidden" role="status" aria-live="polite"></div>`

async function api<T>(path:string, init:RequestInit = {}):Promise<T> {
  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), path === '/oc/auto-split' ? 2400000 : path.includes('/oc/') ? 240000 : 20000)
  try {
    const response = await fetch(`/api${path}`, { ...init, signal:controller.signal, headers: init.body instanceof FormData ? init.headers : { 'Content-Type':'application/json', ...init.headers } })
    const text = await response.text()
    let data:unknown
    try { data = text ? JSON.parse(text) : undefined } catch { throw new Error(`服务返回非 JSON 内容（HTTP ${response.status}），请检查后端是否就绪。`) }
    if (!response.ok) {
      const error = data as {error?:unknown;message?:unknown} | undefined
      // Never surface arbitrary server bodies: configuration errors could echo secrets.
      const message = typeof error?.error === 'string' ? error.error : typeof error?.message === 'string' ? error.message : `请求失败（HTTP ${response.status}）`
      throw new Error(path === '/config' ? `配置请求失败（HTTP ${response.status}），请检查服务日志。` : message)
    }
    return data as T
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw new Error('请求超时。服务可能仍在处理，请刷新素材库确认后再重试。')
    if (error instanceof TypeError) throw new Error('无法连接本地服务，请确认 Go 后端正在运行。')
    throw error
  } finally { clearTimeout(timeout) }
}
const post = <T>(path:string, body:unknown) => api<T>(path, {method:'POST', body:JSON.stringify(body)})
function notify(message:string, error = false) {
  clearTimeout(noticeTimer)
  const toast = $('#toast'); toast.textContent = message; toast.className = `toast${error ? ' error' : ''}`; toast.setAttribute('role', error ? 'alert' : 'status')
  noticeTimer = setTimeout(() => toast.classList.add('hidden'), 6500)
}
async function busy(button:HTMLButtonElement, task:()=>Promise<void>) {
  const old = button.innerHTML; button.disabled = true; button.setAttribute('aria-busy','true'); button.textContent = '处理中…'
  try { await task() } catch(error) { notify(error instanceof Error ? error.message : '操作失败，请稍后重试。', true) }
  finally { button.innerHTML = old; button.disabled = false; button.removeAttribute('aria-busy'); updateSelectionButtons() }
}
function updateSelectionButtons() {
  for (const id of ['#go-split','#edit-asset','#split-submit','#add-part']) if (!$(id).hasAttribute('aria-busy')) $(id).toggleAttribute('disabled', !selected || (id === '#split-submit' && !parts.length))
}
function switchMode(next:typeof mode) {
  mode = next
  document.querySelectorAll<HTMLElement>('[data-mode]').forEach(el => { const active = el.dataset.mode === next; el.classList.toggle('active',active); if (active) el.setAttribute('aria-current','page'); else el.removeAttribute('aria-current') })
  const labels = {create:['角色创作','让想象，有自己的模样','从一句灵感开始，创造属于你的原创角色。'], split:['部件拆分','每个细节，都有自己的位置','API 一键隔离补绘，或手动精确裁切，为模型制作准备素材。'],live2d:['Live2D 工作流','让角色，走进真实的舞台','下载并启动 Cubism Editor 制作模型；ViewerEX 仅为可选播放。']}
  $('#crumb').textContent = labels[next][0]; $('#page-title').innerHTML = `${labels[next][1]}<span>。</span>`; $('#page-subtitle').textContent = labels[next][2]
  for (const view of ['create','split','live2d']) $(`#${view}-view`).classList.toggle('hidden',next !== view)
  if (next === 'split') renderCrop()
  if (next === 'live2d') void loadViewer()
}
function selectAsset(asset:Asset) {
  cropEditor?.destroy(); cropEditor = undefined; cropAssetId = ''
  selected = asset; parts = []; activePart = -1
  $('#split-results').innerHTML = '<p class="empty-copy">完成后在这里查看真实导出的部件。</p>'; $('#manifest').classList.add('hidden')
  $('#preview').innerHTML = `<img src="${esc(assetUrl(asset.url))}" alt="${esc(asset.name)}" id="selected-image">`
  $('#selected-image').addEventListener('error',()=> notify('预览图片加载失败，请检查后端资源路径。',true), {once:true})
  $('#preview-meta').textContent = `${asset.width} × ${asset.height} PX`; $('#selected-name').textContent = asset.name
  const link = $<HTMLAnchorElement>('#download-source'); link.href = assetUrl(asset.url); link.classList.remove('hidden')
  renderAssets(); renderParts(); renderCrop(); updateSelectionButtons(); autoSplitPanel?.update()
}
function upsert(asset:Asset) {
  if (!asset || typeof asset.id !== 'string' || !asset.url || !(asset.width > 0) || !(asset.height > 0)) throw new Error('服务返回的素材信息不完整。')
  assetUrl(asset.url)
  assets = [asset, ...assets.filter(a => a.id !== asset.id)]; selectAsset(asset)
}
function renderAssets() {
  $('#asset-count').textContent = String(assets.length)
  $('#asset-list').innerHTML = assets.length ? assets.map(a => `<button class="asset-card ${selected?.id === a.id ? 'selected' : ''}" data-asset="${esc(a.id)}" aria-pressed="${selected?.id === a.id}"><div class="asset-thumb checker"><img src="${esc(assetUrl(a.url))}" alt="" loading="lazy">${selected?.id === a.id ? `<span class="selected-mark">${icon('check')}</span>` : ''}</div><strong>${esc(a.name)}</strong><span>${a.width} × ${a.height}<i>IMAGE</i></span></button>`).join('') : '<div class="library-empty">'+icon('image')+'<div><strong>你的角色档案，从这里开始</strong><p>生成或导入的图片会保存在本地，并出现在素材库。</p></div></div>'
}
async function loadAssets() { const result = await api<Asset[]>('/assets'); if (!Array.isArray(result)) throw new Error('素材列表响应不是数组。'); assets = result; renderAssets() }
async function refresh() {
  const results = await Promise.allSettled([api<{status:string;modules:Record<string,unknown>}>('/health'), loadAssets(), loadConfig()])
  connected = results[0].status === 'fulfilled' && ['ok','healthy'].includes(results[0].value.status)
  $('#connection').className = `connection ${connected ? 'online' : 'offline'}`; $('#connection').innerHTML = `<i></i>${connected ? '本地服务已连接' : '本地服务未就绪'}`
  if (results[0].status === 'fulfilled') $('#connection').title = JSON.stringify(results[0].value.modules)
  if (results[1].status === 'rejected') $('#asset-list').innerHTML = '<p class="empty-copy error-text">素材库读取失败。请检查本地服务后点击刷新。</p>'
  if (results.some(r => r.status === 'rejected')) notify('部分服务尚未就绪。可先在设置中配置，启动后端后重新检测。',true)
}
async function loadConfig() {
  config = await api<Config>('/config')
  $<HTMLInputElement>('#image-base').value = config.imageBaseUrl ?? ''; $<HTMLInputElement>('#image-model').value = config.imageModel ?? ''
  $<HTMLInputElement>('#cubism-executable').value = config.cubismExecutable ?? ''
  $<HTMLInputElement>('#viewer-executable').value = config.viewerExecutable ?? ''; $<HTMLInputElement>('#exapi-url').value = config.exapiUrl ?? ''
  $('#key-state').textContent = config.hasImageKey ? '已保存 · 不回显' : '未配置'
}
async function loadViewer() {
  try { const status = await api<ViewerStatus>('/l2d/status'); $('#viewer-title').textContent = status.configured ? 'ViewerEX 已配置 · 未验证连接' : '尚未配置 ViewerEX'; $('#viewer-note').textContent = status.note || '以实际启动与 EXAPI 响应为准。'; $('#viewer-details').textContent = JSON.stringify(status,null,2) }
  catch(error) { $('#viewer-title').textContent = '无法读取 ViewerEX 状态'; $('#viewer-note').textContent = error instanceof Error ? error.message : '连接失败'; $('#viewer-details').textContent = '状态检测失败，未假定 ViewerEX 已连接。' }
}
function renderParts() {
  $('#parts-count').textContent = String(parts.length)
  $('#parts-list').innerHTML = parts.length ? parts.map((p,i) => `<div class="part-row ${activePart === i ? 'active' : ''}" data-part-row="${i}"><div class="part-title"><span class="part-number">${String(i+1).padStart(2,'0')}</span><input aria-label="部件 ${i+1} 名称" data-part="${i}" data-field="name" value="${esc(p.name)}" maxlength="100"><button class="icon-button remove-part" data-remove="${i}" aria-label="删除部件 ${esc(p.name)}">${icon('close')}</button></div><div class="part-fields">${(['x','y','width','height'] as const).map(key => `<label>${{x:'X',y:'Y',width:'宽 W',height:'高 H'}[key]}<input type="number" step="1" min="${key === 'width' || key === 'height' ? 1 : 0}" data-part="${i}" data-field="${key}" value="${p[key]}" aria-label="${esc(p.name)} ${key}"></label>`).join('')}</div></div>`).join('') : '<p class="empty-copy">拖动画布或点击 + 添加矩形部件。</p>'
  updateSelectionButtons()
}
function renderCrop() {
  if (!selected) return
  $('#crop-meta').textContent = `${selected.name} · ${selected.width} × ${selected.height} px`
  if (!cropEditor || cropAssetId !== selected.id) {
    cropEditor?.destroy()
    cropAssetId = selected.id
    cropEditor = mountCropEditor($('#crop-stage'), {asset:{...selected,url:assetUrl(selected.url)},parts,activeIndex:activePart,onChange:(next,index)=> {parts = next; activePart = index; renderParts()}})
  } else cropEditor.update(parts,activePart)
}
function renderOverlays() { cropEditor?.update(parts,activePart) }

document.querySelectorAll<HTMLElement>('[data-mode]').forEach(el => el.addEventListener('click',()=>switchMode(el.dataset.mode as typeof mode)))
$('#go-split').addEventListener('click',()=>switchMode('split'))
$('#refresh').addEventListener('click',()=>void busy($<HTMLButtonElement>('#refresh'),refresh))
$('#refresh-assets').addEventListener('click',()=>void busy($<HTMLButtonElement>('#refresh-assets'),loadAssets))
$('#asset-list').addEventListener('click', event => { const id = (event.target as HTMLElement).closest<HTMLElement>('[data-asset]')?.dataset.asset; const asset = assets.find(a=>a.id === id); if (asset) selectAsset(asset) })
$('#prompt').addEventListener('input',()=> { $('#prompt-count').textContent = `${$<HTMLTextAreaElement>('#prompt').value.length} / 16000` })
document.querySelectorAll<HTMLElement>('[data-prompt]').forEach(el=>el.addEventListener('click',()=> { const input = $<HTMLTextAreaElement>('#prompt'); input.value = [input.value,el.dataset.prompt].filter(Boolean).join('，'); input.dispatchEvent(new Event('input')); input.focus() }))
$('#generate-form').addEventListener('submit', event => { event.preventDefault(); void busy($<HTMLButtonElement>('.generate-button'),async()=> { const prompt = $<HTMLTextAreaElement>('#prompt').value.trim(); if (!prompt) throw new Error('请先描述你的角色。'); upsert(await post<Asset>('/oc/generate',{prompt,negativePrompt:$<HTMLTextAreaElement>('#negative').value.trim(),size:$<HTMLSelectElement>('#size').value})); notify('角色已生成并保存至本地素材库。') }) })
$('.import-zone').addEventListener('keydown',event => { if(event.key === 'Enter' || event.key === ' ') {event.preventDefault();$<HTMLInputElement>('#import-file').click()} })
$('#import-file').addEventListener('change',async () => {
  const input = $<HTMLInputElement>('#import-file'); const file = input.files?.[0]; if (!file) return
  input.disabled = true; $('#import-status').textContent = '正在上传并导入…'
  try { if (!['image/png','image/jpeg'].includes(file.type)) throw new Error('请选择 PNG 或 JPG 图片。'); const data = new FormData(); data.append('image',file); upsert(await api<Asset>('/oc/import',{method:'POST',body:data})); notify('图片已导入。') }
  catch(error) { notify(error instanceof Error ? error.message : '导入失败',true) } finally {input.value = '';input.disabled = false; $('#import-status').textContent = ''}
})
$('#edit-asset').addEventListener('click',()=>void busy($<HTMLButtonElement>('#edit-asset'),async()=> { const source = selected; const prompt = $<HTMLInputElement>('#edit-prompt').value.trim(); if (!source || !prompt) throw new Error('请选择图片并填写编辑要求。'); const data = new FormData(); data.append('sourceId',source.id);data.append('prompt',prompt); upsert(await api<Asset>('/oc/edit',{method:'POST',body:data})); notify('编辑结果已保存为新素材。') }))
$('#add-part').addEventListener('click',()=> { if (!selected) return; parts.push({name:`部件 ${parts.length+1}`,x:0,y:0,width:selected.width,height:selected.height}); activePart = parts.length-1;renderParts();renderOverlays() })
$('#parts-list').addEventListener('input',event=> { const input = event.target as HTMLInputElement; if (input.dataset.part === undefined) return; const part = parts[Number(input.dataset.part)]; const field = input.dataset.field as keyof Part; if (field === 'name') part.name = input.value; else part[field] = input.valueAsNumber; activePart = Number(input.dataset.part); renderOverlays() })
$('#parts-list').addEventListener('focusin',event=> {const row = (event.target as HTMLElement).closest<HTMLElement>('[data-part-row]'); if(row) {activePart = Number(row.dataset.partRow); renderOverlays(); $('#parts-list').querySelectorAll<HTMLElement>('[data-part-row]').forEach(el=>el.classList.toggle('active',Number(el.dataset.partRow) === activePart))}})
$('#parts-list').addEventListener('click',event=> { const button = (event.target as HTMLElement).closest<HTMLElement>('[data-remove]'); if (!button) return; parts.splice(Number(button.dataset.remove),1);activePart = Math.min(activePart,parts.length-1);renderParts();renderOverlays() })
$('#split-submit').addEventListener('click',()=>void busy($<HTMLButtonElement>('#split-submit'),async()=> { if (!selected) throw new Error('请选择素材。'); const error = validateParts(parts,selected.width,selected.height); if (error) throw new Error(error); const sourceId = selected.id; const result = await post<{id:string;parts:Asset[];manifestUrl:string}>('/oc/split',{assetId:sourceId,parts:parts.map(p=>({...p,name:p.name.trim()}))}); if (selected?.id !== sourceId) { notify('原素材的裁切已完成；当前已选择其他素材，未覆盖当前工作区。'); return } if (!Array.isArray(result.parts)) throw new Error('服务未返回部件列表。'); $('#split-results').innerHTML = result.parts.map(p=>`<a class="output-card" href="${esc(assetUrl(p.url))}" target="_blank" rel="noopener"><div class="checker"><img src="${esc(assetUrl(p.url))}" alt="${esc(p.name)}"></div><strong>${esc(p.name)} ↗</strong><small>${p.width} × ${p.height} px</small></a>`).join(''); const link = $<HTMLAnchorElement>('#manifest'); link.classList.toggle('hidden',!result.manifestUrl); if(result.manifestUrl) link.href = assetUrl(result.manifestUrl); notify(`已导出 ${result.parts.length} 个裁切部件。`) }))
const dialog = $<HTMLDialogElement>('#config-dialog')
$('#open-config').addEventListener('click',()=> {$('#config-state').textContent = config ? '' : '尚未加载配置；可填写后尝试保存。';dialog.showModal()})
$('#close-config').addEventListener('click',()=>dialog.close())
dialog.addEventListener('close',()=> { $<HTMLInputElement>('#image-key').value = '' })
$('#config-form').addEventListener('submit',event=> { event.preventDefault();void busy($<HTMLButtonElement>('#save-config'),async()=> { const keyInput = $<HTMLInputElement>('#image-key'); const body:Record<string,string> = {imageBaseUrl:$<HTMLInputElement>('#image-base').value.trim(),imageModel:$<HTMLInputElement>('#image-model').value.trim(),cubismExecutable:$<HTMLInputElement>('#cubism-executable').value.trim(),viewerExecutable:$<HTMLInputElement>('#viewer-executable').value.trim(),exapiUrl:$<HTMLInputElement>('#exapi-url').value.trim()}; if(keyInput.value.trim()) body.imageApiKey = keyInput.value.trim(); keyInput.value = ''; $('#config-state').textContent = '正在保存…'; try { await api('/config',{method:'PUT',body:JSON.stringify(body)}); $('#config-state').textContent = '配置已保存。'; notify('配置已由本地服务保存。'); await loadConfig() } catch(error) { $('#config-state').textContent = '保存或回读失败，请重新检查配置。'; throw error } }) })
$('#viewer-refresh').addEventListener('click',()=>void busy($<HTMLButtonElement>('#viewer-refresh'),loadViewer))
$('#viewer-launch').addEventListener('click',()=>void busy($<HTMLButtonElement>('#viewer-launch'),async()=> { const result = await post<{status:string}>('/l2d/launch',{}); $('#viewer-details').textContent = JSON.stringify(result,null,2);notify(`本地启动请求已返回：${result.status}。这不代表模型已加载。`) }))
$('#inspect-form').addEventListener('submit', event=> {event.preventDefault();void busy($<HTMLButtonElement>('#inspect-form button'),async()=> {const path = $<HTMLInputElement>('#model-path').value.trim();if(!path) throw new Error('请填写模型绝对路径。');try {$('#inspect-output').textContent = JSON.stringify(await post('/l2d/inspect',{modelPath:path}),null,2)} catch(error) {$('#inspect-output').textContent = '模型检查失败；请检查路径、文件和服务状态。';throw error} })})
$('#command-form').addEventListener('submit',event=> {event.preventDefault();void busy($<HTMLButtonElement>('#command-form button'),async()=> {let message:unknown;try {message = JSON.parse($<HTMLTextAreaElement>('#command-json').value)}catch {throw new Error('命令不是有效的 JSON。')} if(!message || typeof message !== 'object' || Array.isArray(message)) throw new Error('消息必须是 JSON 对象，不能为数组或基本类型。');try {$('#command-output').textContent = JSON.stringify(await post('/l2d/command',{message}),null,2)}catch(error){$('#command-output').textContent = '命令发送失败，未确认 EXAPI 执行。';throw error} })})
autoSplitPanel = mountAutoSplit($('#auto-split'),{getSelected:()=>selected,post,notify,onComplete:()=>{void loadAssets().catch(()=>notify('部件已生成，素材库刷新失败；请手动刷新，勿重复付费拆分。',true))}})
mountLiveWorkflow($('#live-workflow'),{post,notify,openSettings:()=>$('#open-config').click()})
updateSelectionButtons()
void refresh()
