# Cubism 真实导入验收记录（修复后通过）

## 双眼分层与嘴部一键绑定：2026-11-22 18:58

新增 `cubism_build_face_rig`，仅适配 `scripts/make-face-fixture.go` 生成的16图层隔离样例，不适配任意角色。Cubism 5.3.05 FREE 中实际调用插件导出的 runner，生成 `.local/qa/face-rig/face-rig-fe63f66af102/face-rig-fe63f66af102.cmo3`。没有覆盖用户模型或调用付费接口。

眼白、虹膜/瞳孔、上眼睑、下眼睑独立绘制。角色左眼（画面右侧）的上眼睑绑定 `ParamEyeLOpen`，角色右眼绑定 `ParamEyeROpen`；两个上睑分别平移遮住静止眼白/眼球，而非缩放整眼。嘴部有独立唇线、口腔、舌头及上下覆盖层，`ParamMouthOpenY` 驱动上覆盖层平移显露/遮住口腔。最终补充版 `.local/qa/face-rig/face-rig-7d43c326a602/face-rig-7d43c326a602.cmo3` 为舌头 ArtMesh 也创建了 `ParamMouthOpenY` 关键形：闭口时下移28屏幕像素（约12原图像素），开口恢复。关闭并从磁盘重开后，`09-reopened-tongue-closed.png` 与 `10-reopened-tongue-open.png` 显示选中舌头网格边框不同位置及嘴部0/1关键点。尚无独立伸舌/卷舌参数；眼睑目前是覆盖式平移，仍不是生产级曲面眼睑变形。

保存、关闭后，从磁盘重开模型，再切换参数。`09-reopened-left-closed.png`、`10-reopened-right-closed.png`、`11-reopened-mouth-open.png` 为重开后真实截图。眼球固定ROI蓝色像素计数：左眼关闭时角色左0/右10885，右眼关闭时右0/左10896，双眼睁开分别10896/10885。口腔ROI颜色像素由闭口483变为开口11832。检查证明独立参数和嘴部开闭已持久保存，不是外部API临时预览。

首次运行发现保存目录继承历史目录，已限制查找隔离输出和工作室导出两个已知位置；首次嘴部覆盖层过短造成闭口上缘漏缝，延长遮盖层上边界250→190后重新导入重跑，最终截图闭口无口腔漏缝。样例仍有轻微睫毛端点残留，不宣称达到原画生产质量。脚本仅支持固定2576×1416布局，焦点改变停止，共享互斥锁禁止单眼/双眼并发。


## 自动网格与持久参数绑定验收：2026-11-22

在 Cubism Editor 5.3.05 FREE 的隔离眼睛模型中，通过编辑器界面选中 `DSH_PLUGIN_LIVE_EYE` 并执行 Cubism 自带的自动网格生成。随后将 ArtMesh 绑定到 `ParamEyeLOpen`，在 0.0 和 1.0 写入两个关键形；0.0 将整张眼睛贴图对应的网格纵向压缩，1.0 保留睁眼形状。此处验证关键形链路，不视作独立眼睑、眼球和高光的生产级眨眼方案。

另存新 `.cmo3`，关闭文档后从磁盘重开。实际画布截图验证 1.0 睁开、0.5 中间插值、0.0 压缩闭眼形，最后恢复到 1.0 预览。模型交付副本 `.local/qa/cubism-eye-rig-acceptance.cmo3` 与原保存文件逐字节一致；截图 `.local/qa/cubism-eye-auto-mesh.png`、`.local/qa/rig-reopened-open.png`、`.local/qa/rig-reopened-mid.png`、`.local/qa/rig-reopened-closed.png` 记录网格和重开后的三个状态。

本轮网格、关键形与保存均通过代理操作编辑器界面完成，没有使用外部 API 的临时参数值冒充持久绑定，也未新增通用的一键绑定插件工具。未覆盖用户模型。

## 官方 API 授权及真实参数读写验收：2026-11-22

按用户要求启动时仅选择 FREE，不启用 PRO 试用。通过前台进程校验的桌面操作打开「文件→外部应用程序集成的设置」，开启本机端口22033，保持“允许远程连接”关闭，并为“人人皆可建模”勾选权限。直接调用当前 profile 的 `cubism_connect` 返回 connected=true、approved=true。

在同一已授权 Cubism 5.3.05 FREE 进程6472中导入既有测试 PSD 8b2b9c88e31a0519f6fe2cf3，确认新建模型而非替换用户模型。`GetCurrentModelUID` 返回有效UID；`GetParameters` 返回27项参数，其中 ParamAngleX 范围[-30,30]、默认0。实际 `GetParameterValues` 读取0 → `SetParameterValues` 写15 → 再读取15。真实编辑器截图 `.local/qa/cubism-parameter-write-passed.png` 可见「角度 X」为15.0，文档标题和测试PSD一致，画布显示眼睛。

随后显式写回0，再读确认0，并调用 `ClearParameterValues`。实测 Clear 本身不保证当前滑块立即回到默认值，因此验收不把Clear当作恢复动作。未保存模型、未添加关键帧或绑定；测试PSD无角度形变关键形，因此截图证明参数值写入，不声称画稿已完成转头绑定。授权令牌不写入报告或公开截图。此次验收为真实插件官方WS读写，不是模拟服务。

## 当前 profile 安装与真实插件调用：2026-11-22 11:39

用户授权安装后，通过 Host plugin_manager 将 `G:/code/l2d agent/packages/dsh-cubism` 安装到当前 web profile。结果 bundle=dsh-cubism、version=0.0.2-alpha.0、enabled=true、application=applied；pnpm 使用本地 link，无版本豁免。

随后在会话中直接调用已注册的 `cubism_health` 成功、`cubism_connect` 返回connected=false/approved=false（未启用WS，不宣称授权联动通过）。真正的 `cubism_export_psd` 生成导出8b2b9c88e31a0519f6fe2cf3：320×240、PluginAcceptance组、DSH_PLUGIN_LIVE_EYE图层、(100,70)、80×60。使用现有眼睛素材的只读副本，不改变原图或发起付费调用。

直接调用已注册 `cubism_open_psd` 后，通过官方启动器开启新的Cubism进程35968；代理处理FREE启动模式，随后真实窗口标题含8b2b9c88e31a0519f6fe2cf3，画布显示眼睛，部件树显示PluginAcceptance，变形器树显示DSH_PLUGIN_LIVE_EYE。截图 `.local/qa/cubism-plugin-import-passed.png` 已读取检查。这证明“当前profile工具→工作室→PSD导出→官方启动→Cubism主界面导入”链路实际通过，不再仅为手工调用工具定义。启动确认使用桌面辅助脚本，不声称插件自行处理所有许可/启动对话框。未保存用户模型。参数WS连接/写入仍未验证。

## 最终 GUI 验收：2026-11-22 11:22

用户要求代理直接操作，不再交回手动验证。代理启动官方 Cubism 5.3.05 FREE 并通过官方批处理传入修复 PSD。用 AttachThreadInput/SetForegroundWindow 获得焦点，并在输入前校验前台 PID 与 Cubism 进程命令行；没有对其他应用发送输入。实际文档标题包含428b937f02e0cd15e85e167d，已打开三层模型。

真实 PrintWindow 截图 `.local/qa/cubism-import-passed.png` 显示 Front、Middle、Back 分类及红盖绿、绿盖蓝；变形器树列出三层完整测试名称。没有保存或覆盖用户模型。

截图纯色区域测量：蓝底 bbox (615,182)-(1419,785)，对应320×240；绿 bbox (766,283)-(1117,533)，红 bbox (867,359)-(1066,507)。按蓝底显示比例归一化，绿(60,40,140,100)、红(100,70,80,60)各项误差小于1.1原图像素（显示边缘抗锯齿造成）。遮挡像素检查通过，输出 `CUBISM_GUI_POSITION_SIZE_OCCLUSION_PASS`。结合之前原生解析器的精确矩形断言，修复后的隔离PSD位置/尺寸/分组/层序验收通过。

下文“未通过/待确认”为修复前历史，不代表当前最终结果。插件仍为项目原型，未安装profile；真实HTTP健康/发现/PSD导出验证通过，不声称具备任意网格编辑能力。

日期：2026-11-22。编辑器：本机 Live2D Cubism Editor 5.3.05 FREE。

## 2026-11-22 用户复测后的根因修正

用户手动导入仍为空。检查本机 Cubism 日志发现 `com.live2d.graphics.psd.a: error signature ::  @ 0x00000026`，在读取 ColorModeData 后即失败，早于图层读取；此前将失败主要归因于 UI 自动化是不充分的。

使用已安装 Cubism 5.3.05 自带 CPsdDocument 读取器在独立 Java 进程复现旧导出错误。仅添加标准 Image Resource 1005 ResolutionInfo（72 dpi、28 字节资源块）后，同一内容读取成功。生产导出器现包含该资源；新导出再次通过实际 Cubism 读取器，报告320×240、9条记录（三个像素层及六条分组边界/头），退出码0。第三方读取器容许空资源区不能证明 Cubism兼容。新回归测试检查资源签名、ID和固定点分辨率值；Go全量测试及vet通过。组的pass混合模式有日志警告，读取成功但真实UI渲染仍需确认。

修复后的隔离文件：`.local/qa/cubism-fixture/exports/428b937f02e0cd15e85e167d.psd`。这是实际 Cubism 解析器验收，不是主窗口完成建模导入的截图证明；不把两者混为一谈。

## 第五轮：Cubism 原生解析后的几何断言

对修复后的文件继续读取 CPsdDocument 返回的真实 layer 对象，而非只看成功日志。像素层名称和矩形分别为 BOTTOM_BLUE_320x240→(0,0,320,240)、MID_GREEN_140x100_at_60_40→(60,40,140,100)、TOP_RED_80x60_at_100_70→(100,70,80,60)，记录按底到顶顺序排列，分组头和边界保留。Java 独立进程断言三层名称及矩形全部通过，输出 `CUBISM_NATIVE_GEOMETRY_ASSERTIONS_PASS` 并退出0。不修改或注入运行中的编辑器。主窗口实际建模文档的渲染/导入确认仍未验证。

## 隔离测试

使用项目 PSD 编码器生成独立合成测试，不使用或覆盖用户原素材：

- 画布 320 × 240。
- 最上层 TOP_RED：80 × 60，位置 (100,70)，分组 Front。
- 中层 MID_GREEN：140 × 100，位置 (60,40)，分组 Middle。
- 底层 BOTTOM_BLUE：320 × 240，位置 (0,0)，分组 Back。
- 预期：红色覆盖绿色，绿色覆盖蓝色；文件导入后需逐项检查画布及层属性。

测试文件、原始桌面截图及辅助脚本保存在不提交的 `.local/qa` 中。没有调用付费 API。

## 实际观察

真实启动编辑器并观察到 FREE 版主窗口。曾出现原生“打开”对话框，但发送测试路径后没有观察到新模型、导入确认或可见测试图层。主窗口仍没有已打开文档。因此不能宣称导入通过，也没有尺寸、位置或层序的编辑器内验收数据。

桌面前台随后切到用户其他应用；停止全局鼠标/键盘输入，以免操作错误窗口。当前不再自动发送快捷键。后续应在独占且聚焦的测试桌面手动打开隔离 PSD，或使用能确认窗口身份的桌面自动化后端。

## 插件实测

项目本地 `packages/dsh-cubism` 原型通过 7 项传输测试及已安装 DSH 0.2.2-rc.1 的真实 SDK 定义注册检查（8 个工具）。实际调用本机工作室：

- health：ok。
- discover：发现本机 CubismEditor5.exe。
- connect：connected=false、approved=false，提示启用 Editor 外部联动。

以上不是官方 API 授权成功或 PSD 导入成功。未安装/启用到任何 DSH profile；没有用参数 API 伪造网格布局能力。原型通过工作室 API 访问现有受限能力，不是注入到 Cubism 内部的通用脚本执行器。

## 继续验收：非前台窗口方式

本轮改用 Win32 `PrintWindow` 对已确认 PID/窗口标题的 Cubism 窗口截图，不再用屏幕矩形截图，也不抢占用户前台。成功取得真实窗口图像，确认标题仍无文档名、部件列表和工作区为空。

向同一已确认 HWND 发送隔离 PSD 的 `WM_DROPFILES`（不发送全局鼠标键盘）后，再次截图仍为空白。消息发送成功不代表 Java/AWT 接受该拖放机制；本次未观察到导入对话框，因此这一方式也未通过，不作为生产打开实现。未重复启动或修改用户文档。

## 第二轮：独立 PSD 解码验证

将 `ag-psd` 仅安装在忽略的 `.local/qa/psd-reader`，用其独立实现读取同一个真实测试 PSD（非项目自己的解析器）。画布 320×240、Back/Middle/Front 文件夹及各层 RGBA 数据均可正常解码。断言通过：蓝层 (0,0,320,240)、绿层 (60,40,140,100)、红层 (100,70,80,60)；合并像素 (110,80) 为红、(70,50) 为绿、(5,5) 为蓝，alpha 均为255。说明实际 PSD 数据与预期位置/尺寸/遮挡一致，但这不是 Cubism 内部验收。

同时尝试向已验证 Cubism HWND 定向发送文件菜单鼠标消息（无全局输入）；AWT 未展示菜单，未进一步发送快捷键或确认。因此仍未导入，未改生产打开流程。

## 第三轮：插件真实 PSD 导出链路

直接执行原型的 `cubism_export_psd` 工具定义，调用运行中的工作室：对现有素材建立独立导出（不修改素材），画布320×240、组 PluginAcceptance、图层位置(10,20)、输出80×60。收到真实导出 ID `ee5e645f8f9dabab5f4a937d`；下载后由 ag-psd 独立解码验证组名、图层名、画布、尺寸及坐标全部通过。调用前后素材 ID 列表一致，无付费生成。这是“工具定义→真实HTTP服务→PSD编码→下载→独立解码”的验证，不是 Host profile 发现/启用或 Cubism 导入验收。

连续三轮仍缺少可靠的 Cubism UI 导入确认通道：官方 WS 未启用/授权，Java/AWT 不响应已尝试的定向文件/菜单消息；用户前台应用可能变化，不继续发送盲目全局输入。继续需要独占可交互的编辑器测试环境，或用户手工完成测试 PSD 导入并启用外部联动。当前目标不能标记完成。

## 可复用本机兼容性检查

新增 `scripts/CubismPsdCheck.java`：使用 JDK21+，在项目根执行 `java --class-path "Live2DCubism/app/lib/*" scripts/CubismPsdCheck.java FILE.psd`。通过反射调用用户本机安装的5.3.05读取器，打印画布、图层名称及矩形，无非空像素层或读取异常则退出1，成功退出0。对修复文件实跑输出 `CUBISM_READER_PASS pixelLayers=3`。不打包第三方库，不修改安装或附加到编辑器；内部类名可能随版本变化，此探针不是官方公共API。插件7项测试重新通过。

## 后续验收门槛

1. 独占测试窗口，手工完成启动模式及 PSD 导入确认。
2. 确认画布 320 × 240、三层名称/分组及遮挡顺序。
3. 在编辑器检查位置/尺寸，保存截图和属性证据；不以 PNG 合并预览代替。
4. 启用外部应用联动并明确允许注册，检查 connected/approved，再读取文档 UID。
5. 在获得 profile 安装授权且 Host 允许后安装插件，验证真实 profile 工具发现与执行。
