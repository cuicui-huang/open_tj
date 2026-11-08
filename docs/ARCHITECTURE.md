# 架构

## 总览

```text
apps/desktop/   macOS：Go 单一进程 + MyGo 原生 UI + 提权 helper 子进程
apps/android/   Android：Kotlin/Compose + VpnService，独立实现
testdata/       Go/Kotlin 共用协议样例
```

两端独立实现网关协议，用 `testdata/vpn-protocol.json` 校验一致性；Android
不运行 macOS 的 helper。

## macOS

### 目录职责

| 包 | 职责 |
| --- | --- |
| `apps/desktop` (`main`) | 程序入口、`--helper`/`--diagnose` 分发、MyGo 应用生命周期、窗口与菜单栏接线 |
| `internal/app` | `Controller` 状态机、主窗口/日志/菜单栏视图、托盘图标、格式化函数 |
| `internal/vpn` | 登录、隧道编排、重连与睡眠策略、对 UI 的 `Reporter` 接口 |
| `internal/client` | 网关协议、TLS、helper 握手、utun、路由/DNS/代理事务、流量统计、转发桥、诊断探针 |
| `internal/credentials` | 钥匙串读写与旧版条目迁移 |
| `internal/settings` | 用户偏好 JSON（账号名、认证方式、窗口布局） |
| `internal/logging` | JSONL 诊断日志（2 MB 轮转） |

依赖方向单向：`main → app → vpn → client`；`vpn` 只依赖 `client`，不依赖 UI。

### 运行路径

MyGo 的 `App.Run` 在主 goroutine 上运行事件循环，窗口内容由
`ui.View(controller.ViewMain)` 每帧根据状态构建。用户操作直接修改
`Controller`，`Window.Invalidate` 请求下一帧；监督器 goroutine 通过
`Window.Update` 回到主线程更新状态。

连接流程：

1. `Controller.Connect` 读取表单、保存偏好与钥匙串，启动 `vpn.Supervisor.Run`。
2. 监督器登录网关，读取隧道配置，进行网络预检（出口必须是物理网卡）。
3. 通过 `osascript ... with administrator privileges` 以 root 重新执行**同一份
   应用二进制**并带 `--helper <socket> <token>`；`main` 在初始化 MyGo 之前处理该
   参数，因此特权进程不会加载或显示 UI。
4. helper 创建 utun，事务式配置路由、DNS、代理与 IPv6，写回执；监督器开始转发。
5. 断开、睡眠或失败时，helper 回滚网络并写 `cleanup_confirmed`；监督器核对
   父进程网络快照与外层路由后，才允许自动重连。
6. 管理员授权和 utun 配置可能耗时数十秒，期间监督器按网关心跳间隔发送
   ATP 心跳；网关主动终止会话（原因码 1/8）时不自动重连，并提示账号是否在
   其他设备在线。

睡眠/唤醒由 MyGo 的 `Power.OnSuspend/OnResume` 驱动，语义与旧版
`NSWorkspace` 通知一致。

### 界面

- 主窗口：状态头、连接时长、流量卡片（上下行速率、60 秒折线、峰值、累计）、
  连接表单、连接/断开状态。
- 日志侧栏：级别筛选、搜索、跟随、展开详情、复制、清空；`ui.List` 虚拟滚动，
  上限 2000 条。
- 菜单栏：`NewTray` 模板图标 + `NewFlyout` 面板，显示速率与连接/断开/打开/退出。
- `⌘L` 切换日志侧栏；关闭窗口改为隐藏，应用继续驻留菜单栏；退出前先断开并
  等待网络恢复完成。

### 兼容标识

- Bundle ID / `mygo.json` identifier：`local.motionvpn.prototype`。
- Keychain service：`local.motionvpn.prototype.credentials`；首次启动会读取旧
  Swift 客户端写入的条目并迁移到 MyGo 的账号式存储。
- 日志：`~/Library/Logs/Open TJ/connections-go.jsonl`。
- 旧的 `~/Library/Application Support/MotionVPNPrototype/go-backend` 不再使用，
  也不再复制后端可执行文件。

这些历史标识保留以兼容现有账号与配置。若正式发布前改为自己控制的反向域名，
应同时设计钥匙串/偏好迁移，不要只修改 `mygo.json`。

### 测试

`make test` 运行 `go test -race ./...` 与 `go vet ./...`：

- `internal/client`：协议向量、TLS、helper、路由回滚顺序、转发、流量、诊断。
- `internal/vpn`：重连策略、授权脚本可编译（`osacompile`）。
- `internal/app`：状态机、日志边界与筛选、格式化；视图用 `ui.NewTester`
  渲染并点击，不启动窗口。
- `internal/settings`、`internal/logging`、`internal/credentials` 的独立单测。

真实 VPN 登录、管理员授权与网络修改不在单元测试内，见
[发布验收](RELEASE_CHECKLIST.md)。

## 后续维护边界

- `internal/client` 与 UI 严格分层：界面变化不需要改协议代码，协议变化只需
  扩展 `vpn.Reporter` 与 `Controller`。
- helper 仍是每次授权的子进程，不是已安装的 LaunchDaemon；若改用
  ServiceManagement，需要独立的安装、签名验证与升级设计。
- MyGo 版本锁定在 `apps/desktop/go.mod`，升级单独提交并跑完整测试；原生 UI
  不依赖 WebView，未来也可扩展到 Windows/Linux，但网络配置目前仅实现 macOS。
