import './asset-library.scss'
import { applyCubismTemplate, restoreSourcePositions, cubismParts } from './cubism-parts'
import { LIBRARY_GROUPS_KEY, addLibraryGroup, assignLibraryGroup, assetGroup, changeVisibleSelection, emptyLibraryGroups, filterLibraryAssets, parseLibraryGroups, removeLibraryGroup, visibleSelection, type GroupFilter, type LibraryGroups } from './library-groups'
import { assetUrl, type Asset } from './domain'
import { psdDraftWarnings, createPsdDraft, deleteSequential, psdDownloadUrl, validatePsdDraft, type PsdDraft, type PsdExport } from './library-model'

interface Dependencies {
  getAssets:()=>Asset[]
  getPreviewId:()=>string | undefined
  preview:(asset:Asset)=>void
  refresh:()=>Promise<void>
  remove:(id:string)=>Promise<unknown>
  post:<T>(path:string,body:unknown)=>Promise<T>
  onDeleted:(ids:string[])=>void
  notify:(message:string,error?:boolean)=>void
}
const esc = (value:unknown) => String(value ?? '').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]!))
export function mountAssetLibrary(root:HTMLElement, deps:Dependencies) {
  let checked = new Set<string>()
  let groups = emptyLibraryGroups(), filter:GroupFilter = {kind:'all'}, storageReadable = true
  try { groups = parseLibraryGroups(localStorage.getItem(LIBRARY_GROUPS_KEY)) }
  catch { storageReadable = false; deps.notify('浏览器分组数据无法读取；现有存储不会被覆盖。素材不受影响。请检查浏览器存储设置或备份后修复分组数据。',true) }
  function saveGroups(next:LibraryGroups) {
    if (!storageReadable) { deps.notify('分组存储不可用，未修改分组。请先检查浏览器存储。',true); return false }
    try { localStorage.setItem(LIBRARY_GROUPS_KEY,JSON.stringify(next)); groups = next; return true }
    catch { deps.notify('浏览器分组保存失败（可能存储空间不足或已禁用）；未修改分组，素材不受影响。',true); return false }
  }
  const visibleAssets = () => filterLibraryAssets(deps.getAssets(),groups,filter)
  let pendingIds:string[] = [], deleting = false, exporting = false, draft:PsdDraft | undefined, result:PsdExport | undefined
  root.innerHTML = `<div class="panel-heading"><h2>本地素材库 <span class="count" data-count>0</span></h2><button class="text-button" data-refresh>刷新</button></div>
    <div class="library-group-panel"><div class="library-tools"><label>分组筛选 <select data-group-filter aria-label="分组筛选"></select></label><button class="text-button" data-delete-group>删除当前分组</button></div><form class="library-tools" data-group-form><label>新分组 <input data-group-name maxlength="60" placeholder="输入分组名称" autocomplete="off" aria-label="新分组名称"></label><button class="secondary-button" type="submit" data-create-group>创建分组</button></form><p class="helper">分组仅保存在当前浏览器，不会同步到服务器或其他浏览器。删除分组仅将素材移回未分组，不删除素材文件。</p></div>
    <div class="library-tools"><button class="secondary-button" data-all>全选当前可见</button><button class="secondary-button" data-invert>反选当前可见</button><button class="text-button" data-clear>清空全部选择</button><span data-selection role="status"></span><button class="secondary-button" data-export>选中素材导出 PSD</button><button class="danger-button" data-delete-selected>删除选中</button></div>
    <div class="library-tools"><label>将全部已选素材移至 <select data-group-target aria-label="批量分组目标"></select></label><button class="secondary-button" data-assign-group>移动已选素材</button></div>
    <p class="helper">全选 / 取消全选和反选只影响当前可见素材，隐藏的已选项保留；移动、删除和 PSD 导出作用于全部已选项（含隐藏项）。点击图片预览；PSD 最多 64 层。</p><div class="asset-list" data-list></div>
    <dialog class="delete-dialog" aria-labelledby="delete-title"><h2 id="delete-title">确认永久删除素材？</h2><p class="warning-copy">这将删除本地素材，不能撤销。不会自动删除其他派生素材。</p><ul data-delete-names></ul><p data-delete-status role="status"></p><div class="library-tools"><button class="secondary-button" data-cancel-delete autofocus>取消</button><button class="danger-button" data-confirm-delete>确认永久删除</button></div></dialog>
    <dialog class="psd-dialog" aria-labelledby="psd-title"><div class="panel-heading"><h2 id="psd-title">多图层 PSD 导出</h2><button class="text-button" data-close-export>关闭</button></div><p class="warning-copy">新拆分部件按已记录的原图坐标放置；无坐标的旧素材默认 (0, 0)。未提供原图定位框的 AI 部件比例和位置仍需人工核对。可手动调整输出宽高、X/Y、部件组和上下层；仅导出副本缩放，不修改素材。画布边缘外的内容可能被裁切。</p><div class="library-tools"><button class="secondary-button" data-cubism-template>应用 Cubism 部件方案</button><button class="secondary-button" data-restore-position>自动恢复原图位置</button></div><p class="helper">模板使用截图分类及绘制优先级；参考图置底避免遮挡。数字是排序参考，不是已写入 Cubism 绘制顺序属性。位置在 PSD 中预先恢复，导入时继承；未知比例/坐标不猜测。</p><fieldset data-editor><div class="psd-size"><label>画布宽（px）<input data-width type="number" min="1" step="1"></label><label>画布高（px）<input data-height type="number" min="1" step="1"></label></div><p class="helper">下列图层从上到下排列，首行是最上层。预览仅按比例显示，不改变导出尺寸。</p><div class="psd-editor-layout"><div data-layers></div><div><div class="psd-composite checker" data-composite aria-label="图层合成预览"></div></div></div></fieldset><p data-export-status role="status"></p><div class="library-tools"><button class="primary-button" data-submit-export>生成 PSD</button><a class="secondary-button hidden" data-download download>下载 PSD</a><button class="secondary-button hidden" data-open-cubism>在 Cubism 中打开此 PSD</button></div><p class="helper">启动编辑器/提交打开请求不等于导入完成；请在 Cubism 中确认文档、图层并继续制作。</p></dialog>`
  const q = <T extends HTMLElement = HTMLElement>(selector:string) => root.querySelector<T>(selector)!
  const deleteDialog = q<HTMLDialogElement>('.delete-dialog'), exportDialog = q<HTMLDialogElement>('.psd-dialog')
  function status(message:string) { q('[data-export-status]').textContent = message }
  function invalidate() { result = undefined; q('[data-download]').classList.add('hidden'); q('[data-open-cubism]').classList.add('hidden') }
  function update() {
    const assets = deps.getAssets(), live = new Set(assets.map(a=>a.id))
    for (const id of checked) if (!live.has(id)) checked.delete(id)
    if (draft?.layers.some(l=>!live.has(l.assetId))) { invalidate(); status('部分素材已被删除，请关闭并重新选择图层。') }
    q('[data-count]').textContent = String(assets.length)
    const visible = visibleAssets(), selection = visibleSelection(checked,visible.map(a=>a.id))
    const groupOptions = groups.groups.map(g=>`<option value="${esc(g.id)}">${esc(g.name)}（${assets.filter(a=>assetGroup(groups,a.id)===g.id).length}）</option>`).join('')
    q<HTMLSelectElement>('[data-group-filter]').innerHTML = `<option value="all">全部（${assets.length}）</option><option value="ungrouped">未分组（${assets.filter(a=>!assetGroup(groups,a.id)).length}）</option>${groupOptions}`
    q<HTMLSelectElement>('[data-group-filter]').value = filter.kind === 'group' ? filter.id : filter.kind
    const target = q<HTMLSelectElement>('[data-group-target]'), previousTarget = target.value
    target.innerHTML = `<option value="">未分组</option>${groupOptions}`
    if (groups.groups.some(g=>g.id === previousTarget)) target.value = previousTarget
    q('[data-selection]').textContent = `可见 ${visible.length} / ${assets.length} 项 · 已选共 ${checked.size} 项（可见 ${selection.selected}，隐藏 ${selection.hidden}）`
    q('[data-delete-selected]').textContent = `删除全部已选（${checked.size}）`
    q('[data-export]').textContent = `全部已选导出 PSD（${checked.size}）`
    q('[data-assign-group]').textContent = `移动全部已选（${checked.size}）`
    q('[data-all]').textContent = selection.all ? '取消全选当前可见' : '全选当前可见'
    q<HTMLButtonElement>('[data-delete-selected]').disabled = deleting || !checked.size
    q<HTMLButtonElement>('[data-export]').disabled = deleting || exporting || !checked.size || checked.size > 64
    q<HTMLButtonElement>('[data-all]').disabled = deleting || !visible.length
    q<HTMLButtonElement>('[data-invert]').disabled = deleting || !visible.length
    q<HTMLButtonElement>('[data-clear]').disabled = deleting || !checked.size
    q<HTMLButtonElement>('[data-assign-group]').disabled = deleting || !checked.size || !storageReadable
    q<HTMLButtonElement>('[data-delete-group]').disabled = deleting || filter.kind !== 'group' || !storageReadable
    q<HTMLButtonElement>('[data-create-group]').disabled = deleting || !storageReadable
    q('[data-list]').innerHTML = visible.length ? visible.map(a=>`<article class="asset-card ${deps.getPreviewId() === a.id ? 'selected' : ''}"><label class="asset-checkbox"><input type="checkbox" data-check="${esc(a.id)}" ${checked.has(a.id) ? 'checked' : ''} ${deleting ? 'disabled' : ''} aria-label="选择 ${esc(a.name)}"> 选择</label><button class="asset-preview-button" data-preview="${esc(a.id)}" aria-pressed="${deps.getPreviewId() === a.id}"><div class="asset-thumb checker"><img src="${esc(assetUrl(a.url))}" alt="${esc(a.name)}" loading="lazy"></div><strong>${esc(a.name)}</strong><small>${a.width} × ${a.height} px</small>${a.processingWarning?`<small class="error-text">待手动处理：${esc(a.processingWarning)}</small>`:''}<small class="asset-group-name">${esc(groups.groups.find(g=>g.id === assetGroup(groups,a.id))?.name ?? '未分组')}</small></button><button class="text-button asset-delete" data-delete="${esc(a.id)}" ${deleting ? 'disabled' : ''}>删除</button></article>`).join('') : `<p class="empty-copy">${assets.length ? '当前分组没有素材。可切换到全部并勾选素材后移动至此分组。' : '尚无素材。生成或导入图片后会显示在这里。'}</p>`
  }
  function confirmDelete(ids:string[]) {
    if (deleting || !ids.length) return
    pendingIds = [...ids]
    q<HTMLButtonElement>('[data-confirm-delete]').disabled = false
    q('[data-delete-names]').innerHTML = ids.map(id=>`<li>${esc(deps.getAssets().find(a=>a.id===id)?.name ?? id)}</li>`).join('')
    q('[data-delete-status]').textContent = `将永久删除 ${ids.length} 项素材。`
    deleteDialog.showModal()
  }
  async function performDelete() {
    if (deleting || !pendingIds.length) return
    deleting = true
    q<HTMLButtonElement>('[data-confirm-delete]').disabled = true
    q<HTMLButtonElement>('[data-cancel-delete]').disabled = true
    q('[data-delete-status]').textContent = '正在逐项删除，请勿关闭页面…'
    update()
    try {
      const outcome = await deleteSequential(pendingIds,deps.remove)
      outcome.deleted.forEach(id=>checked.delete(id)); deps.onDeleted(outcome.deleted)
      let refreshError = ''
      try { await deps.refresh() } catch { refreshError = ' 素材库刷新失败，请手动刷新。' }
      update()
      const summary = `已删除 ${outcome.deleted.length} 项，失败 ${outcome.failed.length} 项。${refreshError}`
      q('[data-delete-status]').textContent = summary + (outcome.failed.length ? '\n' + outcome.failed.map(f=>`${deps.getAssets().find(a=>a.id===f.id)?.name ?? f.id}：${f.message}`).join('\n') : '')
      deps.notify(summary,!!outcome.failed.length || !!refreshError)
      pendingIds = []
      if (!outcome.failed.length && !refreshError) deleteDialog.close()
    } finally {
      deleting = false; q<HTMLButtonElement>('[data-confirm-delete]').disabled = !pendingIds.length; q<HTMLButtonElement>('[data-cancel-delete]').disabled = false; update()
    }
  }
  function previewComposite() {
    if (!draft || !Number.isSafeInteger(draft.width) || !Number.isSafeInteger(draft.height) || draft.width < 1 || draft.height < 1) { q('[data-composite]').innerHTML = ''; return }
    const {width,height,layers} = draft
    q('[data-composite]').style.aspectRatio = `${width} / ${height}`
    q('[data-composite]').innerHTML = layers.map((l,i)=> {
      const a = deps.getAssets().find(a=>a.id===l.assetId)
      return a && [l.x,l.y].every(Number.isSafeInteger) ? `<img src="${esc(assetUrl(a.url))}" alt="${esc(l.name)}" style="left:${l.x/width*100}%;top:${l.y/height*100}%;width:${(l.width||a.width)/width*100}%;height:${(l.height||a.height)/height*100}%;z-index:${layers.length-i}">` : ''
    }).join('')
  }
  function renderLayers() {
    if (!draft) return
    q('[data-layers]').innerHTML = draft.layers.map((l,i)=> {
      const a = deps.getAssets().find(a=>a.id===l.assetId)
      return `<div class="psd-layer" data-layer="${i}"><div class="checker psd-thumbnail">${a ? `<img src="${esc(assetUrl(a.url))}" alt="">` : ''}</div><div class="psd-layer-fields"><label>部件组<select data-field="group"><option value="">不分组</option>${[...cubismParts.map(p=>p.name),'未分类'].map(name=>`<option value="${esc(name)}" ${l.group===name?'selected':''}>${esc(name)}${cubismParts.find(p=>p.name===name)?` · ${cubismParts.find(p=>p.name===name)!.order}`:''}</option>`).join('')}</select></label><label>图层 ${i+1} 名称<input data-field="name" maxlength="100" value="${esc(l.name)}"></label><div class="psd-size"><label>X（px）<input type="number" step="1" data-field="x" value="${l.x}"></label><label>Y（px）<input type="number" step="1" data-field="y" value="${l.y}"></label></div><div class="psd-size"><label>输出宽（px）<input type="number" step="1" min="1" max="8192" data-field="width" value="${l.width||a?.width||1}"></label><label>输出高（px）<input type="number" step="1" min="1" max="8192" data-field="height" value="${l.height||a?.height||1}"></label></div><small>${a ? `${a.width} × ${a.height} px · 原始尺寸` : '素材已不存在'}</small></div><div><button class="text-button" data-move="-1" ${i === 0 ? 'disabled' : ''} aria-label="上移图层 ${i+1}">↑ 上移</button><button class="text-button" data-move="1" ${i === draft!.layers.length-1 ? 'disabled' : ''} aria-label="下移图层 ${i+1}">↓ 下移</button></div></div>`
    }).join('')
    previewComposite()
  }
  root.addEventListener('click',event=> {
    const target = (event.target as HTMLElement).closest<HTMLButtonElement>('button'); if (!target) return
    if (target.hasAttribute('data-refresh')) { target.disabled = true; void deps.refresh().catch(e=>deps.notify(e instanceof Error ? e.message : '刷新失败',true)).finally(()=>{target.disabled=false}) }
    if (target.dataset.preview) { const a = deps.getAssets().find(a=>a.id===target.dataset.preview); if(a) deps.preview(a) }
    if ((target.hasAttribute('data-all') || target.hasAttribute('data-invert')) && !deleting) { checked = changeVisibleSelection(checked,visibleAssets().map(a=>a.id),target.hasAttribute('data-invert') ? 'invert' : 'toggle'); update() }
    if (target.hasAttribute('data-delete-group') && !deleting && filter.kind === 'group') {
      if (saveGroups(removeLibraryGroup(groups,filter.id))) { filter = {kind:'ungrouped'}; update(); deps.notify('已删除分组，素材已返回未分组；没有删除素材文件。') }
    }
    if (target.hasAttribute('data-assign-group') && !deleting && checked.size) {
      try { if (saveGroups(assignLibraryGroup(groups,checked,q<HTMLSelectElement>('[data-group-target]').value || null))) { update(); deps.notify(`已移动 ${checked.size} 项素材的浏览器本地分组。`) } }
      catch (error) { deps.notify(error instanceof Error ? error.message : '分组移动失败。',true) }
    }
    if (target.hasAttribute('data-clear') && !deleting) {checked.clear();update()}
    if (target.dataset.delete) confirmDelete([target.dataset.delete])
    if (target.hasAttribute('data-delete-selected')) confirmDelete([...checked])
    if (target.hasAttribute('data-cancel-delete') && !deleting) deleteDialog.close()
    if (target.hasAttribute('data-confirm-delete')) void performDelete().catch(()=>deps.notify('删除操作异常，请刷新确认当前素材状态。',true))
    if (target.hasAttribute('data-export')) {
      const selected = deps.getAssets().filter(a=>checked.has(a.id)); if (!selected.length || selected.length>64) return
      draft = applyCubismTemplate(createPsdDraft(selected),selected); invalidate(); status(['已应用 Cubism 部件方案，可靠坐标已自动继承；参考图置底。',...psdDraftWarnings(draft,selected)].join('\n')); q<HTMLInputElement>('[data-width]').value = String(draft.width); q<HTMLInputElement>('[data-height]').value = String(draft.height); renderLayers(); exportDialog.showModal()
    }
    if (target.hasAttribute('data-close-export') && !exporting) exportDialog.close()
    if (target.dataset.move && draft && !exporting) {
      const index = Number(target.closest<HTMLElement>('[data-layer]')?.dataset.layer), next = index+Number(target.dataset.move)
      if(next >= 0 && next < draft.layers.length) {[draft.layers[index],draft.layers[next]]=[draft.layers[next],draft.layers[index]]; invalidate();renderLayers()}
    }
    if (target.hasAttribute('data-cubism-template') && draft && !exporting) { draft=applyCubismTemplate(draft,deps.getAssets()); invalidate();renderLayers();status('已应用截图部件分类与排序；未生成空白部件或修改原素材。') }
    if (target.hasAttribute('data-restore-position') && draft && !exporting) { const restored=restoreSourcePositions(draft,deps.getAssets());draft=restored.draft;invalidate();renderLayers();status(`已自动恢复 ${restored.restored} 层的已验证原图位置；${restored.unverified} 层缺少可靠坐标或画布不一致，保留原值，请人工调整。重新导出后由 Cubism 导入继承。`) }
    if (target.hasAttribute('data-submit-export')) void generate()
    if (target.hasAttribute('data-open-cubism') && result && !exporting) {
      const exportId = result.id; target.disabled = true; status('正在向本地服务提交打开请求…')
      void deps.post('/l2d/cubism/open-psd',{exportId}).then(()=>status('已通过官方文档启动脚本传递 PSD；请确认 Cubism 的导入对话框。若未出现，请下载 PSD，在 Cubism 用「文件 → 打开」导入。')).catch(e=>status(e instanceof Error ? e.message : '打开请求失败')).finally(()=>{target.disabled=false})
    }
  })
  root.addEventListener('submit',event=> {
    if (!(event.target as HTMLElement).hasAttribute('data-group-form')) return
    event.preventDefault()
    if (deleting) return
    const input = q<HTMLInputElement>('[data-group-name]')
    try {
      const next = addLibraryGroup(groups,crypto.randomUUID(),input.value)
      if (saveGroups(next)) { input.value = ''; update(); deps.notify('分组已创建，仅保存在当前浏览器。') }
    } catch (error) { deps.notify(error instanceof Error ? error.message : '创建分组失败。',true); input.focus() }
  })
  root.addEventListener('change',event=> {
    const input = event.target as HTMLInputElement
    if (input.hasAttribute('data-group-filter')) {
      filter = input.value === 'all' ? {kind:'all'} : input.value === 'ungrouped' ? {kind:'ungrouped'} : {kind:'group',id:input.value}
      update()
    }
    if (input.dataset.check && !deleting) { input.checked ? checked.add(input.dataset.check) : checked.delete(input.dataset.check); update() }
  })
  root.addEventListener('input',event=> {
    const input = event.target as HTMLInputElement; if (!draft || exporting) return
    if(input.hasAttribute('data-width')) draft.width=input.valueAsNumber
    else if(input.hasAttribute('data-height')) draft.height=input.valueAsNumber
    else if(input.dataset.field) {
      const layer=draft.layers[Number(input.closest<HTMLElement>('[data-layer]')?.dataset.layer)]
      if(!layer) return
      if(input.dataset.field === 'name') layer.name=input.value
      else if(input.dataset.field === 'group') layer.group=input.value
      else if(input.dataset.field === 'width' || input.dataset.field === 'height') { const a=deps.getAssets().find(a=>a.id===layer.assetId);layer.width=layer.width||a?.width;layer.height=layer.height||a?.height;layer[input.dataset.field]=input.valueAsNumber }
      else if(input.dataset.field === 'x' || input.dataset.field === 'y') layer[input.dataset.field]=input.valueAsNumber
    } else return
    invalidate(); status('设置已更改，请重新生成 PSD。'); previewComposite()
  })
  async function generate() {
    if (!draft || exporting) return
    const error = validatePsdDraft(draft,deps.getAssets()); if(error) {status(error);return}
    exporting = true; invalidate(); q<HTMLFieldSetElement>('[data-editor]').disabled = true; q<HTMLButtonElement>('[data-submit-export]').disabled = true; status('正在生成 PSD…')
    try {
      const exported = await deps.post<PsdExport>('/oc/export-psd',{...draft,layers:draft.layers.map(l=>({...l,name:l.name.trim()}))})
      const url = psdDownloadUrl(exported)
      result = exported; const link = q<HTMLAnchorElement>('[data-download]'); link.href=url; link.download=`${exported.id}.psd`;link.classList.remove('hidden');q('[data-open-cubism]').classList.remove('hidden')
      status(`PSD 已生成：${exported.width} × ${exported.height} px，${exported.layerCount} 个图层。可下载或提交 Cubism 打开请求。`)
    } catch(e) {status(e instanceof Error ? e.message : 'PSD 导出失败')}
    finally {exporting = false; q<HTMLFieldSetElement>('[data-editor]').disabled=false;q<HTMLButtonElement>('[data-submit-export]').disabled=false;update()}
  }
  deleteDialog.addEventListener('cancel',event=>{if(deleting) event.preventDefault()})
  exportDialog.addEventListener('cancel',event=>{if(exporting) event.preventDefault()})
  update()
  return {update}
}
