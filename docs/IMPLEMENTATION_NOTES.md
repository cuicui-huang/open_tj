# Sprint 1 实现说明

对应 `docs/PRODUCT_SPEC.md` §5。记录实现取舍与偏差，**不属于规格的延后项单独列在 §3**。

---

## 1. 已实现

### S1-A 诊断可视化
- `Controller.Progress` 在 `component == "diagnostics"` 且 `probes != nil` 时，
  经 `c.post` 把结果存进 `c.probes` / `c.probesAt`，并清 `checking` 标志。
- 概览新增「连通性」卡片：每行主机名、三段耗时（DNS/TCP/TLS）、状态点；
  有跳转时缩进列出 `→ host`；未连接时是空状态，不显示假数据。
- `ExplainProbe` 纯函数放在 `internal/app/format.go`，7 个分支 + 长错误截断，
  `TestExplainProbe` 逐条覆盖。
- 刷新按钮调用 `c.CheckConnectivity()`；它在未连接时直接返回，不会空跑。

### S1-B 设置页与桌面系统能力
- 新增 `pageSettings`（`⌘4`）：常规（开机自启、连接成功通知）、连接（启动时自动连接、
  认证方式）、关于（版本、网关、许可）。
- `Config.SetOpenAtLogin` / `Config.Notify` 默认值即 `mygo.App.SetOpenAtLogin` 与
  `mygo.NewNotification`，测试可替换。
- 开机自启失败时错误显示在开关下方并写日志，**不改写已保存的偏好**。
- 连接通知只在「状态由非连接变为已连接」的那一次触发（`became` 标志），不会每帧重发。
- `Preferences` 新增 `open_at_login` / `disable_notify` / `auto_connect`。
  **通知用反向字段 `DisableNotify`**：老配置文件没有该字段时零值为 `false`，
  于是通知默认开启，满足规格「老配置读进来通知是开的」。
  `TestOldFileKeepsNotificationsOn` 验证这一点。

### S1-C 视觉与排版
- 主色 `ui.Oklch(0.52, 0.13, 258)`（深色模式 `0.62`）；红色只用于错误态。
- 导航选中项：3px 竖条 + 淡背景 + 加粗，**不再实心填充**。
- 速率数字与单位用 `ui.Baseline` 对齐，去掉凑基线的 `Margin`。
- 侧栏底部改为真实数据：网关来自 `c.Gateway()`，第二行是认证方式（连接后追加接口名）；
  取不到就不渲染。
- 快捷键提示 `shortcutHint(n)` 按 `runtime.GOOS` 返回 `⌘n` 或 `Ctrl+n`。
- 缺凭据时按钮旁给出具体缺口（`MissingCredential()`：还缺用户名 / 还缺密码）。

### S1-D 空状态引导
- `New()` 中 `prefs.Username` 为空时 `Page = pageAccount`，账号页顶部显示
  「首次使用？用你的学号和校园网密码登录」。`TestFirstLaunchOpensAccount` 覆盖。

## 2. 顺带修掉的既有缺陷

| 问题 | 处理 |
| --- | --- |
| `MaybeAutoConnect()` 已实现但**从未被调用** ——「启动时自动连接」开关形同虚设 | 在 `main.go` 的 `WhenReady` 末尾调用（主线程，因为 `Connect` 改 UI 状态） |
| 速率显示 `4.40 MB/s`，多余尾零 | `RateParts` 裁掉尾零，保留 `1.25` 这类有意义的小数；补了两个测试 |
| `TestAutoConnectNeedsSavedAccount` 用非阻塞 `select ... default` 检查 goroutine 启动，随机失败 | 改为带 2 秒超时的等待 |

## 3. 未做（明确延后，不是遗漏）

- **Sprint 2 / 3 的全部内容**（重连倒计时、被顶下线通知、updater 接入、菜单栏图标状态、
  分流规则、Linux 支持取舍）—— 规格已列明不属于本次。
- **字体宽度限制下的长文案截断**：错误翻译文案最长约 24 个中文字符，
  在 940px 宽的连通性卡片里能完整显示；更窄的窗口会单行裁切。**未做省略号**，
  因为 MyGo 的 `SingleLine()` 已经裁切，加自绘省略号收益低于成本。
- **首启动自动聚焦输入框**：`ui.TextInput` 的聚焦需要窗口级焦点请求，
  headless 测试无法验证，且实际窗口创建顺序由 MyGo 控制；
  改为保持页面直达账号页（比聚焦更可靠地解决「找不到入口」）。
- **Windows 的开机自启**：MyGo 文档明确说明需要管理员权限的 VPN 客户端应注册
  Task Scheduler 任务而不是调用 `SetOpenAtLogin`。当前实现调用的是后者，
  在 Windows 上生效但以非提权令牌启动。**这是一个已知的平台差异**，
  建议在 Sprint 3 处理 Windows 提权启动时一并改为计划任务。

- ~~**手动刷新时不比较隧道 DNS**~~ —— **已在 2026-11-09 的审计轮修好**：
  `Reporter.Connected` 增加了 `dns []string` 参数，supervisor 传入 `cfg.DNS`，
  controller 据此构造真正的隧道 resolver。详见 `docs/AUDIT_2026-11-09.md` §6。

## 3.5 排版紧凑化（第二轮，用户反馈）

第一版交付后用户反馈「排版可以更紧凑」。按下面的基准整体收紧，数值也写进了
`docs/PRODUCT_SPEC.md` §3.3 与 `design/README.md`：

| 位置 | 之前 | 之后 |
| --- | --- | --- |
| 内容区外边距 | `24, 26, 28, 26` | `18, 20` |
| 区块间距 | `16` / `18` | `11`；设置页分组之间 `16`（保证组标题归属清晰） |
| hero 卡内边距 | `20` | `14, 16` |
| 卡片内边距 / 圆角 | `16` / `16` | `13, 14` / `12`（hero、流量卡 `14`） |
| 状态环 | `52` | `40` |
| 页面标题 | `22` | `19` |
| 速率数字 / 单位 | `28` / `13` | `24` / `12` |
| 列表行高 | `36` | `30` |
| 侧栏项高 | `34` | `30` |

同时改掉一处信息冗余：**未连接时不再渲染两张只有 `0` 的流量卡**，改为单行
「连接后显示实时速率 · 最近 60 秒」。这比压缩卡片更有效——空状态下两张卡
占了 130px 却在传达零信息。

结果：默认 860×600 窗口下概览全部内容无需滚动即可看全（`design/compact-narrow.png`）。

## 4. 验证

```
make test   # go test -race ./... + go vet ./...  → 全部通过
make build  # darwin/arm64 → Open TJ.app 13.3 MB + dmg
```

构建产物已实际启动确认（窗口正常、设置页渲染正常、版本读作 0.4.0）。
界面自检用 MyGo 的 headless 渲染（`ui.NewTester` + `tt.Image()`）导出 PNG 逐页核对，
临时截图测试已删除，仓库中无残留。
