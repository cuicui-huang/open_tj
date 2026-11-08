# Third-party notices

Open TJ 的 macOS 客户端运行依赖如下；版本以 apps/desktop/go.mod 和 go.sum 为准。

| Dependency | License | Included license |
| --- | --- | --- |
| github.com/egoist/mygo (native UI, windowing, packaging) | MIT | licenses/mygo-LICENSE.txt |
| golang.zx2c4.com/wireguard (tun package) | MIT | licenses/wireguard-LICENSE.txt |
| github.com/ebitengine/purego | MIT | licenses/purego-LICENSE.txt |
| github.com/go-text/typesetting | MIT | licenses/go-text-typesetting-LICENSE.txt |
| golang.org/x/sys | BSD-3-Clause | licenses/x-sys-LICENSE.txt |
| golang.org/x/text | BSD-3-Clause | licenses/x-text-LICENSE.txt |
| golang.org/x/image | BSD-3-Clause | licenses/x-image-LICENSE.txt |
| golang.org/x/net | BSD-3-Clause | licenses/x-net-LICENSE.txt |
| golang.zx2c4.com/wintun (Windows tun) | MIT | licenses/wintun-LICENSE.txt |

应用不使用 WireGuard 隧道协议。旧 Python 代理依赖的 gVisor 未进入当前发行目标。Go 编译器运行库的许可证见 licenses/go-LICENSE.txt。上述许可证随 app 的 Contents/Resources 一同分发（`resources` 配置会把 `licenses` 复制进应用包）。

本项目采用 MIT 许可证，见根目录 LICENSE。第三方组件仍遵循各自许可证。

## Android

Android 客户端不包含 Go 或 WireGuard 运行库。运行依赖包括 Kotlin 标准库、
kotlinx.coroutines、kotlinx.serialization，以及 AndroidX Activity、
Lifecycle、Compose 和 Material 3，均按 Apache License 2.0 分发。
具体版本以 `apps/android` 的 Gradle 构建文件及解析后的依赖树为准。
Android APK 的 `assets/licenses` 包含项目许可证、Apache 2.0 许可证和通知。
JUnit 与 AndroidX Test 仅用于测试，不进入正式应用的运行依赖。
