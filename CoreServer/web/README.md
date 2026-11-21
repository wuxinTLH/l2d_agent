# 人人皆可建模 · 前端

Vite + vanilla TypeScript + SCSS + Tailwind CSS v4。版本 v0.0.1a。

```sh
npm install
npm test
npm run build
npm run dev
```

需要 Node.js 22.18+。开发代理将 `/api` 和 `/assets` 转发到 http://127.0.0.1:17823 。生产前端由 Go 服务提供。

## 功能

- OC 生成、导入、编辑和素材库；API Key 提交后清空，不保存于浏览器存储、不回显。
- 整数原图坐标裁切、移动、八手柄缩放、撤销重做、滚轮缩放、Ctrl/空格/中键拖动平移。
- 一键 API 拆分默认 10 部件，最多 12 个；按部件请求图片编辑，提前显示费用次数，无自动重试。失败与透明/近白图单独提示，保留原图。
- Cubism Editor 官方下载、路径发现、已配置编辑器启动和 `.model3.json` 引用检查。
- 不自动制作网格、绑定参数或生成 moc3；进程启动不等于编辑器就绪。

## API

| 路径 | 契约 |
|---|---|
| GET `/api/config` | `{imageBaseUrl,imageModel,hasImageKey,cubismExecutable}` |
| PUT `/api/config` | `{imageBaseUrl,imageModel,imageApiKey?,cubismExecutable?}`；省略 Key 保留旧值 |
| GET `/api/assets` | `Asset[]` |
| POST `/api/oc/generate` | `{prompt,negativePrompt,size}` → Asset |
| POST `/api/oc/import` | multipart image → Asset |
| POST `/api/oc/edit` | multipart sourceId,prompt → Asset |
| POST `/api/oc/split` | `{assetId,parts:[{name,x,y,width,height}]}` → `{id,parts,manifestUrl}` |
| POST `/api/oc/auto-split` | `{assetId,parts:string[]}` → `{id,parts,failures,manifestUrl,note}` |
| POST `/api/l2d/cubism/discover` | `{}` → `{candidates,note}` |
| POST `/api/l2d/cubism/launch` | `{}` → `{status,note}` |
| POST `/api/l2d/inspect` | `{modelPath}` → 模型引用检查结果 |

`Asset={id,name,url,width,height,createdAt}`。图片 URL 须为同源相对路径或 HTTP(S)。拆分等待上限 40 分钟，其他 OC 请求 240 秒，其他请求 20 秒；超时可能仍在处理，先核对结果再重试。

测试包含坐标边界、拖动方向、缩放变换和几何约束；严格 TypeScript 检查随构建运行。真实付费 API 与编辑器端到端能力需另行验证。
