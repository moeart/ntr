# What Changes
 
- **Complete rewrite from C# to Golang** for improved cross-platform compatibility
- Added offline ASN database due to online service has been down
- Added GeoIP query functionality with multi-language support (Chinese and English)
- Implemented ASN query and integrated it into traceroute output
- Optimized ASN database loading performance using memory mapping and binary format
- Added cross-platform terminal size detection and adaptive rendering
- Added support for forced IPv4/IPv6 protocol options
- Improved terminal table display alignment and internationalization support
- Added configuration file support
- Optimized code structure and fixed potential memory leaks
- Updated dependency versions to the latest
 
# Known issues
 
- CZ88 IP database has been stopped update service and ended at 2024-9-25
- But has 3rd-party repository for IP database update
 
**Full Changelog**: `https://github.com/moeart/ntr/compare/3.0...golang`