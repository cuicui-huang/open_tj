此版本 macOS 客户端改用 Go 与 MyGo 原生 UI 重写，不再依赖 SwiftUI；功能、凭据标识与日志路径保持兼容。

Open TJ macOS Universal 构建，支持 Apple Silicon 和 Intel，最低系统目标为 macOS 14。

下载 `OpenTJ-*.zip`，解压后将 Open TJ.app 放入「应用程序」，打开并输入账号即可连接。`.sha256` 文件用于核对下载完整性。

此版本由 GitHub Actions 自动测试和构建，采用 ad-hoc 签名，尚未经 Apple 公证。如被 macOS 拦截，尝试打开后到「系统设置 → 隐私与安全性」选择「仍要打开」。

升级前请退出旧版本。不同系统版本、Intel 真机及实际网络环境的兼容性仍需验证。


同时提供 Windows 10/11 的 arm64/amd64 构建，连接与 macOS 使用同一套网关协议（需要管理员权限，未签名）。不再提供 Linux 版本。
