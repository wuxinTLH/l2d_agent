import './live-workflow.scss'

type LiveWorkflowDeps = {
  post: <T>(path: string, body: unknown) => Promise<T>
  notify: (message: string, error?: boolean) => void
  openSettings: () => void
}
type ProbeResult = { connected: boolean; endpoint: string; error?: string }
type DiscoveryResult = { candidates: string[]; note: string }
type SentResult = { status: string; verified: boolean }

/** Appends the guided controls without replacing the existing inspection/raw-command panels. */
export function mountLiveWorkflow(host: HTMLElement, deps: LiveWorkflowDeps): () => void {
  const root = document.createElement('section')
  root.className = 'live-workflow'
  root.setAttribute('aria-label', 'Live2D 引导工作流')
  root.innerHTML = `
    <header class="lw-heading"><div><span class="lw-kicker">GUIDED LIVE2D WORKFLOW</span><h2>从模型，到桌面上的角色</h2><p>先完成模型制作，再连接 ViewerEX，最后发送动作与音频。</p></div><span class="lw-badge">本地执行 · 不自动绑定</span></header>
    <ol class="lw-steps" aria-label="操作顺序"><li><b>01</b>制作并导出模型</li><li><b>02</b>配置与连接</li><li><b>03</b>加载模型</li><li><b>04</b>动作与声音</li></ol>
    <section class="lw-card lw-authoring"><div class="lw-card-title"><span>01 / 模型准备</span><h3>部件 PNG ≠ 可运行的 Live2D 模型</h3></div><p>拆分出的 PNG 需要在 <strong>Cubism Editor</strong> 中整理图层、制作网格与变形器、绑定参数及物理效果，再导出模型、纹理、动作和表情。在 ViewerEX 中导入、配置后，取得它使用的 <code>.model.json</code> 配置文件。</p><p class="lw-hint">本页不实现自动建模、自动绑定或自动生成 moc3。Cubism 导出的 <code>.model3.json</code> 可用下方现有模型检查工具检查，但不等同于此处加载命令所需的 ViewerEX 配置。</p></section>
    <section class="lw-card"><div class="lw-card-title"><span>02 / 连接 ViewerEX</span><h3>配置一次，启动后验证真实回显</h3></div><p>先在设置中保存 ViewerEX 可执行文件与 EXAPI 地址，并在 ViewerEX 中启用对应的 EXAPI 服务。发现路径只提供候选建议，不会自动保存或启动。</p>
      <div class="lw-actions"><button type="button" data-action="settings">打开设置</button><button type="button" data-action="discover">发现 exe 路径建议</button><button type="button" data-action="launch">启动已配置的 ViewerEX</button><button type="button" data-action="probe" class="lw-primary">探测 EXAPI 连接</button><button type="button" data-action="echo">注册回显测试 · 10000</button></div>
      <div class="lw-connection" data-role="connection" role="status" aria-live="polite">尚未探测；已配置或已启动都不代表 EXAPI 已连接。</div>
      <div class="lw-discovery" data-role="discovery" hidden><h4>可执行文件候选</h4><p data-role="discovery-note"></p><ul data-role="candidates"></ul><p class="lw-hint">请核对候选路径，复制后到设置中手动保存。未发现时，也可直接填写已知的安装路径。</p></div>
    </section>
    <section class="lw-card"><div class="lw-card-title"><span>03 / 加载模型</span><h3>选择槽位，发送 ViewerEX 配置路径</h3></div>
      <label class="lw-slot">模型槽位 id<input data-field="slot" type="number" min="0" max="7" step="1" value="0" required><small>0–7 的整数；以下所有操作使用此槽位。</small></label>
      <form data-form="load"><label>ViewerEX 模型配置 · 绝对路径<input data-field="model" type="text" placeholder="D:/Models/Character/character.model.json" spellcheck="false" required></label><p class="lw-hint">msg 13000 · 文件须为 ViewerEX 使用的 .model.json；请勿直接填写 PNG、moc3 或 Cubism .model3.json。浏览器不会上传文件，路径由本机 ViewerEX 读取。</p><button type="submit" class="lw-primary">加载到所选槽位 · 13000</button></form>
    </section>
    <section class="lw-card"><div class="lw-card-title"><span>04 / 预览动作与表情</span><h3>使用模型已有的动作和表情</h3></div><div class="lw-form-grid">
      <form data-form="motion"><h4>触发动作</h4><label>动作来源<select data-field="motion-type"><option value="0">模型内动作 · group:name</option><option value="1">动作文件 · 绝对路径</option></select></label><label><span data-role="motion-label">动作组与名称</span><input data-field="motion" type="text" placeholder="Idle:idle_01" spellcheck="false" required></label><p class="lw-hint">模型内动作按 group:name 填写。文件模式填写你已有的动作文件绝对路径；不会生成动作。</p><button type="submit">触发动作 · 13200</button></form>
      <form data-form="expression"><h4>切换表情</h4><label>表情 expId<input data-field="expression" type="number" min="0" step="1" placeholder="0" required></label><p class="lw-hint">填写当前模型已有表情的非负整数索引。此处不能枚举或创建表情，也不能确认索引存在。</p><button type="submit">切换表情 · 13300</button></form>
    </div></section>
    <section class="lw-card"><div class="lw-card-title"><span>05 / 音频与口型</span><h3>播放本地音频，观察模型口部响应</h3></div><form data-form="audio"><label>音频文件 · 绝对路径<input data-field="audio" type="text" placeholder="D:/Audio/voice.wav" spellcheck="false" required></label><p class="lw-hint">支持 wav / mp3 / ogg。固定通道 0、音量 1、延迟 0、不循环。播放前请确认本地音量。</p><div class="lw-actions"><button type="submit" class="lw-primary">播放音频 · 13500</button><button type="button" data-action="stop">停止通道 0 · 13501</button></div></form><p class="lw-warning">音量驱动的嘴部开合控制器只反映声音强弱，不是音素级 viseme 口型同步。需要模型已绑定可用的嘴部参数，并在 ViewerEX 中配置相应控制器；本页不能自动绑定 moc3，也不会凭空添加口型。</p></section>
    <section class="lw-card lw-feedback"><h3>操作反馈</h3><p data-role="feedback" role="status" aria-live="polite">等待操作。动作、表情、加载、音频的 sent / verified:false 仅表示写入连接，不代表 ViewerEX 已执行。</p><details><summary>查看最近一次请求与原始响应</summary><pre data-role="output">尚无请求。</pre></details><p class="lw-hint">请在 ViewerEX 中目视或试听确认结果。超时也可能已送达，不要盲目重复发送。下方保留原有的模型检查与原始 JSON 命令工具。</p></section>`
  host.append(root)

  const get = <T extends HTMLElement = HTMLElement>(selector: string): T => root.querySelector<T>(selector)!
  const field = (name: string) => get<HTMLInputElement | HTMLSelectElement>(`[data-field="${name}"]`)
  const value = (name: string) => field(name).value.trim()
  let disposed = false
  let busy = false
  let messageId = Date.now() % 1_000_000_000
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
  const integer = (name: string, label: string, max = Number.MAX_SAFE_INTEGER) => {
    const text = value(name)
    const n = Number(text)
    if (!/^\d+$/.test(text) || !Number.isSafeInteger(n) || n < 0 || n > max) {
      field(name).focus()
      throw new Error(`${label}必须为 0–${max} 的整数。`)
    }
    return n
  }
  const absolutePath = (name: string, label: string, extension?: RegExp) => {
    const path = value(name)
    const normalized = path.replace(/\\/g, '/')
    // Require a drive-rooted path or a complete UNC server/share/file path, never C:relative.
    const drive = /^[a-z]:\//i.test(normalized)
    const unc = /^\/\/[^/]+\/[^/]+\/.+/.test(normalized)
    const tail = drive ? normalized.slice(3) : normalized.slice(2)
    if ((!drive && !unc) || !tail || /[<>:"|?*\x00-\x1f]/.test(tail) || normalized.endsWith('/')) {
      field(name).focus()
      throw new Error(`${label}须填写 Windows 绝对文件路径，例如 D:/Models/character.model.json；不要添加引号。`)
    }
    if (extension && !extension.test(path)) {
      field(name).focus()
      throw new Error(name === 'model' ? '加载需要 ViewerEX 的 .model.json 配置，不是 Cubism .model3.json 或 moc3。' : '请选择 .wav、.mp3 或 .ogg 音频文件。')
    }
    return path
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
      feedback(`${message} 尚未确认执行结果；如请求超时，请先在 ViewerEX 中检查。`, true)
    } finally {
      busy = false
      if (!disposed) {
        buttons.forEach(button => { button.disabled = false })
        root.removeAttribute('aria-busy')
      }
    }
  }
  const command = async (msg: number, data: unknown, label: string) => {
    const message = { msg, msgId: ++messageId, data }
    output(message, '等待响应…')
    const response = await deps.post<SentResult>('/l2d/command', { message })
    output(message, response)
    if (response?.status !== 'sent' || response.verified !== false) throw new Error('命令响应格式异常，不能确认发送状态。')
    feedback(`${label}：已发送，未验证（sent / verified:false）。请在 ViewerEX 中确认实际效果。`)
  }
  const actions: Record<string, () => Promise<void>> = {
    discover: async () => {
      output({ path: '/l2d/discover' }, '等待响应…')
      const result = await deps.post<DiscoveryResult>('/l2d/discover', {})
      output({ path: '/l2d/discover' }, result)
      if (!Array.isArray(result?.candidates) || !result.candidates.every(path => typeof path === 'string')) throw new Error('路径发现响应格式异常。')
      get('[data-role="discovery"]').hidden = false
      get('[data-role="discovery-note"]').textContent = result.note || '请核对以下候选。'
      const list = get('[data-role="candidates"]')
      list.replaceChildren()
      result.candidates.forEach(path => {
        const li = document.createElement('li')
        const code = document.createElement('code')
        code.textContent = path
        li.append(code)
        list.append(li)
      })
      feedback(result.candidates.length ? `发现 ${result.candidates.length} 个候选；未自动保存，请到设置中确认。` : '未发现候选。请在设置中手动填写 ViewerEX 的 exe 绝对路径。')
    },
    launch: async () => {
      output({ path: '/l2d/launch' }, '等待响应…')
      const response = await deps.post<unknown>('/l2d/launch', {})
      output({ path: '/l2d/launch' }, response)
      get('[data-role="connection"]').textContent = '启动请求已返回，EXAPI 连接仍待探测。请等待 ViewerEX 就绪后点击探测。'
      feedback('启动请求已返回；不代表模型已加载或 EXAPI 已连接。请继续探测连接。')
    },
    probe: async () => {
      const status = get('[data-role="connection"]')
      status.textContent = '正在探测 EXAPI，等待实际回显…'
      status.classList.remove('lw-connected')
      output({ path: '/l2d/probe' }, '等待响应…')
      try {
        const result = await deps.post<ProbeResult>('/l2d/probe', {})
        output({ path: '/l2d/probe' }, result)
        if (typeof result?.connected !== 'boolean' || typeof result.endpoint !== 'string') throw new Error('连接探测响应格式异常。')
        status.classList.toggle('lw-connected', result.connected)
        status.textContent = result.connected ? `探测时已收到 EXAPI 回显 · ${result.endpoint}（不代表模型或动作已验证）` : `未确认 EXAPI 连接 · ${result.endpoint}${result.error ? ` · ${result.error}` : ''}`
        feedback(result.connected ? '已收到 EXAPI 探测回显。连接状态仅代表本次探测时刻。' : '未收到有效探测回显，请检查 ViewerEX、EXAPI 开关与地址。', !result.connected)
      } catch (error) {
        status.textContent = '探测失败，连接状态未知；请检查服务与设置后重新探测。'
        throw error
      }
    },
    echo: async () => {
      const message = { msg: 10000, msgId: ++messageId }
      output(message, '等待关联回显…')
      const response = await deps.post<unknown>('/l2d/command', { message })
      output(message, response)
      // The backend waits for the correlated msg10000 reply and returns it raw.
      feedback('注册回显请求已返回；请展开原始响应核对。此测试不验证模型、动作或口型。')
    },
    stop: () => command(13501, { id: integer('slot', '模型槽位', 7), channel: 0 }, '停止音频'),
  }
  root.querySelectorAll<HTMLButtonElement>('[data-action]').forEach(button => {
    button.addEventListener('click', () => {
      const action = button.dataset.action!
      if (action === 'settings') { deps.openSettings(); return }
      void run(actions[action])
    })
  })
  const forms: Record<string, () => Promise<void>> = {
    load: () => command(13000, { id: integer('slot', '模型槽位', 7), file: absolutePath('model', '模型配置', /\.model\.json$/i) }, '加载模型'),
    motion: () => {
      const id = integer('slot', '模型槽位', 7)
      const type = integer('motion-type', '动作来源', 1)
      const mtn = type === 1 ? absolutePath('motion', '动作文件') : value('motion')
      if (type === 0 && !/^[^:\s][^:]*:[^:\s][^:]*$/.test(mtn)) {
        field('motion').focus()
        throw new Error('模型内动作请填写 group:name，冒号两侧均须有名称。')
      }
      return command(13200, { id, type, mtn }, '触发动作')
    },
    expression: () => command(13300, { id: integer('slot', '模型槽位', 7), expId: integer('expression', '表情索引') }, '切换表情'),
    audio: () => command(13500, { id: integer('slot', '模型槽位', 7), channel: 0, volume: 1, delay: 0, loop: false, type: 0, sound: absolutePath('audio', '音频文件', /\.(wav|mp3|ogg)$/i) }, '播放音频'),
  }
  root.querySelectorAll<HTMLFormElement>('[data-form]').forEach(form => {
    form.addEventListener('submit', event => {
      event.preventDefault()
      void run(forms[form.dataset.form!])
    })
  })
  field('motion-type').addEventListener('change', () => {
    const fileMode = value('motion-type') === '1'
    get('[data-role="motion-label"]').textContent = fileMode ? '动作文件绝对路径' : '动作组与名称'
    field('motion').setAttribute('placeholder', fileMode ? 'D:/Models/Character/idle.motion3.json' : 'Idle:idle_01')
    field('motion').value = ''
  })
  return () => { disposed = true; root.remove() }
}
