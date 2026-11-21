type Deps = {post:<T>(path:string,body:unknown)=>Promise<T>;notify:(message:string,error?:boolean)=>void}
export function mountCubismAutomation(host:HTMLElement,deps:Deps):()=>void {
 const root=document.createElement('section');root.className='lw-card';root.innerHTML=`<h3>Cubism 官方外部 API · 自动化联动</h3><p>在 Cubism 的外部应用程序联动设置中启用 API 服务，首次连接后允许「人人皆可建模」访问，然后重新检测授权。</p><label>本机 WebSocket 地址 <input data-role="endpoint" value="ws://127.0.0.1:22033" aria-label="Cubism API 地址"></label><div class="lw-actions"><button data-cmd="connect">连接 / 检测授权</button><button data-cmd="GetDocuments">读取文档</button><button data-cmd="GetCurrentModelUID">读取当前模型 UID</button><button data-cmd="disconnect">断开</button></div><label>模型 UID（连接期间有效）<input data-role="model" aria-label="模型 UID"></label><div class="lw-actions"><button data-cmd="GetParameters">读取参数定义</button><button data-cmd="GetParameterValues">读取参数当前值</button></div><label>参数 ID <input data-role="parameter" placeholder="ParamAngleX" aria-label="参数 ID"></label><label>预览数值 <input type="number" data-role="value" value="0" step="0.1" aria-label="参数预览数值"></label><div class="lw-actions"><button data-cmd="SetParameterValues">应用临时参数预览</button><button data-cmd="ClearParameterValues">清除临时预览</button></div><p class="lw-warning">参数预览不是永久关键帧，官方临时缓冲区约 0.5 秒后清除。此联动不自动导入 PSD、不自动生成精细网格、绑定或 moc3；PSD 导出后请在编辑器中确认导入选项。</p><p data-role="status" role="status">尚未连接。</p><pre data-role="output" style="white-space:pre-wrap;overflow-wrap:anywhere;max-height:320px;overflow:auto"></pre>`;host.append(root)
 const get=<T extends HTMLElement>(role:string)=>root.querySelector<T>(`[data-role="${role}"]`)!
 let busy=false,disposed=false
 root.addEventListener('click',async event=>{
 const button=(event.target as HTMLElement).closest<HTMLButtonElement>('button[data-cmd]');if(!button||busy)return
 busy=true;root.querySelectorAll('button').forEach(b=>b.disabled=true)
 try {
 const cmd=button.dataset.cmd!;let result:unknown
 if(cmd==='connect') {const status=await deps.post<{connected:boolean;approved:boolean;note:string}>('/l2d/cubism/connect',{endpoint:get<HTMLInputElement>('endpoint').value.trim()});result=status;get('status').textContent=`${status.connected?'已连接':'未连接'} · ${status.approved?'已授权':'尚未授权'} · ${status.note}`}
 else if(cmd==='disconnect'){result=await deps.post('/l2d/cubism/disconnect',{});get('status').textContent='已断开。'}
 else {
 const data:Record<string,unknown>={};if(!['GetDocuments','GetCurrentModelUID'].includes(cmd)){const uid=get<HTMLInputElement>('model').value.trim();if(!uid)throw new Error('请先读取当前模型 UID。');data.ModelUID=uid}
 if(cmd==='SetParameterValues'){const id=get<HTMLInputElement>('parameter').value.trim(),value=Number(get<HTMLInputElement>('value').value);if(!id||!Number.isFinite(value))throw new Error('请输入参数 ID 和有限数值。');data.Parameters=[{Id:id,Value:value}]}
 result=await deps.post('/l2d/cubism/command',{method:cmd,data})
 const response=result as {Data?:{ModelUID?:string};ModelUID?:string};const uid=response?.Data?.ModelUID??response?.ModelUID;if(typeof uid==='string')get<HTMLInputElement>('model').value=uid
 get('status').textContent=cmd==='SetParameterValues'?'临时预览请求已响应，未保存关键帧。':'Cubism 请求已响应。'
 }
 if(!disposed)get('output').textContent=JSON.stringify(result,null,2)
 }catch(error){const message=error instanceof Error?error.message:'Cubism 操作失败';if(!disposed)get('status').textContent=message;deps.notify(message,true)}finally{busy=false;if(!disposed)root.querySelectorAll('button').forEach(b=>b.disabled=false)}
 });return()=>{disposed=true;root.remove()}
}
