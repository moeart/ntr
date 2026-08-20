# NTR (MoeArt's Network TraceRoute)

[中文版本 (Chinese Version)](READMECN.md)

## Project Background
NTR is a network diagnostic tool initially developed in C#. To achieve cross-platform compatibility, it was rewritten in Go language. This version is based on the open-source [mtr](https://github.com/tonobo/mtr) project and fully replicates the functionality of the original C# version while adding new features. The original C# version, located in the `dotnet` branch, is no longer being updated.

NTR is a powerful network diagnostic tool designed to help network engineers troubleshoot connectivity issues. It can identify all routers between the source and destination hosts using the ICMP protocol, resolve each router's BGP AS number through the IPtoASN public service, and determine each IP's geographic location using the QQWry database.

# Features

## Core Functionality
* **Cross-platform support**: Runs seamlessly on Windows, Linux, and macOS operating systems
* **Go-based implementation**: Built with Go 1.25+ for improved performance and reliability
* **ICMP traceroute**: Uses the ICMP protocol to discover routers along the network path
* **BGP ASN resolution**: Converts IP addresses to BGP AS numbers to identify network providers
* **GeoIP lookup**: Determines the geographic location of IP addresses in both Chinese and English

## User Experience
* **Beautiful console output**: Features a well-organized, colorized interface for easy reading
* **Multi-language support**: Available in both Chinese and English languages
* **IPv4/IPv6 support**: Works with both IPv4 and IPv6 addresses, with options to force a specific protocol

## Customization
* **Configurable settings**: Adjustable timeout and interval parameters to suit different network conditions
* **Custom socket binding**: Ability to bind the outgoing socket to a specific network interface

## Database Management
* **Auto-update functionality**: Automatic updates for ASN and GeoIP databases to ensure accurate information
* **Offline support**: Can work with local GeoIP database (QQWry) for offline Chinese IP location lookup

# Download
You can download from the [Releases](https://github.com/moeart/ntr/releases/latest) page.

# Screenshots
## Chinese Interface
![Chinese Interface](PreviewCN.png)

## English Interface
![English Interface](PreviewEN.png)

# Quick Start
### Simplest Way
```bash
ntr www.acgdraw.com
```

### Disable BGP ASN Query
```bash
ntr www.acgdraw.com -a
```

### Disable GeoIP Query
```bash
ntr www.acgdraw.com -g
```

### Use English Interface
```bash
ntr www.acgdraw.com -L en
```

### Force IPv4 Protocol
```bash
ntr www.acgdraw.com -4
```

### Update ASN Database
```bash
ntr -U
```

### Update GeoIP Database
```bash
ntr -G
```

### Other Options
```
NTR - MoeArt's Network Traceroute
(c)2016-2026 MoeArt OpenSource, www.acgdraw.com

Usage:
  ntr TARGET [flags]

Flags:
  -s, --address string      The address to bind the outgoing socket to
  -a, --disable-asn         Disable IP to BGP AS number query.
  -g, --disable-geoip       Disable IP to geographic location query.
  -h, --help                help for ntr
  -i, --interval duration   Seconds between each traceroute (default 1s).
  -4, --ipv4                Force using IPv4 protocol
  -6, --ipv6                Force using IPv6 protocol
  -L, --lang string         Set language (zh for Chinese, en for English) (default "zh")
  -m, --max-hop int         Maximum number of hops to try. (min:1, max:255) (default 25)
  -t, --timeout duration    Stop waiting for router response in seconds. (min:1) (default 1s)
  -U, --update-asn          Update ASN database from online source.
  -G, --update-geoip        Update GeoIP database from online source.
  -v, --version             Print version information
```

# Installation

## Prerequisites
- Go 1.25+ (for building from source)
- Git (for cloning the repository)
- Administrative/root privileges (required for raw socket access)

## From Source

### Linux/macOS
```bash
# Clone the repository
git clone https://github.com/moeart/ntr.git
cd ntr

# Build the application
go build -o ntr main.go

# Make the binary executable
chmod +x ntr

# Move to a directory in your PATH (optional)
sudo mv ntr /usr/local/bin/
```

### Windows
```powershell
# Clone the repository
git clone https://github.com/moeart/ntr.git
cd ntr

# Build using the provided script
./build.ps1

# Or build manually
go build -o ntr.exe main.go
```

## From Pre-built Binaries

You can download pre-built binaries for your platform from the [Releases](https://github.com/moeart/ntr/releases/latest) page.

### Linux
```bash
# Download the binary
wget https://github.com/moeart/ntr/releases/latest/download/ntr-linux-amd64

# Make it executable
chmod +x ntr-linux-amd64

# Move to a directory in your PATH
mv ntr-linux-amd64 /usr/local/bin/ntr
```

### macOS
```bash
# Download the binary
wget https://github.com/moeart/ntr/releases/latest/download/ntr-darwin-amd64

# Make it executable
chmod +x ntr-darwin-amd64

# Move to a directory in your PATH
mv ntr-darwin-amd64 /usr/local/bin/ntr
```

### Windows
1. Download the binary from the [Releases](https://github.com/moeart/ntr/releases/latest) page
2. Extract the ZIP file
3. Run the `ntr.exe` executable
4. (Optional) Add the directory to your PATH environment variable

# License
This project is released under [MIT License](https://github.com/moeart/ntr/blob/master/LICENSE).

This project is based on [https://github.com/tonobo/mtr](https://github.com/tonobo/mtr), which is also licensed under the MIT License.

## Dependencies and Their Licenses

### Direct Dependencies
- [github.com/buger/goterm](https://github.com/buger/goterm) - MIT License
- [github.com/hokaccha/go-prettyjson](https://github.com/hokaccha/go-prettyjson) - MIT License
- [github.com/spf13/cobra](https://github.com/spf13/cobra) - Apache License 2.0
- [github.com/xiaoqidun/qqwry](https://github.com/xiaoqidun/qqwry) - MIT License
- [golang.org/x/net](https://github.com/golang/net) - BSD 3-Clause License

### Indirect Dependencies
- [github.com/fatih/color](https://github.com/fatih/color) - MIT License
- [github.com/inconshreveable/mousetrap](https://github.com/inconshreveable/mousetrap) - Apache License 2.0
- [github.com/ipipdotnet/ipdb-go](https://github.com/ipipdotnet/ipdb-go) - MIT License
- [github.com/mattn/go-colorable](https://github.com/mattn/go-colorable) - MIT License
- [github.com/mattn/go-isatty](https://github.com/mattn/go-isatty) - MIT License
- [github.com/spf13/pflag](https://github.com/spf13/pflag) - BSD 3-Clause License
- [golang.org/x/sys](https://github.com/golang/sys) - BSD 3-Clause License
- [golang.org/x/text](https://github.com/golang/text) - BSD 3-Clause License
- [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) - MIT License

## Additional Licenses
- IP Geo-Location database uses [QQWry](https://github.com/xiaoqidun/qqwry) database for offline Chinese IP location lookup.
- IP to ASN uses the public service provided by [IPtoASN.com](https://iptoasn.com/), which is serviced under [BSD 2-Clause License](https://github.com/jedisct1/iptoasn-webservice/blob/master/LICENSE). It provides public convert service and offline databases.

### MoeArt Development Team
Github: https://github.com/moeart    
Developer Home: http://lab.acgdraw.com    
Official Website: http://www.acgdraw.com    
