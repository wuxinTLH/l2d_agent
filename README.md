# 人人皆可建模 · l2d_agent

当前版本：**v0.0.1a**（前端 npm 元数据以兼容 SemVer 的 `0.0.1-a` 表示）。

仅本机运行的人设素材与 Live2D Cubism Editor 建模工作台（ViewerEX 仅为可选展示）。后端 Go，前端 Vite + TypeScript + SCSS + Tailwind CSS。没有云端部署、账号服务、数据库服务器或 Blender/Unity 实现。调用远程图片 API 仍需要联网，可能产生供应商费用；这不等于部署远程服务器。

## 模块边界

- `oc/`：图片 API 适配、PNG/JPEG 导入、PNG 素材库、矩形裁切、带原图坐标的分层清单。不依赖 CoreServer。
- `l2d/`：现有 Cubism 导出资源检查、本机 ViewerEX 启动、官方 ExAPI WebSocket 桥接。不依赖 CoreServer 或 oc。
- `CoreServer/`：Go 本地 HTTP 协调层及 `web/` GUI，通过 Go 包调用模块。
- `cmd/studio`、`cmd/oc`、`cmd/l2d`：工作台和独立模块命令入口。

**这是可运行基础版，不是完整自动 Live2D 制作器。** 分层首版是人工框选裁切，不是语义分割；不能补出被头发、衣服遮挡的身体区域。真实制作还需要透明蒙版、遮挡补绘、图层整理、网格/变形器/参数绑定和 Cubism 导出。矩形裁切中可能包含相邻部件，不能直接当作制作级拆件。

## 本地构建与启动（Windows）

依赖 Go 1.24+、Node.js 22.12+ 和 npm；建议本机现有较新版本。Git Bash 中执行：

```bash
bash scripts/build.sh
bash scripts/start.sh
```

打开 **http://127.0.0.1:17823**。也可从项目根目录直接运行 `./bin/studio.exe`。构建物的前端不内嵌，移动可执行文件时需同时保留 `CoreServer/web/dist`，或传入 `-web` 指向该目录。

```bash
go run ./cmd/studio -addr 127.0.0.1:17823 -data .local -web CoreServer/web/dist
```

只允许 loopback IP 监听。默认 `.local/` 储存配置与素材，已加入忽略规则。不配置第三方 API 也能导入、浏览、裁切图片和检查已有模型。

前端开发分两个终端：

```bash
go run ./cmd/studio
npm --prefix CoreServer/web run dev
```

Vite 绑定本机，`/api` 和 `/assets` 代理至 17823。构建模式只需一个 Go 进程。

## OC 工作流

1. 设置图片 API Base URL（例如 `https://api.openai.com/v1`）、模型名与 Key。采用 OpenAI 风格 `POST /images/generations` 和 multipart `POST /images/edits`，**不保证所有所谓兼容供应商都支持图片或相同参数**。
2. 输入人设 prompt，或直接导入 PNG/JPEG。负面提示会合并进文本 `Avoid:`，不是供应商原生 negative_prompt 参数。
3. 编辑会把原图和编辑提示提交给配置的接口。它是供应商控制的整图编辑，不保证角色身份、姿态或未指定像素完全不变，当前未实现蒙版修补。
4. 裁切页提供矩形 / 选择 / 平移工具、滚轮指针锚定缩放、放大缩小、适应画布及 100% 原尺寸。拖动已有框可移动，拖动八个边角手柄可调整；空格或中键拖动画布。撤销 / 重做支持绘制、移动、缩放框、删除和右侧数值修改；快捷键仅在画布获得焦点时启用，不拦截输入框编辑。所有坐标始终是原图整数像素，缩放不会降低导出分辨率。命名后导出 PNG 与包含源素材 ID、画布和坐标映射的 manifest。
5. 「一键 API 拆分」使用同一套配置的 `/images/edits`，按可编辑的部件名称逐一请求隔离、去背景和遮挡补绘，默认 10 个精细部件（左右眼睛、头、左右手、嘴、身体、左右耳朵和头发；左右以角色自身为准）；每个部件一次可能计费的请求，最多 12 个，**不自动重试**。需要供应商支持图片编辑；仅支持生成的接口不能使用本功能。
6. 拆分前显示请求数量并防止重复提交；输出经透明空图 / 近白图检测，失败单独列出，不删除原图或已有部件。结果可切换棋盘格 / 深色 / 浅色背景预览，下载 PNG 和来源 manifest。
7. AI 拆分是生成式重绘，可能改变身份、细节或输出不透明背景；不能保证完整分层或坐标对齐。manifest 仅记录来源与生成过程，不伪造矩形坐标。检查补绘、对齐与遮挡关系之后，再转入真实建模流程。

供应商返回支持 `data[0].b64_json` 或 HTTPS `data[0].url`，优先使用 Base64；图片下载不会附带 API Key，且禁止重定向。单张图片限制 32 MiB / 4000 万像素。图片生成请求最长 180 秒，界面错误不应自动重试付费调用。

Key 不会从配置读取接口返回，也不会写入浏览器 localStorage。为了重启后继续使用，Key **明文存于本机 `.local/config.json`**，并非系统凭证保险库。Go 写入使用 0600；Windows ACL 的实际隔离由操作系统决定。不要共享或提交该目录；本机其他可访问这些文件的程序仍可能读取它。配置留空 Key 可清除（API 字段省略则保留）。

## Live2D Cubism Editor 下载与本地安装

建模目标为 **Cubism Editor**，而非 ViewerEX。请从 [Cubism 官网下载页](https://www.live2d.com/zh-CHS/cubism/download/editor/) 阅读许可并获取 Windows 安装包。启动工作室会创建 `Live2DCubism/`；安装包可保存于此，但仍须运行官方安装流程。设置填写实际安装的 `CubismEditor*.exe`，不是安装器。不要单独复制 EXE 而遗漏依赖。仓库不含软件本体或激活工具。

「发现 Cubism 路径」仅检查项目目录和常见安装目录；「启动 Cubism」只确认已创建进程，不确认许可证、编辑器就绪或项目已打开。新增 `cubismExecutable` 配置和 `POST /api/l2d/cubism/discover`、`POST /api/l2d/cubism/launch` 接口。主要流程是整理分层素材 → Cubism 内建网格/变形器/参数 → 导出模型；没有借用 ViewerEX ExAPI 伪装 Cubism 自动化。

## 可选：Live2DViewerEX 下载与本地安装

设置和 Live2D 工作流均提供 [官方 Steam 下载 / 安装入口](https://store.steampowered.com/app/616720/Live2DViewerEX/)。[官方文档](https://live2d.pavostudio.com/doc/zh-cn/) 指向该渠道；需要 Steam 登录及相应授权，本项目不提供第三方安装包。

如需可选 ViewerEX 展示功能，可自行使用项目根目录的 `L2dViewerEX/` 文件夹存放官方取得的完整软件（含依赖），内含安装说明和官方商店快捷方式。**没有随仓库下载软件本体**；Steam 管理其安装目录，不能通过网页强制下载到此文件夹。取得官方软件后按该目录说明操作，或直接配置 Steam 原安装位置。程序从项目根目录启动时优先检测 `live2dViewerEX/Live2DViewerEX.exe`，只建议路径，不自动启动或保存。安装文件已排除出 Git。

## Live2DViewerEX 的真实能力与限制

已核实官方文档：ExAPI 是 `ws://127.0.0.1:10086/api`，不是 REST。端口被占用时 ViewerEX 可递增选择，请按安装版本实际端口配置。在 ViewerEX 中启用 ExAPI，并配置本机可执行文件路径；启动按钮只确认创建进程，不确认软件加载完成。

引导式操作面板按「模型准备 → 配置与连接 → 加载模型 → 动作与声音」排列：

1. 点击「发现 exe 路径建议」搜索常见安装目录（包括默认 Steam 目录），核对后复制到设置保存；自定义库/便携安装手动填写。不会扫描整盘或自动启动候选程序。
2. 启动已配置 ViewerEX，在软件中启用 ExAPI 后点击「探测 EXAPI 连接」。实际收到关联的 10000 回显才显示本次探测连接成功；配置存在不等于已连接。
3. 填写 ViewerEX `.model.json` 的 Windows 绝对路径及槽位，点击加载。随后可填写已有命名动作/动作文件、表情索引，或播放/停止本地音频。无需手写 JSON。
4. 加载、动作、表情和音频都是 send-only 操作，反馈明确标记「已发送，未验证」，实际效果需在 ViewerEX 中确认。下方仍保留 Cubism 引用检查及高级原始命令。

新增 `POST /api/l2d/probe` 返回 `{connected,endpoint,error?}`；`POST /api/l2d/discover` 返回 `{candidates,note}`。探测不自动重试、不自动改端口或配置。

- 检查页读取已有 Cubism `.model3.json`，检查引用是否存在、非空且不越出模型目录。它**不解析或验证 moc3 二进制合法性、纹理内容和动画质量**，不表示已成功加载 ViewerEX。
- ExAPI 文档加载示例使用 ViewerEX 的 `*.model.json` 配置。Cubism 的 `.model3.json` 与 ViewerEX 配置不是可随意互换的文件，建议先用 EX Studio 导入并配置现有模型。
- EX Studio 能编辑模型配置、关联动作/表情、配置控制器和口型；它不是已证实能从 PNG 自动生成网格/绑定/moc3 的编译器。
- 正式 moc3 需要 Cubism 建模与导出。手势/表情先在现有模型中制作，再通过 ViewerEX 触发。
- 官方公开 ExAPI 清单未提供枚举模型/参数、通用直接设置参数或从图片建模接口。本项目不伪造这些能力。
- 音量口型是配置控制器驱动嘴部开合，不等同于音素/viseme 合成。未来适配器可触发预配置命名动作，不可将 EX Script 命令误当 WebSocket API。

零起始模型槽位示例（必须替换为真实槽位与配置动作）：

```json
{"msg":10000,"msgId":1}
```

上面是注册事件/官方回显探测。触发已配置动作：

```json
{"msg":13200,"msgId":3,"data":{"id":0,"type":0,"mtn":"Tap:Wave"}}
```

播放音频（需真实本地路径、已配置口型控制器）：

```json
{"msg":13500,"msgId":6,"data":{"id":0,"channel":0,"volume":1,"delay":0,"loop":false,"type":0,"sound":"C:\\Audio\\speech.wav"}}
```

桥接连接是单次请求，不是持久事件订阅器。仅 `msg:10000` 等待最多 5 秒并校验 `msg` / `msgId` 回显后关闭；其余命令在 WebSocket 写入后返回 `status:sent, verified:false`，不等待不存在的通用回执。这只表示发送，不证明目标动作实际生效；选择气泡的延迟回复也不会收集。探测超时或网络异常时请勿盲目重发。当前未连接真实 ViewerEX 或验证嘴型/动作。

### 官方参考

- [ExAPI 概述与端口](https://live2d.pavostudio.com/doc/en-us/exapi/)
- [消息结构](https://live2d.pavostudio.com/doc/en-us/exapi/api-structure/)
- [官方 API 列表](https://live2d.pavostudio.com/doc/zh-cn/exapi/api-list/)
- [官方开源 ExAPI 示例](https://github.com/pavostudio/ExAPI)（仅作为协议参考，未捆绑或声称拥有其许可）
- [EX Studio 模型配置编辑器](https://live2d.pavostudio.com/doc/en-us/exstudio/live2d-editor/)
- [配置创建教程：需现有 Moc](https://live2d.pavostudio.com/doc/en-us/exstudio/live2d-editor-tutorial/)
- [Cubism MOC3 / motion3 导出](https://docs.live2d.com/en/cubism-editor-manual/export-moc3-motion3-files/)

## 独立调用

```bash
go run ./cmd/oc -action list
go run ./cmd/oc -action import -file input.png
# Key 从环境变量 OC_API_KEY 读取，不传命令行
go run ./cmd/oc -action generate -prompt '正面全身角色立绘，四肢清晰分离' -model gpt-image-1
go run ./cmd/oc -action split -id ASSET_ID -file parts.json
go run ./cmd/l2d -action inspect -model 'C:/Models/sample.model3.json'
go run ./cmd/l2d -action command -message '{"msg":10000,"msgId":1}'
```

`parts.json` 是数组，例如 `[{"name":"head","x":10,"y":10,"width":200,"height":200}]`。`cmd/oc` 与 GUI 默认共用 `.local/assets`；CLI 生成不读取 GUI 配置，使用显式 base/model 和环境变量 Key。

## 验证与后续边界

```bash
go test ./...
go vet ./...
go build ./cmd/...
npm --prefix CoreServer/web run build
```

测试使用本机模拟图片供应商和 WebSocket，覆盖素材导入、裁切/透明度/非法坐标、生成与编辑协议、错误脱敏、模型引用检查、连接协议、Host/Origin 校验和配置 Key 隐藏。无付费 API 或真实模型端到端测试。

后续应按独立适配器增加：蒙版/语义拆件与补绘供应商、可恢复任务队列、Cubism 制作工具集成、持久 ExAPI 连接和动作/音频编排。Blender 与 Unity 作为未来扩展，当前不安装、不实现。仅本机工具并不提供多用户安全边界：拥有本机浏览器/文件权限的操作者可以调用本地文件检查、启动配置的程序和发送 ViewerEX 命令；请只配置可信程序及 API。
