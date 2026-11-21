import './live-workflow.scss'
import { mountCubismAutomation } from './cubism-automation'

type LiveWorkflowDeps = {
  post: <T>(path: string, body: unknown) => Promise<T>
  notify: (message: string, error?: boolean) => void
  openSettings: () => void
}
type DiscoveryResult = { candidates: string[]; note: string }

/** Appends the Cubism authoring guide alongside the existing model inspection panel. */
export function mountLiveWorkflow(host: HTMLElement, deps: LiveWorkflowDeps): () => void {
  const root = document.createElement('section')
  root.className = 'live-workflow'
  root.setAttribute('aria-label', 'Live2D 引导工作流')
  root.innerHTML = `
    <header class="lw-heading"><div><span class="lw-kicker">GUIDED LIVE2D WORKFLOW</span><h2>用 Cubism Editor，制作真实的 Live2D 模型</h2><p>先下载并启动官方编辑器完成模型制作，再导出并检查模型结构。</p></div><span class="lw-badge">本地执行 · 不自动绑定</span></header>
    <ol class="lw-steps" aria-label="操作顺序"><li><b>01</b>下载 / 启动 Cubism</li><li><b>02</b>在编辑器制作模型</li><li><b>03</b>导出并检查模型</li></ol>
    <section class="lw-card lw-authoring" aria-label="Cubism Editor 下载与启动"><div class="lw-card-title"><span>首要步骤 / 官方模型编辑器</span><h3>Live2D Cubism Editor · 下载、发现路径与启动</h3></div><p><a href="https://www.live2d.com/zh-CHS/cubism/download/editor/" target="_blank" rel="noopener noreferrer">前往官网下载 Cubism Editor ↗</a>。默认查找项目根目录的 <code>Live2DCubism/</code> 文件夹：请放入官方安装完成后的完整编辑器和依赖，或在设置中使用真实安装路径。本项目不附带编辑器。</p><p class="lw-warning">安装包不等于编辑器；不要把安装程序作为启动路径。此处只发现候选或请求启动已配置的编辑器，不自动操作 Cubism、不制作网格、不绑定参数，也不生成 moc3。</p><div class="lw-actions"><button type="button" data-action="cubism-discover">发现 Cubism 编辑器路径</button><button type="button" data-action="settings">设置编辑器路径</button><button type="button" data-action="cubism-launch" class="lw-primary">启动已配置的 Cubism Editor</button></div><p data-role="cubism-status" class="lw-connection" role="status" aria-live="polite">尚未请求启动。先发现候选，核对后到设置中手动填写并保存。</p><div class="lw-discovery" data-role="cubism-discovery" hidden><h4>Cubism 编辑器候选路径 · 可选中文本复制</h4><p data-role="cubism-discovery-note"></p><ul data-role="cubism-candidates"></ul><p class="lw-hint">发现不会自动保存或启动。请复制确认后的编辑器路径，在设置中手动填写并保存；未发现时可直接使用真实安装路径。</p></div></section>
    <section class="lw-card lw-authoring"><div class="lw-card-title"><span>模型准备 / 编辑器内制作</span><h3>部件 PNG ≠ 可运行的 Live2D 模型</h3></div><p>拆分出的 PNG 需要在 <strong>Cubism Editor</strong> 中整理图层、制作网格与变形器、绑定参数及物理效果，再导出模型、纹理、动作和表情。制作在真正的编辑器中完成，本页不能代替这些步骤。</p><p class="lw-hint">本页不实现自动建模、自动绑定或自动生成 moc3。Cubism 导出的 <code>.model3.json</code> 可用下方模型检查工具检查；结构检查不代表模型已完成绑定或运行效果已验证。</p></section>
    <section class="lw-card lw-feedback"><h3>操作反馈</h3><p data-role="feedback" role="status" aria-live="polite">等待操作。发现路径不会修改配置；启动请求不代表模型已制作完成。</p><details><summary>查看最近一次请求与原始响应</summary><pre data-role="output">尚无请求。</pre></details><p class="lw-hint">请在桌面确认 Cubism Editor 是否就绪。启动超时也可能已执行，请先检查桌面，不要盲目重复启动。导出后可用下方工具检查模型结构。</p></section>`
  host.append(root)
  const disposeAutomation = mountCubismAutomation(root, deps)

  const get = <T extends HTMLElement = HTMLElement>(selector: string): T => root.querySelector<T>(selector)!
  let disposed = false
  let busy = false
  const output = (request: unknown, response: unknown) => {
    if (!disposed) get('[data-role="output"]').textContent = JSON.stringify({ request, response }, null, 2)
  }
  const feedback = (message: string, error = false) => {
    if (disposed) return
    const el = get('[data-role="feedback"]')
    el.textContent = message
    el.classList.toggle('lw-error', error)
    deps.notify(message, error)
  }
  const run = async (task: () => Promise<void>) => {
    if (busy || disposed) return
    busy = true
    const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('button:not([data-action="settings"])'))
    buttons.forEach(button => { button.disabled = true })
    root.setAttribute('aria-busy', 'true')
    get('[data-role="feedback"]').textContent = '正在请求本机服务…'
    get('[data-role="feedback"]').classList.remove('lw-error')
    try { await task() }
    catch (error) {
      const message = error instanceof Error ? error.message : '操作失败，请检查本机服务。'
      feedback(`${message} 尚未确认执行结果；如启动请求超时，请先在本机桌面检查 Cubism Editor，不要盲目重试。`, true)
    } finally {
      busy = false
      if (!disposed) {
        buttons.forEach(button => { button.disabled = false })
        root.removeAttribute('aria-busy')
      }
    }
  }
  const actions: Record<string, () => Promise<void>> = {
    'cubism-discover': async () => {
      const path = '/l2d/cubism/discover'
      const status = get('[data-role="cubism-status"]')
      status.textContent = '正在查找 Cubism 编辑器候选路径…'
      get('[data-role="cubism-discovery"]').hidden = true
      output({ path }, '等待响应…')
      try {
        const result = await deps.post<DiscoveryResult>(path, {})
        output({ path }, result)
        if (!Array.isArray(result?.candidates) || !result.candidates.every(candidate => typeof candidate === 'string') || typeof result.note !== 'string') throw new Error('Cubism 路径发现响应格式异常。')
        const list = get('[data-role="cubism-candidates"]')
        list.replaceChildren()
        result.candidates.forEach(candidate => {
          const li = document.createElement('li')
          const code = document.createElement('code')
          code.textContent = candidate
          li.append(code)
          list.append(li)
        })
        get('[data-role="cubism-discovery-note"]').textContent = result.note
        get('[data-role="cubism-discovery"]').hidden = false
        const message = result.candidates.length ? `发现 ${result.candidates.length} 个 Cubism 候选；请核对并复制到设置中手动保存，未自动修改配置或启动。` : '未发现 Cubism 编辑器候选。请先安装官方编辑器，再在设置中填写真实安装路径；安装包不能作为编辑器启动。'
        status.textContent = message
        feedback(message)
      } catch (error) {
        status.textContent = 'Cubism 路径发现失败；未修改配置或请求启动。请检查服务后重试，或手动填写已安装的编辑器路径。'
        throw error
      }
    },
    'cubism-launch': async () => {
      const path = '/l2d/cubism/launch'
      const status = get('[data-role="cubism-status"]')
      status.textContent = '正在请求启动已保存路径对应的 Cubism Editor…'
      output({ path }, '等待响应…')
      try {
        const result = await deps.post<{ status: 'started'; note: string }>(path, {})
        output({ path }, result)
        if (result?.status !== 'started' || typeof result.note !== 'string') throw new Error('Cubism 启动响应格式异常，不能确认启动状态。')
        const message = `本机服务报告 Cubism 启动请求已发出（started）。${result.note ? ` ${result.note}` : ''} 请在桌面确认编辑器是否就绪；这不代表模型已打开、网格已制作、绑定已完成或 moc3 已生成。`
        status.textContent = message
        feedback(message)
      } catch (error) {
        status.textContent = '未确认 Cubism 启动结果。请核对设置与本机桌面；请求超时也可能已启动，不要盲目重复启动。'
        throw error
      }
    },
  }
  root.querySelectorAll<HTMLButtonElement>('[data-action]').forEach(button => {
    button.addEventListener('click', () => {
      const action = button.dataset.action!
      if (action === 'settings') { deps.openSettings(); return }
      void run(actions[action])
    })
  })
  return () => { disposed = true; disposeAutomation(); root.remove() }
}
