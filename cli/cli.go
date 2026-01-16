package cli

import (
	"fmt"
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
	ENABLE_ASN       = false
	UPDATE_ASN       = false
	ENABLE_GEOIP     = false
	UPDATE_GEOIP     = false
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use: "ntr TARGET",
	Args: func(cmd *cobra.Command, args []string) error {
		// 如果使用 --version、--help、--update-asn 或 --update-geoip，则不要求必须有目标参数
		if versionFlag || cmd.Flags().Changed("help") || cmd.Flags().Changed("update-asn") || cmd.Flags().Changed("update-geoip") {
			return nil
		}
		// 否则要求必须有且仅有一个目标参数
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if versionFlag {
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

		m, ch, err := ntr.NewNTR(args[0], srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
			MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP, ENABLE_ASN, ENABLE_GEOIP)
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
	RootCmd.Flags().IntVarP(&COUNT, "count", "c", COUNT, "Amount of pings per target")
	RootCmd.Flags().DurationVarP(&TIMEOUT, "timeout", "t", TIMEOUT, "ICMP reply timeout")
	RootCmd.Flags().DurationVarP(&INTERVAL, "interval", "i", INTERVAL, "Wait time between icmp packets before sending new one")
	RootCmd.Flags().DurationVar(&HOP_SLEEP, "hop-sleep", HOP_SLEEP, "Wait time between pinging next hop")
	RootCmd.Flags().IntVar(&MAX_HOPS, "max-hops", MAX_HOPS, "Maximal TTL count")
	RootCmd.Flags().IntVar(&MAX_UNKNOWN_HOPS, "max-unknown-hops", MAX_UNKNOWN_HOPS, "Maximal hops that do not reply before stopping to look")
	RootCmd.Flags().IntVar(&RING_BUFFER_SIZE, "buffer-size", RING_BUFFER_SIZE, "Cached packet buffer size")
	RootCmd.Flags().BoolVar(&jsonFmt, "json", jsonFmt, "Print json results")
	RootCmd.Flags().BoolVarP(&PTR_LOOKUP, "ptr", "n", PTR_LOOKUP, "Reverse lookup on host")
	RootCmd.Flags().BoolVar(&versionFlag, "version", false, "Print version")
	RootCmd.Flags().StringVar(&srcAddr, "address", srcAddr, "The address to be bound the outgoing socket")
	RootCmd.Flags().BoolVar(&ENABLE_ASN, "enable-asn", ENABLE_ASN, "Enable ASN lookup")
	RootCmd.Flags().BoolVar(&UPDATE_ASN, "update-asn", UPDATE_ASN, "Update ASN database")
	RootCmd.Flags().BoolVar(&ENABLE_GEOIP, "enable-geoip", ENABLE_GEOIP, "Enable GeoIP (location) lookup")
	RootCmd.Flags().BoolVar(&UPDATE_GEOIP, "update-geoip", UPDATE_GEOIP, "Update GeoIP (location) database")
}
