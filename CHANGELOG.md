# 变更记录

## 未发布

- macOS 客户端改用 Go + MyGo 原生 UI 重写，移除 SwiftUI 与 stdin/stdout JSON
  事件协议，后端与界面合并为单一 Go 程序。
- 拆分为 `internal/client`、`internal/vpn`、`internal/app`、`internal/settings`、
  `internal/credentials`、`internal/logging`，新增视图与状态机单元测试。
- 菜单栏面板改为 MyGo flyout，新增托盘速率标题；配置、凭据、日志路径保持兼容。
- 构建与 CI 改为 `go tool mygo build -platform darwin/universal`，不再需要
  Swift 工具链。
- 修复 `host_network_modified` 未记录，导致网关断线后不重试、网络恢复结果误报。
- 管理员授权与 utun 配置期间保持网关心跳，避免慢授权导致会话被回收。
- 网关主动终止会话时不再无限重连，并按原因码给出可操作的提示。
- CI 并行构建 macOS、Windows、Linux 的 arm64/amd64 六个产物并随 Release 发布；
  Windows/Linux 版本目前仅用于界面预览，连接仍限 macOS。

## Android 0.2.1（预览版）

- 添加 Android 原生预览客户端，含系统 VPN 授权、后台通知、流量与日志界面。
- 将网关协议移植为独立 Kotlin 模块，加入 Go/Kotlin 共用协议样例与测试。
- 支持 Android Keystore 凭据加密、底层网络切换与分类自动重连。
- 修复部分厂商在 TUN 建立前拒绝 `protect` 导致的持续重连，并安全忽略 IPv6 首包。
- 添加 Android 测试、打包、CI 和真机验收文档。
- Android 使用独立版本文件和 `android-v*` 标签；GitHub Actions 自动构建、
  固定密钥签名并验证 Android 8/16 安装后发布可直接下载的 APK。
- 统一 Android 产物目录为 `dist/android`，提供公开签名指纹及 SHA-256 校验文件。

## 0.2.1

- 加入项目 Logo 与 macOS 应用图标。
- 简化 README，支持 GitHub Actions 双架构构建及标签自动发布。
- 修正 CI 架构验证命令。

## 0.2.0

- 将现有 SwiftUI + Go 客户端提取为独立项目。
- 提供统一测试、构建、Universal ZIP 打包与签名公证脚本。
- 清理历史代理依赖，补齐第三方许可证与发布文档。
- 保持现有凭据标识、网络行为、菜单栏和流量日志功能。
