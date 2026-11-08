# Open TJ

<img src="assets/logo.png" alt="Open TJ" width="128">

同济校园 VPN 原生客户端，支持 macOS 和 Android、流量统计和自动重连。
macOS 客户端使用 Go 与 [MyGo](https://github.com/egoist/mygo) 原生 UI 实现，不依赖 Swift 或 WebView。

本项目 100% 由 **gpt6-astra** 开发。

## 下载与使用

| 平台 | 下载 | 系统要求 |
| --- | --- | --- |
| Android | [下载 APK](https://github.com/cuicui-huang/open_tj/releases/download/android-v0.2.1/OpenTJ-0.2.1-android.apk) · [发布说明](https://github.com/cuicui-huang/open_tj/releases/tag/android-v0.2.1) | Android 8.0 及以上 |
| macOS | [下载 Universal ZIP](https://github.com/cuicui-huang/open_tj/releases/latest) | macOS 14 及以上，Apple Silicon / Intel |

**Android：** 下载 `.apk` 后在手机上打开，按系统提示允许本次安装来源。安装完成后
输入校园 VPN 账号与密码，点击连接并同意系统 VPN 授权。无需电脑、ADB、JDK 或额外插件。
发布 APK 使用固定密钥签名，后续正式签名版本可覆盖升级；Android 目前为预览版，
真机验证范围和限制见 [Android 指南](docs/ANDROID.md)。

**macOS：** 下载并解压 ZIP，将 **Open TJ.app** 拖入「应用程序」，打开后输入账号，
点击连接并完成系统授权。

macOS 构建尚未经 Apple 公证。如被系统拦截，尝试打开后，到「系统设置 → 隐私与安全性」
选择「仍要打开」。两平台的 Release 安装包均可直接下载，无需 GitHub 账号。

关闭窗口后应用继续在菜单栏运行；升级前请先退出旧版本。

## 项目目录

```text
apps/desktop/         Go + MyGo 原生 macOS 客户端
  main.go             入口、菜单栏、生命周期
  internal/app/       状态机、窗口与面板视图
  internal/vpn/       登录、隧道、重连监督器
  internal/client/    网关协议、helper、路由与诊断
  internal/…          凭据、偏好、日志
  mygo.json           应用名、标识、版本与打包配置
apps/android/         Kotlin/Compose 客户端、协议模块、Android VERSION
testdata/             Go/Kotlin 共用协议样例
scripts/              本地与 CI 共用的测试、构建、签名脚本
docs/                 架构、开发与发布验收说明
.github/workflows/    Android 与 macOS 独立构建发布
build/、dist/         本地工具链与产物，不提交 Git
```

## 开发构建

macOS 需要 Go 1.27 或更高版本；MyGo 作为 Go 工具固定在 `apps/desktop/go.mod`，
无需额外安装。开发时可直接运行：

```sh
cd apps/desktop
go tool mygo dev      # 编辑 .go 文件后自动重启
```

��整测试、构建与打包：

```sh
make test             # go test -race ./... + go vet ./...
make build            # 当前架构的 .app
make build-all        # 六个目标：macOS/Windows/Linux × arm64/amd64
make package          # macOS 双架构 Universal ZIP 到 dist/
```

`make build` 可通过 `ARCHS="arm64 x86_64"` 构建 Universal，`SIGN_IDENTITY` 指定
Developer ID 时由 MyGo 完成签名。

macOS 客户端不使用 Swift、Xcode 或 WebView；界面由 MyGo 在 Metal 上绘制。
Windows/Linux 版本可以编译和启动界面，但 VPN 连接目前仅支持 macOS。

Android（需要 JDK 17 和 Android SDK）：

```sh
make android-test
make android          # 本机 debug 签名，dist/android/debug/
make android-release  # 优化未签名包，dist/android/unsigned/
```

推送 `main` 或提交 PR 会在 [Actions](https://github.com/cuicui-huang/open_tj/actions) 自动测试和构建。
Android 推送与 `apps/android/VERSION` 一致的 `android-v*` 标签，自动构建、签名，
在 Android 8/16 模拟器验证安装后发布 APK。macOS 使用根目录 `VERSION` 和 `v*` 标签。
普通用户下载 Release 中的 APK；Actions 中的 `unsigned` 附件仅供开发检查，不能直接安装。
维护者首次配置签名和后续发布步骤见 [分发指南](docs/DISTRIBUTION.md)。

## 许可证

[MIT](LICENSE)
