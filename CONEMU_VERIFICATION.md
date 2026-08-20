# ConEmu v23.07.24 验证说明

已使用官方 ConEmu v23.07.24 便携版，在 Wine 9.0、Xvfb、Openbox 和 x11vnc 图形环境中运行 Windows amd64 版本 NTR。

验证启动命令为：

```text
ConEmu64.exe -Single -run "Z:\home\ubuntu\conemu-vnc\ntr-windows-amd64-test.exe" -a -g -L en -m 25 -i 1s -t 1s 127.0.0.1
```

在 80x25 ConEmu 窗口中，TUI 保持原始 NTR 表格的完整十列顺序：`# / HOST / LOSS% / SENT / LAST / BEST / AVG / WORST / ASN / IP LOCATION`。普通标题、DEST/START、正文、状态和快捷键区域沿用终端自身的默认配色，不修改用户的 ConEmu 主题；表头仅局部固定为黑字白底，等价复现原始 render.go，数字列右对齐，目标、ASN 和位置列左对齐。鼠标输入已关闭，不会因指针移动或点击产生额外显示；Hop 不再有选中行、粗体或 Selected 状态。键盘 `↑` / `↓` 被映射为整页 PageUp / PageDown 翻页；Hop 超出可视区域时最右侧显示 ASCII `|` 轨道和 `#` 滑块。底部提示为 `[Q]uit [R]efresh [C]olor [UP/DN]Page`，中文界面对应 `[Q]退出 [R]刷新 [C]颜色 [UP/DN]翻页`；footerHelp 按文本实际宽度固定在右侧，状态栏使用剩余宽度显示最近活动时间。

已发送 `c`、`r`、`Up`、`Down` 和 `Page_Down` 快捷键，界面保持响应；80x25 截图文件为 `vnc-conemu-80x25-tview-original-header.png`，连续 resize 截图文件为 `vnc-conemu-resize-tview-original-header.png`。

注意：127.0.0.1 在 Wine 环境中可能因 ICMP 权限/兼容层限制显示超时行；这不影响本次对 TUI 布局、颜色和输入事件的验证。此前 ConEmu 启动失败的原因是 Bash 未引用 Windows 路径，反斜杠被吞掉；最终版本已修复验证脚本的启动方式。
