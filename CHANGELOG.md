## NTR Version: 5.3.2026.0212, build date: 2026-01-30
### What Changes
 
- fix(cli): Fix terminal state restore issue after interrupt
- fix(render): Fix title bar duplicate printing issue
- feat(terminal): Add terminal state save/restore and interrupt handling functionality
- refactor(render): Remove unused offset parameter and unify default value display
- fix(render): Fix loop condition and output issue in hop count rendering
- refactor(ntr): Remove platform-specific terminal size implementation
- style(config): Format configuration file loading path code
- fix(render): Use sync.Once to ensure screen clears only once to avoid flashing
- refactor(render): Refactor rendering module to optimize terminal display
 
### Known issues
 
- CZ88 IP database has been stopped update service and ended at 2024-9-25
- But has 3rd-party repository for IP database update
 
## NTR Version: 5.0.2026.0116, build date: 2026-01-16
### What Changes
 
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
 
### Known issues
 
- CZ88 IP database has been stopped update service and ended at 2024-9-25
- But has 3rd-party repository for IP database update
 
**Full Changelog**: `https://github.com/moeart/ntr/compare/3.0...golang`