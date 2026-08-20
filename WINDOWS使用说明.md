# NTR Windows 运行说明

本目录已经包含可直接运行的 Windows 版本，不需要安装 Go，也不需要自行编译。普通 64 位 Intel/AMD Windows 请运行 `ntr-windows-amd64.exe`；32 位 Windows 请运行 `ntr-windows-386.exe`；Windows on ARM 请运行 `ntr-windows-arm64.exe`。

## 使用步骤

请将对应的 `.exe`、`config.yaml` 和本说明放在同一个目录中。打开 PowerShell 或命令提示符，进入该目录后执行：

```powershell
.\ntr-windows-amd64.exe example.com
```

也可以直接把目标地址拖入命令行后运行。程序界面保持原始 NTR 的紧凑表格风格：顶部显示产品标题、版权、DEST 和 START，中间显示路由统计，底部显示运行状态和按键提示。程序会根据窗口宽度自动隐藏次要列，适合小窗口使用。如果 Windows Defender 防火墙或安全软件弹出网络访问提示，请允许该程序访问网络；ICMP 探测需要系统允许相关网络权限。

## 数据库

程序默认从可执行文件所在目录查找以下文件：

| 文件 | 用途 |
| --- | --- |
| `ipasn.bin` | ASN/BGP 信息 |
| `geoip.bin` | GeoIP/QQWry 信息 |

如果目录中没有数据库，可以先尝试执行：

```powershell
.\ntr-windows-amd64.exe -U
.\ntr-windows-amd64.exe -G
```

如果公司网络、代理或安全软件阻止在线更新，可以把数据库文件手动复制到 `.exe` 同目录，或者使用 `-a` 和 `-g` 暂时关闭 ASN、GeoIP 查询：

```powershell
.\ntr-windows-amd64.exe -a -g example.com
```

## 常用参数

```text
-L en                 使用英文界面
-4                    强制使用 IPv4
-6                    强制使用 IPv6
-i 1s                 设置每轮探测间隔
-t 2s                 设置单次响应超时
-m 30                 设置最大 TTL
-s 192.168.1.10       指定源地址
-U                    更新 ASN 数据库
-G                    更新 GeoIP 数据库
-v                    查看版本
```

## TUI 操作

| 操作 | 功能 |
| --- | --- |
| 鼠标点击、鼠标滚轮 | 选择或滚动路由行，选中行会高亮 |
| `Up` / `Down`、`PageUp` / `PageDown` | 键盘选择和滚动表格 |
| `r` | 立即刷新 |
| `c` | 开关丢包颜色 |
| `q` / `Esc` / `Ctrl+C` | 退出程序并停止后台探测 |

按键提示固定放在窗口最底部。成功、丢包和超时行会根据颜色开关显示不同颜色；按 `c` 可以切换。默认配置已经包含本次修复后的请求节奏和显示参数，默认每轮间隔为 `1s`。若需要调整网络行为，可以编辑同目录下的 `config.yaml`，修改后重新启动程序即可；命令行参数会覆盖配置文件。

## 版本选择

| 文件 | 适用系统 |
| --- | --- |
| `ntr-windows-amd64.exe` | 大多数 64 位 Intel/AMD Windows，推荐优先使用 |
| `ntr-windows-386.exe` | 32 位 Windows |
| `ntr-windows-arm64.exe` | ARM64 Windows |

这些文件由本次修复后的源码直接交叉编译生成，构建时已关闭 CGO 依赖，正常运行不需要额外安装运行库。
