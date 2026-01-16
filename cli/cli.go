package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	tm "github.com/buger/goterm"
	pj "github.com/hokaccha/go-prettyjson"
	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/ntr"
	"github.com/spf13/cobra"
)

var (
	version string
	date    string

	COUNT            = 5
	TIMEOUT          = 1000 * time.Millisecond
	INTERVAL         = 200 * time.Millisecond
	HOP_SLEEP        = time.Nanosecond
	MAX_HOPS         = 25
	MAX_UNKNOWN_HOPS = 10
	RING_BUFFER_SIZE = 128
	PTR_LOOKUP       = false
	jsonFmt          = false
	srcAddr          = ""
	versionFlag      bool
	ENABLE_ASN       = true
	UPDATE_ASN       = false
	ENABLE_GEOIP     = true
	UPDATE_GEOIP     = false
	LANG             = "zh" // 默认语言为中文
	forceIPv4        = false
	forceIPv6        = false
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use:          "ntr TARGET",
	SilenceUsage: true, // 命令执行失败时不自动显示帮助信息
	Args: func(cmd *cobra.Command, args []string) error {
		// 如果使用 --version、--help、--update-asn 或 --update-geoip，则不要求必须有目标参数
		if versionFlag || cmd.Flags().Changed("help") || cmd.Flags().Changed("update-asn") || cmd.Flags().Changed("update-geoip") {
			return nil
		}
		// 如果没有参数，则显示帮助信息
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
		// 否则要求必须有且仅有一个目标参数
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// 检查是否请求了帮助
		if cmd.Flags().Changed("help") {
			// 输出标题和版权信息
			fmt.Printf("%s\n", ntr.ToolName)
			fmt.Printf("%s\n", ntr.ToolCopyright)
			fmt.Println()
			// 调用默认的帮助函数
			cmd.Help()
			return nil
		}

		if versionFlag {
			fmt.Printf("%s\n", ntr.ToolName)
			fmt.Printf("%s\n", ntr.ToolCopyright)
			fmt.Printf("NTR Version: %s, build date: %s\n", version, date)
			return nil
		}
		// 处理 --update-asn 参数
		if UPDATE_ASN {
			fmt.Println("Updating ASN database...")
			if err := asn.UpdateASNDatabase(); err != nil {
				return fmt.Errorf("Failed to update ASN database: %v", err)
			}
			fmt.Println("ASN database updated successfully.")
			return nil
		}

		// 处理 --update-geoip 参数
		if UPDATE_GEOIP {
			fmt.Println("Updating GeoIP database...")
			if err := geoip.UpdateGeoIPDatabase(); err != nil {
				return fmt.Errorf("Failed to update GeoIP database: %v", err)
			}
			fmt.Println("GeoIP database updated successfully.")
			return nil
		}

		// 检测时区，判断是否使用QQWry
		useQQWry := true
		zone, _ := time.Now().Zone()
		// 中国时区通常包含 "CST"（China Standard Time）
		if !strings.Contains(strings.ToUpper(zone), "CST") || LANG == "en" {
			useQQWry = false
		}

		// 验证协议选项
		if forceIPv4 && forceIPv6 {
			return fmt.Errorf("cannot use both -4 and -6 options at the same time")
		}

		m, ch, err := ntr.NewNTR(args[0], srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
			MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP, ENABLE_ASN, ENABLE_GEOIP, LANG, useQQWry, forceIPv4, forceIPv6)
		if err != nil {
			return err
		}
		if jsonFmt {
			go func(ch chan struct{}) {
				for {
					<-ch
				}
			}(ch)
			m.Run(ch, COUNT)
			s, err := pj.Marshal(m)
			if err != nil {
				return err
			}
			fmt.Println(string(s))
			return nil
		}
		fmt.Println("Start:", time.Now())
		tm.Clear()
		mu := &sync.Mutex{}

		// 启动窗口大小变化监听
		go watchWindowSize()

		// 处理网络更新和窗口大小变化
		go func(ch chan struct{}) {
			for {
				select {
				case <-ch:
					mu.Lock()
					render(m)
					mu.Unlock()
				case <-resizeChan:
					mu.Lock()
					render(m)
					mu.Unlock()
				}
			}
		}(ch)

		m.Run(ch, COUNT)
		close(ch)
		mu.Lock()
		render(m)
		mu.Unlock()
		return nil
	},
}

// 创建窗口大小变化检测通道
var resizeChan = make(chan bool, 1)

func render(m *ntr.NTR) {
	tm.Clear()
	tm.MoveCursor(1, 1)
	m.Render(1)
	tm.Flush() // Call it every time at the end of rendering
}

// 监听窗口大小变化的函数
func watchWindowSize() {
	// 使用轮询方式检测窗口大小变化
	prevWidth, _ := ntr.GetTerminalSize()
	for {
		time.Sleep(200 * time.Millisecond) // 每 200ms 检查一次
		currWidth, _ := ntr.GetTerminalSize()
		if currWidth != prevWidth {
			resizeChan <- true
			prevWidth = currWidth
		}
	}
}

func init() {
	// 设置自定义的帮助信息格式，包含标题和版权信息
	RootCmd.SetUsageTemplate(`{{printf "%s" "NTR - MoeArt's Network Traceroute"}}
{{printf "%s" "(c)2016-2026 MoeArt OpenSource, www.acgdraw.com"}}

Usage:
  {{.UseLine}}

{{if .HasAvailableSubCommands}}
Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}

{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimRightSpace}}

{{end}}{{if .HasHelpSubCommands}}
Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}

{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.
{{end}}`)

	RootCmd.Flags().StringVarP(&srcAddr, "address", "s", srcAddr, "The address to bind the outgoing socket to")
	// 添加短选项 -a 和 -g
	var disableASN bool
	RootCmd.Flags().BoolVarP(&disableASN, "disable-asn", "a", false, "Disable IP to BGP AS number query.")
	// 添加验证函数来修改 ENABLE_ASN 变量
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if disableASN {
			ENABLE_ASN = false
		}
	}

	var disableGeoIP bool
	RootCmd.Flags().BoolVarP(&disableGeoIP, "disable-geoip", "g", false, "Disable IP to geographic location query.")
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		// 检查是否请求了帮助
		if cmd.Flags().Changed("help") {
			// 输出标题和版权信息
			fmt.Printf("%s\n", ntr.ToolName)
			fmt.Printf("%s\n", ntr.ToolCopyright)
			fmt.Println()
		}
		if disableASN {
			ENABLE_ASN = false
		}
		if disableGeoIP {
			ENABLE_GEOIP = false
		}
	}

	RootCmd.Flags().DurationVarP(&INTERVAL, "interval", "i", INTERVAL, "Seconds between each traceroute. (min:1)")
	// 添加 IPv4 和 IPv6 选项
	RootCmd.Flags().BoolVarP(&forceIPv4, "ipv4", "4", false, "Force using IPv4 protocol")
	RootCmd.Flags().BoolVarP(&forceIPv6, "ipv6", "6", false, "Force using IPv6 protocol")
	RootCmd.Flags().BoolVarP(&jsonFmt, "json", "j", jsonFmt, "Print JSON formatted results")
	RootCmd.Flags().IntVarP(&MAX_HOPS, "max-hop", "m", MAX_HOPS, "Maximum number of hops to try. (min:1, max:255)")
	RootCmd.Flags().DurationVarP(&TIMEOUT, "timeout", "t", TIMEOUT, "Stop waiting for router response in seconds. (min:1)")

	// 添加域名验证选项
	var unverifyTLD bool
	RootCmd.Flags().BoolVarP(&unverifyTLD, "unverify-tld", "D", false, "Disable Domain Available Verification.")

	RootCmd.Flags().BoolVarP(&UPDATE_ASN, "update-asn", "U", UPDATE_ASN, "Update ASN database from online source.")
	RootCmd.Flags().BoolVarP(&UPDATE_GEOIP, "update-geoip", "G", UPDATE_GEOIP, "Update GeoIP database from online source.")
	RootCmd.Flags().StringVarP(&LANG, "lang", "L", LANG, "Set language (zh for Chinese, en for English)")
	RootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print version information")
}
