# 人人皆可建模 · l2d_agent

当前版本：**v0.0.2a**（npm 元数据为兼容 SemVer 的 `0.0.2-a`）。

仅本机运行的人设素材与 Live2D Cubism Editor 工作台。后端 Go，前端 Vite + TypeScript + SCSS + Tailwind CSS。没有云端部署、账号系统或 Blender/Unity 实现。调用远程图片 API 需要联网，可能产生费用。

## 模块边界

- `oc/`：图片 API、PNG/JPEG 导入、素材库、矩形裁切、API 部件拆分及来源清单。
- `l2d/`：Cubism Editor 路径发现与启动、现有 Cubism 导出资源引用检查。
- `CoreServer/`：Go 本地 HTTP 协调层及 `web/` GUI。
- `cmd/studio`、`cmd/oc`、`cmd/l2d`：工作台与独立命令入口。

## 本地构建与启动

需要 Go 1.24+、Node.js 22.12+ 和 npm。Windows Git Bash：

```bash
bash scripts/build.sh
bash scripts/start.sh
```

打开 http://127.0.0.1:17823 。也可从项目根目录运行 `./bin/studio.exe`。前端不内嵌，需要保留 `CoreServer/web/dist` 或通过 `-web` 指定路径。

```bash
go run ./cmd/studio -addr 127.0.0.1:17823 -data .local -web CoreServer/web/dist
# 前端开发另开终端
npm --prefix CoreServer/web run dev
```

只允许 loopback IP 监听。Vite 将 `/api`、`/assets` 代理到 17823。默认 `.local/` 存配置及素材，禁止上传。不配置远程 API 仍可导入、浏览、裁切和检查已有模型。

## OC 工作流

1. 设置 API Base URL（例如 `https://api.openai.com/v1`）、模型和 Key。供应商须支持 OpenAI 风格 `POST /images/generations` 与 multipart `POST /images/edits`；不保证所有兼容供应商支持。
2. 生成或导入 PNG/JPEG。负面提示合并进 `Avoid:` 文本。整图编辑不能保证身份、姿态和未指定像素完全不变，不支持蒙版编辑。
3. 手动裁切支持矩形/选择/平移、指针锚定滚轮缩放、适应画布、100%、移动、八手柄缩放、撤销重做和数值输入。Ctrl、空格或中键拖动平移。坐标始终为原图整数像素，导出 PNG 及坐标 manifest。
4. 一键 API 拆分每个部件对原图发起一次编辑请求，目标为隔离、去背景及遮挡补绘。默认 10 部件：左右眼睛、头、左右手、嘴、身体、左右耳朵、头发，左右以角色自身为准；最多 12 个。最多 3 个部件并发请求，输出仍按请求顺序排列。请求数量提前显示，可能计费，不自动重试。失败后可点击「仅重试失败部位」，锁定原任务原图和定位框，不重复请求已成功部位；重试可能再次计费。网络响应未知不推断失败清单，请先核对服务商记录和素材库。
5. 拆分前可在原图画同名部件框（名称必须与 API 名称一致）；通过真实 alpha 检查后，将生成部件透明包围盒等比缩放居中放入指定框，输出原图大小的透明画布。无框时要求供应商保留原画布尺寸，但不能证明位置正确。优先本地 AI 处理不透明背景；本地处理失败或蒙版不可靠时保留原始生成图，标记待手动处理，不因透明化失败丢弃部件。真正的空图和上游请求失败仍单独列出，不删除原图或已有部件。支持棋盘格/深色/浅色预览及 manifest 下载。
6. AI 拆分为生成式重绘，可能改变身份、细节、透明度和对齐。manifest 不伪造坐标。必须人工检查补绘、图层和遮挡关系后再建模。

供应商响应支持 `data[0].b64_json` 或 HTTPS `data[0].url`。下载不携带 API Key且禁止重定向。单图限制 32 MiB / 4000 万像素。单次供应商请求超时 180 秒；超时应核对结果，不要盲目重发付费请求。

Key 不回显、不写浏览器存储，但**明文保存于本机 `.local/config.json`**。Go 写入权限为 0600，Windows 隔离由实际 ACL 决定。不要共享或提交该目录。配置 API 省略 Key 保留旧值，显式空字符串清除。

## Cubism 部件方案与位置

PSD 面板默认使用参考方案：草图、脸、眼睛、眼球、眉毛、嘴、鼻子、耳朵、前发、侧发、后发、脖子、身体、背景、[参考图]。有素材的分类导出为 PSD 文件夹；未知素材放入未分类，不虚构空部件。前发700、五官600、脸/耳朵/侧发500、身体/脖子400、后发300、背景0用于默认层级排序，不表示已设置 Cubism 的数值绘制顺序。参考图保留在底层，避免遮挡可编辑部件。

导出前自动继承部件的原图坐标；「自动恢复原图位置」只恢复 verifiedScale 为真且源画布与当前画布相同的部件。其他旧素材保留手动位置并提示，不能凭名称或抠图准确找回原图位置。PSD 导入后由 Cubism 继承这些像素坐标，不是通过外部 API 修改已导入 ArtMesh；现有联动不支持该操作。

## 素材删除与分层 PSD

素材库支持分组筛选、选中素材批量分组、多选、当前分组全选/取消全选和反选。分组信息保存在当前浏览器的 localStorage 中，不移动或复制图片；更换浏览器/来源或清除网站数据不会保留分组。删除分组仅解除归类，不删除素材。切换分组保留其他组已选项，批量导出/删除会作用于全部已选项，请核对总选择数量。

素材库支持单项/批量删除（界面二次确认）。只删除所选素材的 PNG 和索引，不级联删除其他部件或已导出的 PSD；旧 manifest 可能引用已删除素材，请先备份。

勾选图片后打开 PSD 图层面板，设置画布、图层名称、顺序（列表首项为最上层）和像素 X/Y。导出 RGB 8 位 PSD，保留各图层透明度、Unicode 名称、位置及合并预览。支持手动填写每层输出宽高（只缩放导出副本），调整坐标及层序；未知比例的 AI 补绘部件仍需人工校正。限制 64 图层、画布每边 8192px / 4000 万像素，并限制解码总量和输出大小。

- `DELETE /api/assets/{id}`：删除单个素材。
- `POST /api/oc/export-psd`：`{width,height,layers:[{assetId,name,x,y}]}` → `{id,url,width,height,layerCount}`。
- `GET /exports/{id}.psd`：下载已导出的 PSD，保存于本机 `.local/assets/exports/`。
- `POST /api/l2d/cubism/open-psd`：`{exportId}`。Windows 使用与配置 EXE 同目录同名的官方 `.bat`（已安装 5.3.05 的脚本会将首个文件参数转交 Java 编辑器），不再向可能忽略参数的 EXE 直接传文件；缺少脚本会报错并提示手动打开。拒绝可能被 cmd 展开的危险路径字符。仍只能确认启动请求，不能绕过 Cubism 新建模型/替换确认；若未弹出导入框，下载后用 Cubism「文件→打开」选择 PSD。

[官方 PSD 导入说明](https://docs.live2d.com/en/cubism-editor-manual/psd-import/)。PSD 导入会创建初始 ArtMesh，不代表精细网格或参数绑定已完成。

## Live2D Cubism Editor

从 [Cubism 官方下载页](https://www.live2d.com/zh-CHS/cubism/download/editor/) 阅读许可并下载安装。启动工作室创建 `Live2DCubism/`。安装包可以保存于此，但安装包不是编辑器；设置须填写已安装的 `CubismEditor*.exe`。保留完整依赖，仓库不含软件本体或激活工具。

工作流：整理分层素材 → Cubism 内制作网格、变形器、参数和物理 → 导出模型。工作台不自动绑定、不自动生成 moc3。

- `POST /api/l2d/cubism/discover`：有限安装目录内发现候选，不自动保存或启动。
- `POST /api/l2d/cubism/launch`：使用 `cubismExecutable` 配置启动编辑器，仅确认创建进程，不确认许可证、就绪或项目状态。
- `POST /api/l2d/inspect`：`{modelPath}` 检查已有 `.model3.json` 中引用文件的存在、非空和安全相对路径；不解析 moc3 二进制或验证纹理、动画质量。

### 官方外部 API 联动

Cubism 的「外部应用程序联动」启用服务后，在工作台连接默认 `ws://127.0.0.1:22033`。首次须在编辑器允许插件访问，再检测授权。工作台保持连接，支持读取文档、当前模型 UID、参数定义/当前值以及临时参数预览、清除预览。UID 仅当前连接有效；重连后重新读取。参数预览不创建持久关键帧，官方缓冲区约 0.5 秒后丢弃。

接口为 `POST /api/l2d/cubism/connect {endpoint?}`、`/command {method,data}`、`/disconnect {}`。仅允许官方六个文档/参数方法，不提供任意命令执行。注册 Token 仅保存在本机 `.local/cubism-token`（0600，Windows 权限仍依赖 ACL），不返回浏览器，不要共享 `.local`。服务仅连接 loopback WS 地址。首次授权必须由用户在 Cubism 内完成，不绕过许可或授权。

官方 API 未提供 PSD 导入/精细网格/完整自动绑定接口；这些编辑步骤仍在 Cubism 内执行。

[官方外部 API 协议](https://docs.live2d.com/en/cubism-editor-manual/external-application-integration-api/) · [官方函数列表](https://docs.live2d.com/en/cubism-editor-manual/external-application-integration-api-list/)

[官方 Cubism 导出说明](https://docs.live2d.com/en/cubism-editor-manual/export-moc3-motion3-files/)

## 独立调用

```bash
go run ./cmd/oc -action list
go run ./cmd/oc -action import -file input.png
# Key 从 OC_API_KEY 环境变量读取，不传命令行
go run ./cmd/oc -action generate -prompt '正面全身角色立绘，四肢清晰分离' -model gpt-image-1
go run ./cmd/oc -action split -id ASSET_ID -file parts.json
go run ./cmd/l2d -action inspect -model 'C:/Models/sample.model3.json'
go run ./cmd/l2d -action cubism-discover
go run ./cmd/l2d -action cubism-launch -exe 'C:/Live2D/CubismEditor5.exe'
```

`parts.json` 示例：`[{"name":"head","x":10,"y":10,"width":200,"height":200}]`。OC CLI 与 GUI 默认共用 `.local/assets`；CLI 生成使用显式 base/model 与环境变量 Key，不读取 GUI 配置。

## 验证与边界

```bash
go test ./...
go vet ./...
go build ./cmd/...
npm --prefix CoreServer/web test
npm --prefix CoreServer/web run build
```

测试使用本机模拟供应商，覆盖素材、裁切、透明度、生成编辑协议、错误脱敏、模型引用、Cubism 路径和 Host/Origin/Key 隔离。没有付费供应商或真实模型端到端验证。

未来可扩展蒙版拆件、任务队列与 Cubism 制作工具适配。仅本机不等于多用户安全边界；本机操作者可读取模型文件并启动配置的可信编辑器，请保护本地访问权限。
