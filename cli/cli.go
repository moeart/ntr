package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moeart/ntr/pkg/asn"
	"github.com/moeart/ntr/pkg/config"
	"github.com/moeart/ntr/pkg/geoip"
	"github.com/moeart/ntr/pkg/ntr"
	"github.com/moeart/ntr/pkg/render"
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

	srcAddr      = ""
	versionFlag  bool
	ENABLE_ASN   = true
	UPDATE_ASN   = false
	ENABLE_GEOIP = true
	UPDATE_GEOIP = false
	LANG         = "zh" // Default language is Chinese
	forceIPv4    = false
	forceIPv6    = false
)

// rootCmd represents the root command
var RootCmd = &cobra.Command{
	Use:          "ntr TARGET",
	SilenceUsage: true, // Don't automatically show help information when command execution fails
	Args: func(cmd *cobra.Command, args []string) error {
		// If using --version, --help, --update-asn, or --update-geoip, target parameter is not required
		if versionFlag || cmd.Flags().Changed("help") || cmd.Flags().Changed("update-asn") || cmd.Flags().Changed("update-geoip") {
			return nil
		}
		// If no parameters, show help information
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
		// Otherwise, require exactly 1 target parameter
		if len(args) != 1 {
			return fmt.Errorf("requires exactly 1 argument")
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if help was requested
		if cmd.Flags().Changed("help") {
			// Output title and copyright information
			fmt.Printf("%s\n", ntr.ToolName)
			fmt.Printf("%s\n", ntr.ToolCopyright)
			fmt.Println()
			// Call default help function
			cmd.Help()
			return nil
		}

		if versionFlag {
			fmt.Printf("%s\n", ntr.ToolName)
			fmt.Printf("%s\n", ntr.ToolCopyright)
			fmt.Printf("NTR Version: %s, build date: %s\n", version, date)
			return nil
		}

		// Load configuration
		cfg, err := config.LoadConfigFromDefaultPath()
		if err != nil {
			return fmt.Errorf("Failed to load configuration: %v", err)
		}

		// Handle --update-asn parameter
		if UPDATE_ASN {
			fmt.Println("Updating ASN database...")
			if err := asn.UpdateASNDatabase(cfg.GetASNDownloadURL()); err != nil {
				return fmt.Errorf("Failed to update ASN database: %v", err)
			}
			fmt.Println("ASN database updated successfully.")
			return nil
		}

		// Handle --update-geoip parameter
		if UPDATE_GEOIP {
			fmt.Println("Updating GeoIP database...")
			if err := geoip.UpdateGeoIPDatabase(cfg.GetGeoIPDownloadURL()); err != nil {
				return fmt.Errorf("Failed to update GeoIP database: %v", err)
			}
			fmt.Println("GeoIP database updated successfully.")
			return nil
		}

		// Detect timezone to determine whether to use QQWry
		useQQWry := true
		zone, _ := time.Now().Zone()
		// Chinese timezone usually contains "CST" (China Standard Time)
		if !strings.Contains(strings.ToUpper(zone), "CST") || LANG == "en" {
			useQQWry = false
		}

		// Set IP protocol preference according to configuration (if not specified by command line parameters)
		if !forceIPv4 && !forceIPv6 {
			switch cfg.Network.IPPreference {
			case "ipv4":
				forceIPv4 = true
			case "ipv6":
				forceIPv6 = true
				// "auto" or other values maintain default behavior
			}
		}

		// Verify protocol options
		if forceIPv4 && forceIPv6 {
			return fmt.Errorf("cannot use both -4 and -6 options at the same time")
		}

		m, ch, err := ntr.NewNTR(args[0], srcAddr, TIMEOUT, INTERVAL, HOP_SLEEP,
			MAX_HOPS, MAX_UNKNOWN_HOPS, RING_BUFFER_SIZE, PTR_LOOKUP, ENABLE_ASN, ENABLE_GEOIP, LANG, useQQWry, forceIPv4, forceIPv6)
		if err != nil {
			return err
		}

		mu := &sync.Mutex{}

		// Start window size change monitoring
		go render.WatchWindowSize()

		// Handle network updates and window size changes
		go func(ch chan struct{}) {
			for {
				select {
				case <-ch:
					mu.Lock()
					render.MoveCursor(1, 1)
					m.Render()
					render.Flush()
					mu.Unlock()
				case <-render.GetResizeChan():
					mu.Lock()
					render.MoveCursor(1, 1)
					m.Render()
					render.Flush()
					mu.Unlock()
				}
			}
		}(ch)

		m.Run(ch, COUNT)
		close(ch)
		mu.Lock()
		render.MoveCursor(1, 1)
		m.Render()
		render.Flush()
		mu.Unlock()
		return nil
	},
}

func init() {
	// Set custom help information format, including title and copyright information
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
	// Add short options -a and -g
	var disableASN bool
	RootCmd.Flags().BoolVarP(&disableASN, "disable-asn", "a", false, "Disable IP to BGP AS number query.")
	// Add validation function to modify ENABLE_ASN variable
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		if disableASN {
			ENABLE_ASN = false
		}
	}

	var disableGeoIP bool
	RootCmd.Flags().BoolVarP(&disableGeoIP, "disable-geoip", "g", false, "Disable IP to geographic location query.")
	RootCmd.PreRun = func(cmd *cobra.Command, args []string) {
		// Check if help was requested
		if cmd.Flags().Changed("help") {
			// Output title and copyright information
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
	// Add IPv4 and IPv6 options
	RootCmd.Flags().BoolVarP(&forceIPv4, "ipv4", "4", false, "Force using IPv4 protocol")
	RootCmd.Flags().BoolVarP(&forceIPv6, "ipv6", "6", false, "Force using IPv6 protocol")

	RootCmd.Flags().IntVarP(&MAX_HOPS, "max-hop", "m", MAX_HOPS, "Maximum number of hops to try. (min:1, max:255)")
	RootCmd.Flags().DurationVarP(&TIMEOUT, "timeout", "t", TIMEOUT, "Stop waiting for router response in seconds. (min:1)")

	RootCmd.Flags().BoolVarP(&UPDATE_ASN, "update-asn", "U", UPDATE_ASN, "Update ASN database from online source.")
	RootCmd.Flags().BoolVarP(&UPDATE_GEOIP, "update-geoip", "G", UPDATE_GEOIP, "Update GeoIP database from online source.")
	RootCmd.Flags().StringVarP(&LANG, "lang", "L", LANG, "Set language (zh for Chinese, en for English)")
	RootCmd.Flags().BoolVarP(&versionFlag, "version", "v", false, "Print version information")
}
