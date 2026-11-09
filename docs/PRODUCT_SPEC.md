# Open TJ 产品规格 v1

> 作者：Alma（产品经理）· 2026-11-09
> 目标读者：实现者（Claude Code）
> 本文件是验收依据。任何与本文冲突的既有实现，以本文为准。

---

## 1. 一句话定位

**一个你愿意常驻菜单栏的同济校园 VPN 客户端。**
打开就知道通不通，点一下就通，断了它自己回来，出事说人话。

## 2. 现状诊断（已核实，不是猜测）

| # | 问题 | 证据 |
| --- | --- | --- |
| P1 | 诊断能力写好了但用户看不见 | `internal/client/diagnostics.go` 有 DNS/系统解析/TCP/TLS/首字节全套探针，只能 `--diagnose` 命令行触发；UI 里零展示 |
| P2 | 没有设置页 | `settings.Preferences` 只有 5 个字段（Username/Method/Remember/BackgroundCloseExplained/ShowLog/SplitWidth），`ShowSettings` 字段存在但从未被读 |
| P3 | 桌面应用该有的系统能力全缺 | 全仓库 grep 无 `SetOpenAtLogin`、无 `NewNotification`、无 updater 插件；而 MyGo v0.8.3 三者都自带 |
| P4 | 首次启动是死路 | 概览文案「输入同济 VPN 账号后连接」无跳转入口，用户要自己猜去点侧栏 |
| P5 | 强调色过载 | 选中导航项是整块实心红胶囊，hero 卡片粉底，红色在同济语境里还像警告色 |
| P6 | 数字排版不成对 | `0` 用 26px、`B/s` 用 12px 靠 `.Margin(0,0,4,0)` 硬凑基线 |
| P7 | 侧栏底部是假状态 | `"IPv4 · 同济大学"` 硬编码字符串 |
| P8 | 快捷键不分平台 | 侧栏永远渲染 `⌘1/⌘2/⌘3`，Windows/Linux 上也显示 ⌘ |
| P9 | 失败只说「连不上」 | `Connect()` 的空账号分支只设 `c.detail`，不指出是缺用户名还是缺密码，按钮禁用也不给理由 |

## 3. 视觉方向（明确指定，不要自创）

> **先看 `design/` 目录里的四张图**（`compact-overview.png`、`compact-narrow.png`、
> `compact-idle.png`、`compact-settings.png`）。它们是用 MyGo headless 渲染器输出的
> 真实像素，是本节的可视化等价物，`design/README.md` 列了完整的间距基准数值。
> 若实现结果与图明显不符，以图为准。

### 3.1 颜色
- **主色改为同济蓝**：`ui.Oklch(0.52, 0.13, 258)`（深海军蓝，接近 #0f4c9c 的观感）。
  红色降级为**只用于错误态**（`ctx.Theme().Danger` 一类的语义色）。
- 强调色只出现在三个地方：主操作按钮、当前状态指示点、「已连接」徽标。**导航选中项不再用实心填充**，改用「左侧 3px 竖条 + 文字加粗 + 淡背景 `p.hover`」。
- hero 卡片底色从 `p.accent.Alpha(0.07)` 改为中性 `p.card`，把颜色预算留给状态本身。

### 3.2 状态语义色（唯一定义处，全应用引用）
| 状态 | 颜色 | 表现 |
| --- | --- | --- |
| 未连接 | `p.muted` | 空心环 |
| 连接中 | 警告色 | 环内旋转 spinner + 环有呼吸动画 |
| 已连接 | 成功绿 | 实心环 + 时长计时 |
| 出错 | 危险红 | 实心环 + 图标改 `exclamationmark.triangle.fill` |

### 3.3 排版
- 数字与单位**共用同一基线**：用 `ui.Row(ctx).AlignItems(ui.Baseline)`（若无此枚举则用同 `fontSize` 的 `AlignItems(ui.End)` 并去掉 magic `Margin`）。速率数字 28px/680，单位 13px/500，`TextColor(p.muted)`。
- 空状态禁止只留白：每个空区域必须有图标 + 一句可操作的话。
- 侧栏宽度 **196**，侧栏项高 `30`；主内容外边距 `Padding(18, 20)`，区块之间 `Gap(11)`。
- 卡片内边距 `13, 14`，圆角 `12`（hero 与流量卡 `14`）；列表行高 `30`。
- 状态环 `40`，标题 `19`，速率数字 `24` / 单位 `12`。

### 3.4 平台化文案
- 快捷键提示：新增 `isMac := runtime.GOOS == "darwin"`，非 mac 渲染 `Ctrl+1` 等。
- 删除所有「VPN 连接支持 macOS 和 Windows。」这类硬编码平台断言，改为按 `vpn.Supported()` 出不同文案。

## 4. 信息架构

```
概览 ── 状态环 + 时长 + 主操作 + 诊断卡 + 流量卡 + 会话信息
账号 ── 凭据表单（连接中禁用）
诊断 ── 连通性检测结果（新页面）
设置 ── 常规 / 连接 / 关于（新页面）
日志 ── 保留现有能力，收进设置的下级或保留为独立页（由实现者按 4 项上限取舍）
```
侧栏项数上限 **4 项**，超过就归组。`⌘1..⌘4` 对应。

## 5. 迭代切片

### Sprint 1（本次交付）

#### S1-A 诊断可视化（P1 修复，最高优先）

**已核实的真实链路**（不要重新设计，只补齐断掉的那一段）：
1. `internal/vpn/supervisor.go:267` 连接建立后启动探针 goroutine，跑完调
   `reporter.Progress(client.ProbeSummary(results), "diagnostics", results)` —— **探针数据已经在这里了**。
2. `internal/app/controller.go:511` `Progress()` 收到 `probes` 参数后**直接丢弃**：
   只把摘要写进 JSONL 日志文件，`probes` 从未赋给任何字段，UI 完全看不到。
3. 断开时还会再跑一次，结果塞进 `Result.Site.ConnectivityProbes`，同样只有日志能看。

**所以要做的只是：在 UI 侧接住它。**

- `Controller` 新增字段 `probes []client.ProbeResult`，并新增 `probesAt time.Time`。
- 修改 `Progress()`：当 `component == "diagnostics" && probes != nil` 时，
  在 `c.post` 内赋值 `c.probes = probes; c.probesAt = time.Now()`。**必须放进 `post` 队列**，
  不要在外面直接写字段。
- 概览新增「连通性」卡片，从上到下：
  - 标题行：`连通性` + 右侧「更新于 <相对时间>」或「检测中」+ 一个刷新图标按钮。
  - 每条 `ProbeResult` 一行：主机名（从 `URL` 解析，如 `xlcp.tongji.edu.cn`）/ 状态点 / 关键数值
    `DNS <系统DNS ms>ms · TCP <TCP ms>ms · TLS <TLS ms>ms`。
  - `r.Error != ""` 时状态点为错误色，右侧数值位改为**翻译后的人话**（见下）。
  - `r.Redirects` 存在时，行下方缩进显示 `→ <host>` 与各自状态。
- 空状态（未连接、`c.probes` 为空）：图标 + 「连接后自动检测校园网连通性」，**不显示假数据、不显示按钮**。

**错误翻译（必须做，不许直接抛 err 字符串）**：新增一个纯函数
`func ExplainProbe(r client.ProbeResult) string`，放在 `internal/app/format.go`，
可单测。至少覆盖：
| 条件 | 输出 |
| --- | --- |
| `SystemDNSError != "" && VPNDNSError == ""` | 「系统 DNS 解析失败，隧道内正常；可能是本机 DNS 配置问题」 |
| `SystemDNSError != "" && VPNDNSError != ""` | 「隧道内也无法解析，请尝试断开重连」 |
| `VPNDNSError != ""` 且系统正常 | 「隧道 DNS 不可用，正在使用系统 DNS」 |
| DNS 正常但 `TLSMS == 0 && TCPMS > 0` | 「能连通但证书握手未完成，可能被中间设备拦截」 |
| `Status >= 500` | 「校园网服务端异常（HTTP <状态码>），稍后重试」 |
| `Error != ""` 且有内容 | 原文截断，前缀「检测失败：」 |
| 其余 | 「正常」 |

**验收**：
- `format_test.go` 有 `TestExplainProbe`，覆盖上表每一行，至少 7 个 case。
- `controller_test.go` 有测试直接调用 `c.Progress("连接诊断：…", "diagnostics", []client.ProbeResult{...})`
  并 drain `postQueue` 后断言卡片文案出现（`tt.HasText("连通性")`、`tt.HasText("xlcp.tongji.edu.cn")`）。
- 未连接时 `tt.HasText("连接后自动检测校园网连通性")` 为真。
- `go test -race ./internal/app` 通过。

#### S1-B 设置页与桌面系统能力（P2/P3 修复）
新增 `pageSettings`，含三项：

1. **开机自启**（默认关）— `mygo.App.SetOpenAtLogin(bool)` / `OpenAtLogin()`。
   开关失败要在开关下方展示错误文案，不许静默失败。
2. **连接成功后发送系统通知**（默认开）— `mygo.NewNotification(mygo.NotificationOptions{...})`，文案：「已连接同济校园网 · <时长>」。仅在**状态由非连接变为已连接**时发一次，禁止每帧重发。
3. **启动时自动连接**（默认关）— 若开启且 `Remember` 为真且有用户名，启动后自动调用 `Connect()`。若凭据缺失，降级为不自动连接并记一条 info 日志。

`Preferences` 新增字段（全部 `omitempty`，保持向后兼容）：
```go
OpenAtLogin   bool `json:"open_at_login,omitempty"`
NotifyOnConnect bool `json:"notify_on_connect,omitempty"`
AutoConnect   bool `json:"auto_connect,omitempty"`
```
> `NotifyOnConnect` 语义是「用户显式设过」——因为默认值应为开，所以实现需用一个 `*bool` 或者反向字段 `DisableNotify`。**由实现者选择，但必须满足：老配置文件（无该字段）读进来后通知是开的。**

**验收**：`settings` 包有测试证明「旧 JSON 不含新字段 → Normalize 后通知为开、自启为关」；`app` 包有测试证明切换开关会写回 Preferences。

#### S1-C 视觉与排版修复（P5/P6/P7/P8/P9）
- 主色改同济蓝，红色仅错误态（见 §3.1）。
- 导航选中项改竖条样式（见 §3.1）。
- 速率数字/单位基线对齐，去掉 magic margin（§3.3）。
- 侧栏底部改为真实信息：`vpn.tongji.cn` + 真实认证方式或当前接口名；取不到就整行隐藏，**不许硬编码**。
- 快捷键提示按平台渲染。
- 空账号/空密码时，主按钮旁给出**具体缺什么**的提示（「还缺密码」/「还缺用户名」），而不是笼统的 `disabled`。

**验收**：`tt.HasText("IPv4 · 同济大学")` 必须为假；新增截图测试可选（见 §6）。

#### S1-D 空状态引导（P4 修复）
首次启动（`prefs.Username == ""`）时：主窗口直接打开到**账号页**，用户名输入框自动聚焦，顶部一句「首次使用？用你的学号和校园网密码登录」。
不引入多步向导（那是 Sprint 3 的事）。

**验收**：`app` 包测试证明 `prefs.Username == ""` 时 `New()` 后 `Page == pageAccount`；有用户名时 `Page == pageOverview`。

### Sprint 2（下一轮，本次不做）
- 自动重连策略可配置 + 重连倒计时可见
- 被顶下线（reason 1/8）时发系统通知并可一键重连
- `plugins/updater` 接入 GitHub Releases + 更新窗口
- 菜单栏图标状态区分（未连接/连接中/已连接）

### Sprint 3（暂缓）
- 分应用代理 / 校园网白名单分流
- Linux 真支持或彻底移除构建目标（二选一，不要再留「界面预览」这种半吊子状态）
- 多账号 / 多网关配置

## 6. 技术约束（务必遵守）

1. **只用 MyGo v0.8.3 已导出的 API**。动手前先 `grep` `~/go/pkg/mod/github.com/egoist/mygo@v0.8.3/`，别臆造方法名。
2. **状态变更必须经 `Post` 回主线程**，沿用 `controller.go` 现有模式。诊断 goroutine 直接改字段会导致数据竞争 —— `go test -race` 会挂。
3. **不用 Swift、不用 WebView、不加 cgo**。界面全靠 `ui` 包。
4. **不加新依赖**，除非无法避免且能说明理由。
5. **必须通过** `make test`（`go test -race ./...` + `go vet ./...`）。
6. **必须通过** `make build`，构建产物能正常启动。
7. 视觉改动要**同时更新对应的 `_test.go` 断言**（现有测试检查了具体文案，改文案就要改断言）。
8. 想要「看见」界面：MyGo 有 headless 渲染，`ui.NewTester(view, w, h)` + `tt.Image()` 可导出 PNG。用它自检排版，别靠想象。

## 7. 不做的事

- 不改 Android 任何代码
- 不改网关协议（`internal/client/protocol.go`、`testdata/`）
- 不重构 supervisor 状态机（Sprint 2 再动）
- 不做自定义主题/换肤
- 不引入 i18n 框架（当前只有中文，别提前抽象）
- 不碰 `.github/workflows` 和版本号

## 8. 交付物

1. 代码改动（Go）
2. 更新后的测试
3. `docs/PRODUCT_SPEC.md` 中若某条无法实现，在 `docs/IMPLEMENTATION_NOTES.md` 里写清原因和替代方案 —— **不许默默跳过**
4. 一段简短的自检报告：做了什么、验收怎么过的、哪些没做及原因
