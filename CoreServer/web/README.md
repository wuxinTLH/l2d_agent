# 人人皆可建模 · 本地 OC / Live2D 工作室前端

Vite + vanilla TypeScript + SCSS + Tailwind CSS v4（`@tailwindcss/vite`）。仅实现前端，不自建服务、不内置假成功或示例素材。

## 本地开发

需要支持 Vite 6 的 Node.js（推荐 Node.js 22.18+，测试使用原生 TypeScript strip）。

```sh
npm install
npm test
npm run build
# 用户需要时自行运行；本次实现未启动服务器
npm run dev
```

开发代理将 `/api` 与 `/assets` 转发至 `http://127.0.0.1:17823`。生产构建位于 `dist/`，由 Go 服务或同源反向代理提供；不要以 `file://` 直接打开。

## 功能边界

- 中文暗色响应式工作室：生成、导入、预览、基于当前素材编辑、本地素材库。
- 使用原图**整数像素**坐标，左上角为 `(0,0)`；可正反方向拖拽矩形，也可通过表单输入数值。保存前验证唯一名称、正尺寸和图片边界。
- 手动矩形裁切不是语义分割。透明化、去背景、修边和被遮挡部位补绘需另行处理。
- Live2D 模型检查、ViewerEX 启动与可选 EXAPI 原始 JSON 对象转发。配置状态不代表已连接；收到返回也不等于动作执行成功。
- 不提供自动绑定、`.moc3` 生成或 Cubism Editor 替代能力。
- API Key 只在保存请求中发送，提交后立即清空输入；不写 localStorage/sessionStorage，也不接收或显示已保存密钥。配置读取只依赖 `hasImageKey`。

## 精确 API 契约

所有接口返回 JSON；非 2xx 进入失败提示。普通错误可为 `{error:string}` 或 `{message:string}`。配置失败不回显服务错误正文，防止密钥泄漏。

| 方法 / 路径 | 请求 | 预期成功响应 |
|---|---|---|
| GET `/api/health` | 无 | `{status:"ok"\|"healthy",modules:{oc:unknown,l2d:unknown}}` |
| GET `/api/config` | 无 | `{imageBaseUrl,imageModel,hasImageKey,viewerExecutable,exapiUrl}` |
| PUT `/api/config` | JSON `{imageBaseUrl,imageModel,imageApiKey?,viewerExecutable,exapiUrl}`；空密钥时**省略**字段，保留原值 | 任意 JSON 或空正文；随后重新 GET 配置 |
| GET `/api/assets` | 无 | `Asset[]`，不是外层包裹对象 |
| POST `/api/oc/generate` | JSON `{prompt,negativePrompt,size}`；size 为 `1024x1536` / `1024x1024` / `1536x1024` | `Asset` |
| POST `/api/oc/import` | multipart `image:File`（PNG/JPEG） | `Asset` |
| POST `/api/oc/edit` | multipart `sourceId:string,prompt:string`（界面先导入文件，再以 sourceId 编辑） | 新的 `Asset` |
| POST `/api/oc/split` | JSON `{assetId,parts:[{name,x,y,width,height}]}`，整数原图像素 | `{id,parts:Asset[],manifestUrl}` |
| GET `/api/l2d/status` | 无 | `{configured,exapiUrl,capabilities:unknown[],note}` |
| POST `/api/l2d/inspect` | JSON `{modelPath}`，本机绝对路径 | 任意模型检查 JSON，在文本区域原样格式化 |
| POST `/api/l2d/launch` | JSON `{}` | `{status:string}`；仅显示服务返回的状态，不宣布模型已加载 |
| POST `/api/l2d/command` | JSON `{message:object}`，不能是 null/数组 | 任意 JSON，显示原始响应，不假定执行成功 |

`Asset = {id:string,name:string,url:string,width:number,height:number,createdAt:string}`。图片与 manifest URL 由后端返回，支持同源相对 URL 与 HTTP(S) 绝对 URL；拒绝非 HTTP(S) 协议。图片尺寸必须为正，裁切使用这些原图尺寸。库读取不会自动选择或生成任何假素材。

EXAPI 默认地址提示为 `ws://127.0.0.1:10086/api`，以实际配置为准。官方注册回显对象可用于连接测试：`{"msg":10000,"msgId":1}`。只有 msg 10000 等待关联回显，其他消息仅返回 `{status:"sent",verified:false,note:...}`，表示已写入连接，不是动作成功。超时可能已经送达，切勿盲目重复执行。模型检查针对 Cubism `.model3.json`，EXAPI 加载所需 ViewerEX `.model.json` 配置不保证等同于直接加载 Cubism 文件。

一键 API 拆分通过 `POST /api/oc/auto-split` 发送 `{assetId,parts:string[]}`，返回 `{id,parts:Asset[],failures:[{name,error}],manifestUrl,note}`。每个名称一次独立的图片编辑请求，默认 10 个（左右眼睛、头、左右手、嘴、身体、左右耳朵、头发），最多 12 个；没有自动重试。透明 / 近白空图检测不等于语义准确性验证。结果的原图位置与遮挡关系需人工检查。

前端 API 拆分请求超时 40 分钟（最多 12 次顺序调用），其他 OC 请求超时 240 秒，其他请求 20 秒；超时明确提示后台可能仍在处理，建议先刷新核对再重试。

## 验证

`npm test` 覆盖正向/反向拖拽、四周边界、单击最小矩形、合法边缘与非法/重复/非整数坐标。`npm run build` 执行严格 TypeScript 检查与生产打包。真实图像供应商、ViewerEX、本地后端全链路与浏览器视觉验收需运行相应真实环境；构建成功不代替这些验证。
