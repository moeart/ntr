# TUI 库选型资料

## tview

来源：https://github.com/rivo/tview

tview 是 MIT 许可的 Go 终端 UI 库，官方仓库列出 TextView、Table、Flex/Grid 布局、Frame、Application 等组件，并支持边框、标题、表格、输入捕获和全屏应用。项目使用版本 `v0.42.0`，模块页显示该版本发布于 2025-08-27。官方仓库说明其基于 `github.com/gdamore/tcell` 和 `github.com/rivo/uniseg`。

来源：https://pkg.go.dev/github.com/rivo/tview

已核对的 API 包括 `NewApplication`、`SetRoot`、`Run`、`Stop`、`QueueUpdateDraw`、`SetInputCapture`；`Table` 的 `SetBorders`、`SetBordersColor`、`SetSeparator`、`SetFixed`、`SetSelectable`、`SetCell`、`Clear`；以及 `Box` 的 `SetBorder`、`SetBorderColor`、`SetTitle`。这些 API 与 NTR 的实时路由表、边框布局和按键处理需求直接匹配。

## Bubble Tea

来源：https://github.com/charmbracelet/bubbletea

Bubble Tea 采用 Elm Architecture，核心是 Model、Update、View，官方说明其提供高性能 cell renderer、键盘和鼠标处理，并可与 Bubbles、Lip Gloss 等组件配合。其抽象更适合状态驱动、动画和复杂交互界面，但对当前 NTR 的现成表格、边框和简单实时刷新而言，tview 的直接组件模型改动更小。

来源：https://pkg.go.dev/github.com/charmbracelet/bubbletea

官方文档核对了键盘消息、程序生命周期、窗口大小和退出命令等能力。当前实现最终选择 tview，原因是它直接提供表格和 Box/Flex 组件，同时依赖更少、迁移成本更低。
