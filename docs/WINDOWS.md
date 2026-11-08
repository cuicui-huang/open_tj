# Windows 使用与实现说明

Windows 10/11（arm64 / amd64）从 0.4.0 起支持连接，属于实验性功能。下载
`OpenTJ-<版本>-windows-<架构>.zip`（免安装包），解压后运行其中的
`Open TJ.exe`；首次连接会弹出 UAC 管理员授权。`wintun.dll` 必须与 exe 保持同目录。

包内已带 `wintun.dll`。exe 与 DLL 均未签名，SmartScreen 可能提示“未知发布者”，
需要手动允许运行。正式分发前应配置代码签名证书。

## 工作方式

与 macOS 共用同一套网关协议和流量转发，只有主机网络配置不同：

| 环节 | macOS | Windows |
| --- | --- | --- |
| 提权 | `osascript … with administrator privileges` | `ShellExecute("runas")` + UAC |
| helper 通信 | Unix socket + 随机令牌 | `127.0.0.1` 回环 TCP + 随机令牌 |
| 虚拟网卡 | WireGuard `utun` | Wintun（`OpenTJ` 适配器） |
| 路由 | `/sbin/route` | `route.exe` |
| DNS | `networksetup` 修改网络服务 | `Set-DnsClientServerAddress` |
| 代理 | `networksetup -set*proxystate` | `HKCU\…\Internet Settings\ProxyEnable` |
| IPv6 | 关闭网络服务 IPv6 + reject 路由 | 临时关闭各适配器 IPv6 绑定 |
| 网络快照 | 网络服务 DNS/代理/IPv6 | 接口 DNS、各适配器 IPv6、系统代理 |

连接时把物理接口的 DNS 改为网关下发的 VPN DNS，添加 `0.0.0.0/1`、`128.0.0.0/1`
两条隧道路由和一条到网关的物理主机路由；断开时按日志逆序回滚，并重新读取快照
核对是否恢复。

## 已知限制

- 仅 IPv4；IPv6 通过关闭适配器绑定阻断，未实现 IPv6 隧道。
- 不支持按应用/域名分流，全部 IPv4 流量进入隧道。
- 修改的是当前用户的系统代理，其他用户不受影响。
- 适配器名称固定为 `OpenTJ`，同时运行多个实例会冲突。
- Wi-Fi 切换、休眠唤醒沿用监督器重连逻辑，尚未在 Windows 真机充分验证。
- arm64 构建可运行，但 Wintun 与驱动兼容性需要真机验收。

## 开发

```sh
cd apps/desktop
go build ./...                       # 编译检查
GOOS=windows GOARCH=amd64 go build . # 交叉编译
go tool mygo build -platform windows/amd64 -skip-installer
```

`go build` 直接运行生成的 exe 时，需要把 `resources/windows-<arch>/wintun.dll`
复制到 exe 同目录；`mygo build` 会自动复制平台资源。

## 验收清单

- [ ] UAC 允许后成功创建 Wintun 适配器并分配地址
- [ ] 路由与 DNS 生效：`Find-NetRoute -RemoteIPAddress 1.1.1.1` 指向 `OpenTJ`
- [ ] 断开后物理接口 DNS、系统代理、IPv6 绑定全部恢复
- [ ] UAC 取消、网关断线、睡眠唤醒、Wi-Fi/有线切换行为正确
- [ ] 未签名 exe 的 SmartScreen 提示可被接受（正式版应签名）
