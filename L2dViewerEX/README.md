# Live2DViewerEX 本地安装目录

工作室启动时默认创建项目根目录的 `L2dViewerEX/` 文件夹。请将通过官方渠道取得的完整 Live2DViewerEX 程序及依赖放到这里，使 `L2dViewerEX/Live2DViewerEX.exe` 存在，再使用「发现 exe 路径建议」。**仓库不附带软件本体，也没有下载或安装完成。**

官方文档将 Windows / macOS 下载引导至 Steam：
- 官方文档：https://live2d.pavostudio.com/doc/zh-cn/
- 官方商店：https://store.steampowered.com/app/616720/Live2DViewerEX/

## 安装

1. 打开本目录的 `官方下载.url`，在 Steam 登录并取得所需授权。
2. 通过 Steam 安装 Live2DViewerEX。Steam 管理库目录，网页不能强制将其安装到任意文件夹；请勿将快捷方式当作程序本体。
3. 若官方发行版允许移动完整安装目录，可将完整文件放到此目录，使 `Live2DViewerEX.exe` 位于目录根部。不要只复制 EXE；需要保留资源和依赖，且运行仍可能依赖 Steam。请遵循软件许可，不在 GitHub 上传安装文件。
4. 或保留 Steam 原安装位置，在工作室点击「发现 exe 路径建议」，将实际绝对路径复制到设置。非默认 Steam 库可能需要手动填写。
5. 在 ViewerEX 中启用 ExAPI，再在工作室探测连接。

项目从仓库根目录启动时，路径发现优先检查本目录，以及其下的 `Live2DViewerEX/`、`steamapps/common/Live2DViewerEX/`，不会自动启动或修改配置。
