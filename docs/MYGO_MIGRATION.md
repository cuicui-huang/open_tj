# 用 MyGo 重构 macOS 客户端

> **实施状态：** 阶段 0–5 已完成。macOS 客户端位于 `apps/desktop`，旧的
> `apps/macos` 与 `backend` 已删除，构建、测试、CI 与文档均已切换。唯一与下文
> 方案的差异：提权 helper 复用应用自身的 `--helper` 模式，而不是独立二进制，
> 由 `main` 在 MyGo 初始化前分发。真机网络验收仍按 `RELEASE_CHECKLIST.md` 执行。

本文是重构的评估、设计与迁移记录。结论：**可行，且收益明显**——桌面端
本来就是「Go 后端 + SwiftUI 前端」的两进程结构，MyGo 的原生 UI 可以把两者合并
成一个 Go 程序，去掉 stdin/stdout JSON 协议、Swift 单文件 UI 和每次启动复制
后端的逻辑，同时保留管理员 helper 的独立提权路径。

- 框架：<https://github.com/egoist/mygo>，评估版本 `v0.8.3`
- 影响范围：仅 `apps/macos/` 与 `backend/`；`apps/android/` 与共享协议向量不动
- 最低系统：继续 macOS 14（MyGo 默认 12，配置里显式设为 14）

## 1. 现状盘点

### 1.1 组成

| 位置 | 内容 | 规模 |
| --- | --- | --- |
| `apps/macos/OpenTJ.swift` | SwiftUI 单文件客户端：窗口、菜单栏面板、流量图、日志面板、钥匙串、进程与 JSON 事件解析 | 672 行 |
| `backend/cmd/motionvpn/main.go` | 监督进程：读凭据 → 登录 → 提权 helper → 配置网络 → 转发 → 重连 → 事件输出 | 393 行 |
| `backend/internal/client/*` | 网关协议、TLS、helper、utun、路由/DNS/代理事务、流量统计、诊断探针、转发桥 | 1169 行实现 + 855 行测试 |
| `scripts/build.sh` `test.sh` `package.sh` `release.sh` | swiftc 编译、lipo 合并、Go helper 交叉编译、签名、公证、ZIP | — |
| `testdata/vpn-protocol.json` | Go/Kotlin 共用协议样例 | — |

### 1.2 现有运行时结构

```
OpenTJ.app (SwiftUI, 主进程)
  └─ stdin: {"username","password","method"} / "stop" / "sleep" / "wake"
     stdout: 逐行 JSON 事件 (traffic / log / connecting / progress /
             connected / reconnecting / sleeping / failed / result)
  └─ backend/Resources/motionvpn 复制到 ~/Library/Application Support/
     MotionVPNPrototype/go-backend 后执行
        └─ osascript 提权: motionvpn --helper <socket> <token>
             └─ 创建 utun、配置路由/DNS/代理，退出时回滚并写回执
```

事件流是唯一的 UI 数据源，`VPNModel.consume(_:)` 负责解析与状态机；
`VPNModel.selfTest()` 在 Swift 进程内验证分帧、限流、图表刻度等。

### 1.3 关键耦合点与痛点

1. **两个语言、两套模型**：`LogEntry`/流量/状态在 Swift 与 Go 各一份；事件
   字段名靠字符串约定，Swift 侧用 `[String: Any]` 取值，改字段不会编译报错。
2. **进程生命周期复杂**：复制可执行文件、管道读写、`terminationHandler`、
   `pending` 缓冲、`sessionID` 防串台，都在手工处理。
3. **UI 单文件 672 行**：`VPNModel` + 5 个 View + 3 个 `NSViewRepresentable`
   + AppDelegate，无法单测（只能跑 `--self-test` 断言）。
4. **构建依赖 Swift 工具链 + Go**，`test.sh` 还要额外编译一次 Swift 自测。
5. 传输层已经完全是 Go，Swift 只做展示；这使「用 Go 写 UI」几乎没有迁移成本。

## 2. 为什么 MyGo 合适

| 现有能力 | MyGo 对应 |
| --- | --- |
| SwiftUI 原生窗口 | `mygo.NewWindow(WindowOptions{Content: ui.View(view)})`，GPU 原生绘制，无 WebView |
| `MenuBarExtra(.window)` 面板 | `mygo.NewTray` + `mygo.NewFlyout(FlyoutOptions{Tray:…, Popover:true})`（官方 `examples/menubar-flyout`） |
| 窗口尺寸/位置记忆 | `WindowOptions.StateKey` / `Window.SetContentSize` |
| 钥匙串 | `mygo.Secrets`（macOS Keychain，`Secrets.Service(name)` 可指定 service） |
| 睡眠/唤醒 | `mygo.Power.OnSuspend` / `OnResume` |
| 流量图表（60 点折线） | `ui/chart`（`Plot` + `NewLinear` + `Draw`）或 `ui.Canvas` 等价绘制 |
| 日志列表（自动跟随、2000 条上限） | `ui.List` + `ui.ListState{FollowEnd:true}`，虚拟滚动 |
| 启动、重连、退出拦截 | `App.WhenReady` / `OnBeforeQuit` / `OnWillQuit`；`Window.Update` 从任意 goroutine 更新 |
| 无窗口单测 | `ui.NewTester(view, w, h)`：可 `Click("连接 VPN")`、`HasText`、`Key`，无需 GUI |
| 通用包、签名、公证 | `go tool mygo build -platform darwin/universal`；`macos.helperEntitlements` 可单独签 helper |
| 视觉 | `Vibrancy`、主题（浅/深色自动跟随）、`glass` 插件（macOS 26 Liquid Glass） |

另外 MyGo 的 `i18n`、`Updater`、`Distribution` 等能力本次不使用，但为后续
自动更新（当前没有）留了空间。

## 3. 目标架构

### 3.1 单一 Go 模块

新建 `apps/desktop/`（模块 `open-tj.local/desktop`），把 `backend/internal/client`
整体迁入；`backend/` 在迁移完成后删除。Android 保持 Kotlin 独立实现。

```text
apps/desktop/
├── go.mod                 # module open-tj.local/desktop
├── go.sum
├── mygo.json              # name/identifier/version/main/resources/macos
├── main.go                # 参数分发(--helper/--diagnose)、App 生命周期、tray
├── controller.go          # 应用状态机(替代 VPNModel)：状态/流量/日志/凭据
├── supervisor.go          # 原 cmd/motionvpn 主循环，改为同进程回调
├── view_main.go           # 主窗口(Dashboard + 连接表单 + 设置)
├── view_log.go            # 日志面板(筛选/搜索/展开/复制/清空)
├── view_traffic.go        # 速度、累计、折线图
├── view_menu.go           # 菜单栏 flyout
├── tray.go                # 托盘图标(运行时绘制 PNG，模板图标)
├── helper/                # 独立提权二进制(仅 darwin)
│   └── main.go            # --helper <socket> <token>，调用 client.RunHelper
├── internal/client/       # 从 backend/internal/client 迁入(原样 + 路径修正)
├── resources/
│   ├── icon.png           # 复用 assets/logo.png 生成 1024×1024
│   └── darwin-universal/bin/opentj-helper
└── *_test.go              # supervisor 策略、controller、view(Tester)
```

> 也可以让 app 二进制自身以 `--helper` 提权运行（`os.Executable()`，无需额外
> 资源）。但推荐独立 helper：特权进程不含任何 UI/框架代码，体积更小，签名与
> 审计更清晰。MyGo 会随 app 一起签 `resources` 中的 Mach-O。

### 3.2 同进程监督器

去掉 stdin/stdout JSON：

```go
// 原来: emit("traffic", …) -> stdout -> Swift consume(_:)
// 现在: 同进程回调 -> controller -> win.Update(重绘)
type Reporter interface {
    Log(level, component, message string)
    Progress(message string)
    Traffic(client.TrafficSample)
    Connecting()
    Connected(iface string)
    Reconnecting(seconds int)
    Sleeping()
    Failed(message string)
    Result(result map[string]any, level string)
}
```

- `supervisor.Run(ctx, cred, reporter, cfg)` 保留原有重连指数退避、sleep/wake、
  `retryPolicy` 与 `result` 结构，测试可注入假 reporter。
- 日志仍写 `~/Library/Logs/Open TJ/connections-go.jsonl`（MyGo 的 `PathLogs`
  正好是 `~/Library/Logs/Open TJ`，行为不变）。
- 流量使用现有 `client.TrafficMeter` + `BridgeStats`，每秒采样一次，
  `win.Update` 更新 `uploadBPS/downloadBPS/history`。

### 3.3 状态模型（Go 版 `VPNModel`）

```go
type Controller struct {
    win  *mygo.Window
    mu   sync.Mutex

    username, password, method string
    remember, showPassword     bool

    status, detail             string
    active, stopping, connected bool
    connectedAt                time.Time

    uploadBPS, downloadBPS     float64
    uploadBytes, downloadBytes int64
    downHistory, upHistory     []float64 // 60 点

    entries  []LogEntry // 上限 2000
    logLevel, logSearch string
    followLogs bool
    expanded map[int]bool

    cancel  context.CancelFunc
    stopped chan struct{}
    wake    chan struct{}
    sleeping bool

    session generation // 防旧 goroutine 覆盖新会话(对应 sessionID)
}
```

- 所有字段只在主线程或 `win.Update` 内修改；网络 goroutine 只通过回调 +
  `win.Update` 进入。
- `stop` → `cancel()`；`sleep`/`wake` → 与现有语义一致（睡眠取消当前会话，
  唤醒后重新连接）。
- 关闭窗口 → `Hide()`（保留在菜单栏）；退出 → `OnBeforeQuit` 里
  `PreventDefault` + 断开 + 网络恢复回执后 `App.Quit()`（对应
  `applicationShouldTerminate` + `terminateLater`）。

### 3.4 Swift → Go 映射

| Swift | Go |
| --- | --- |
| `@Published` 字段 | `Controller` 字段 + `win.Update` |
| `VPNModel.consume(_:)` | 删除；`Reporter` 方法直接赋值 |
| `VPNModel.connect()` | `Controller.Connect()` → `go supervisor.Run(...)` |
| `VPNModel.disconnect()` | `Controller.Disconnect()` → `cancel()` |
| `send("stop"/"sleep"/"wake")` | 直接调用 / channel |
| `saveCredentials()` + Security | `mygo.Secrets` |
| `NSWorkspace.willSleep/didWake` | `mygo.Power.OnSuspend/OnResume` |
| `Dashboard` / `SpeedColumn` / `TrafficGraph` | `view_main.go` + `ui/chart` |
| `LogPanel` (LazyVStack) | `ui.List` + `ListState` |
| `MenuBarPanel` + `MenuBarExtra` | `NewFlyout` + `NewTray` |
| `AppDelegate` 窗口/退出策略 | `OnBeforeQuit` / `OnWindowAllClosed` / `Window.OnClose` |
| `WindowSizing` / `SplitPersistence` | `Window.SetContentSize`、`ui.Split` + `StateKey` |
| `ScrollPauseObserver` | `ListState.FollowEnd=false`（用户上滚即停止跟随） |
| `--self-test` | `go test` + `ui.NewTester` |
| `Info.plist` / 打包脚本 | `mygo.json` + `go tool mygo build` |

### 3.5 应用标识与数据兼容（需明确决策）

| 项 | 现状 | MyGo 默认 | 建议 |
| --- | --- | --- | --- |
| Bundle ID | `local.motionvpn.prototype` | `mygo.json.identifier` | 迁移期保持不变，避免权限/钥匙串失效 |
| 用户数据 | `~/Library/Application Support/MotionVPNPrototype/go-backend` | `~/Library/Application Support/<App Name>` | 新版本不再需要复制后端；旧目录可保留为空或清理 |
| 日志 | `~/Library/Logs/Open TJ/connections-go.jsonl` | `PathLogs` = 同名目录 | 无需迁移 |
| 钥匙串 | service `local.motionvpn.prototype.credentials`，JSON 值 | `Secrets` 按 identifier + key 存储 | 一次性读取旧条目迁移到 `Secrets`，成功后提示或删除旧条目 |
| helper socket | `os.MkdirTemp` | 同 | 不变 |

## 4. 分阶段计划

### 阶段 0：骨架与依赖（0.5 天）

- `git mv backend/internal/client apps/desktop/internal/client`，
  修正 `protocol_vectors_test.go` 的相对路径（`../../../testdata` →
  仓库根 `testdata`）。
- 新建 `apps/desktop/go.mod`，`go get -tool github.com/egoist/mygo/cmd/mygo@v0.8.3`。
- `mygo.json`（`identifier` 暂用 `local.motionvpn.prototype`，`macos.minimumSystemVersion: "14.0"`）。
- `main.go` 只开空窗口 + 托盘，验证 `go tool mygo dev` 可启动、可打包。

验收：`go build ./...`、`go test ./...` 通过；`go tool mygo build -platform darwin/arm64` 产出可启动 app。

> 本机注意：`proxy.golang.org` 只有 IPv6 且当前网络不通，拉依赖需
> `GOPROXY=https://goproxy.io,direct`（`go.sum` 与 CI 不受影响）。

### 阶段 1：监督器同进程化（1 天）

- 把 `cmd/motionvpn/main.go` 拆成 `supervisor.go`（`Run`/重连/睡眠）与
  `helper/main.go`（原样 `client.RunHelper`）。
- 定义 `Reporter`，实现 `jsonlReporter`（仅写日志文件）与 `Controller` 适配。
- 把 `--diagnose` 保留在 app 的参数分发中。
- 迁移现有测试：`policy_test.go` → `supervisor_test.go`，其余 `client` 测试原样。

验收：`go test -race ./...`，覆盖登录失败重试、睡眠唤醒、stop 后网络回执。

### 阶段 2：主窗口与托盘（2–3 天）

- `view_main.go`：状态头、连接时长、断开按钮、连接表单（用户名/密码/认证方式/
  记住密码）、连接信息、设置折叠区。
- `view_traffic.go`：上下行速度、累计、60 秒折线、峰值/刻度。
- `view_log.go`：级别筛选、搜索、跟随、展开详情、复制/清空；`ui.List` 虚拟滚动。
- `tray.go` + `view_menu.go`：模板托盘图标、点击弹出 flyout、右键菜单、退出。
- 窗口关闭 → 隐藏；`OnActivate` 重新打开；`⌘L` 切换日志侧栏。

验收：与现有 SwiftUI 行为逐项对照（见第 6 节清单）。

### 阶段 3：凭据与系统集成（1 天）

- `mygo.Secrets` 替换 Security 框架；旧条目一次性迁移。
- `mygo.Power` 替换 `NSWorkspace` 通知。
- 认证方式、用户名、remember 存 `UserDefaults` 等价物（`mygo` 未直接提供偏好
  API 时用 `PathUserData/preferences.json`；或用 `Secrets` 只存密码）。

### 阶段 4：测试与构建（1 天）

- `make test`：`cd apps/desktop && go test -race ./...`，删除 swiftc 自测。
- 视图测试用 `ui.NewTester` 覆盖：分帧后流量更新、日志筛选/搜索、2000 条上限、
  图表刻度、单位格式化、展开项淘汰、退出拦截。
- `scripts/build.sh`：`go tool mygo build -platform darwin/universal`；
  `scripts/package.sh`：改用 mygo 产出的 `.dmg`/ZIP（或继续 ditto 打包
  `build/Open TJ.app`）。
- `scripts/release.sh`：`mygo.json` 的 `macos.notarize` 或保留现有 notarytool。
- CI：`setup-go` 读 `apps/desktop/go.mod`；产物校验从两个二进制改为
  app + `Resources/bin/opentj-helper`。

### 阶段 5：清理（0.5 天）

- 删除 `apps/macos/`、`backend/`、相关 Makefile 目标。
- 更新 `README.md`、`docs/ARCHITECTURE.md`、`docs/RELEASE_CHECKLIST.md`。
- Android 的 `testdata` 与 workflow 不动。

## 5. 关键实现草图

```go
// main.go
func main() {
    // 提权和诊断必须在 mygo 启动之前，避免以 root 初始化 UI。
    if handleHelperOrDiagnose(os.Args) { return }

    ctrl := NewController()
    mygo.App.OnWindowAllClosed(func() { /* 菜单栏常驻，不退出 */ })
    mygo.App.OnActivate(func(visible bool) { if !visible { ctrl.ShowMain() } })
    mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
        if ctrl.Active() { e.PreventDefault(); ctrl.DisconnectThen(mygo.App.Quit) }
    })
    mygo.App.WhenReady(func() {
        ctrl.Start() // 主窗口、托盘、flyout、凭据恢复
    })
    if err := mygo.App.Run(); err != nil { log.Fatal(err) }
}
```

```go
// controller.go（节选）
func (c *Controller) Connect() {
    c.mu.Lock()
    if c.active { c.mu.Unlock(); return }
    c.username = strings.TrimSpace(c.username)
    cred := client.Credentials{Username: c.username, Password: c.password, Method: c.method}
    c.active, c.stopping, c.connected = true, false, false
    c.status, c.detail = "正在连接", "正在验证 VPN 账号…"
    ctx, cancel := context.WithCancel(context.Background())
    c.cancel = cancel
    c.resetTrafficLocked()
    c.mu.Unlock()
    c.win.Invalidate()

    go func() {
        supervisor.Run(ctx, cred, c) // 内部循环：登录→提权→转发→重连
        c.win.Update(func() {
            c.active, c.stopping, c.connected = false, false, false
            c.cancel = nil
        })
        c.onStopped() // 退出流程用
    }()
}
```

```go
// view_main.go（节选）
func (c *Controller) viewMain(ctx *ui.Context) {
    t := ctx.Theme()
    ui.Column(ctx).Fill().Children(func() {
        header(ctx, c, t)     // 状态、时长、断开
        ui.Divider(ctx)
        ui.Scroll(ctx).Grow(1).Children(func() {
            ui.Column(ctx).Padding(22).Gap(20).Children(func() {
                if !c.connected { ui.Text(ctx, c.detail).TextColor(t.TextMuted) }
                trafficCard(ctx, c, t)
                ui.Form(ctx, func() {
                    ui.Field(ctx, "VPN 用户名", func() { ui.TextInput(ctx, &c.username) })
                    ui.Field(ctx, "VPN 密码", func() { ui.TextInput(ctx, &c.password).Password() })
                    ui.Field(ctx, "认证方式", func() {
                        ui.Select(ctx, &c.method, []string{"LDAP3", "LDAP2", "LDAP", "default_method_localdb"})
                    })
                    ui.Field(ctx, "在钥匙串中记住密码", func() { ui.Switch(ctx, &c.remember) })
                }).Disabled(c.active)
                if !c.active {
                    if ui.PrimaryButton(ctx, "连接 VPN").Clicked() { c.Connect() }
                }
            })
        })
    })
}
```

```go
// tray.go / view_menu.go（节选）
tray, _ := mygo.NewTray(mygo.TrayOptions{
    ID: "opentj", Icon: iconPNG(), IconIsTemplate: true, ToolTip: "Open TJ",
})
tray.OnClick(func() { panel.Toggle() })
tray.SetTitle(fmt.Sprintf("%5.1f MB/s", c.downloadBPS/1e6)) // 用 U+2007 定宽
panel = mygo.NewFlyout(mygo.FlyoutOptions{
    Tray: tray, Placement: mygo.PlacementBottom, Gap: 4,
    Width: 280, Height: 300, Focusable: true, Popover: true,
    Content: ui.View(c.viewMenu),
})
```

```go
// controller_test.go
func TestViewShowsTrafficAndFiltersLogs(t *testing.T) {
    c := newTestController()
    tt := ui.NewTester(c.viewMain, 420, 600)
    c.ApplyTraffic(client.TrafficSample{DownloadBPS: 4096, UploadBytes: 2048})
    if !tt.HasText("4.00 KB/s") { t.Fatal(tt.Texts()) }
    c.AddLog("warning", "retry test", "backend")
    c.logLevel = "warning"
    if !tt.HasText("retry test") { t.Fatal(tt.Texts()) }
}
```

## 6. 行为对照清单（阶段 2 验收）

- [ ] 连接中状态：鉴权 → 系统授权弹窗提示 → 网络预检日志 → 已连接 + 接口名
- [ ] 断开：按钮禁用、显示「正在恢复网络…」、回执 clean/恢复失败提示
- [ ] 失败/重连：指数退避秒数、错误详情、`result` JSON 可展开
- [ ] 睡眠：暂停并清空瞬时速率；唤醒自动重连
- [ ] 流量：上下行速率、累计、60 秒历史、峰值、刻度（2 的幂）、单位格式
- [ ] 日志：级别计数、筛选、搜索、跟随、上滚停止、展开、复制、清空、2000 条上限
- [ ] 凭据：记住密码、重启后回填、关闭记住时删除
- [ ] 窗口：关闭继续驻留菜单栏、首次提示、`⌘L` 日志侧栏、尺寸/位置记忆
- [ ] 退出：先断开并等待网络回执，再退出；拒绝退出时保持连接
- [ ] 菜单栏：图标随状态变化、速率标题、连接/断开/打开/退出

## 7. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| MyGo `v0.8.x` 仍快速迭代 | API 变动 | 锁 `v0.8.3`，升级单独 PR；核心 UI 与 `client` 解耦 |
| 提权 helper 的签名/公证 | 发布失败 | 用 `resources/darwin-universal/bin/` + `macos.helperEntitlements`；本地先跑 `codesign --verify --deep --strict` |
| 旧钥匙串条目读不到 | 用户需重输密码 | 用 `security find-generic-password` 一次性迁移；失败则回退为提示重输，不清除旧条目 |
| `App.PathUserData` 目录变更 | 旧配置丢失 | 迁移期显式读写旧路径；用户名/认证方式自行存 JSON |
| macOS 26 Liquid Glass | 视觉差异 | MyGo `glass` 插件；或保持普通 `bordered` 样式，仅 macOS 26 启用 |
| 图表/日志性能 | 掉帧 | `ui.List` 虚拟滚动；图表只保留 60 点并复用 `ui/chart`；流量更新 1 Hz |
| 单人维护的框架可用性 | 长期风险 | `internal/client` 与 UI 严格分层，必要时可退回 SwiftUI/其他 Go GUI |

## 8. 待决问题

1. 是否保留历史 Bundle ID `local.motionvpn.prototype`（建议保留到 1.0）。
2. 旧钥匙串条目迁移后是否删除（建议保留并标记，不主动删除）。
3. helper 用独立二进制还是复用 app 二进制 `--helper`（建议独立）。
4. 是否顺手把菜单栏面板从「独立窗口」改为真正的 `NewFlyout` 弹出层
   （建议是，交互与 `MenuBarExtra(.window)` 一致）。
5. 是否借机把 `apps/desktop` 扩展到 Windows/Linux（MyGo 支持，但校园 VPN
   的 utun/路由逻辑目前只有 macOS 实现，本次不纳入）。
