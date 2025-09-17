# Windows Support for Azure Storage FUSE

This document outlines the Windows support implementation for Azure Storage FUSE (blobfuse2).

## Overview

Windows support has been added to blobfuse2 through:

1. **Cross-platform abstraction layers** for platform-specific functionality
2. **Windows Event Log support** replacing syslog on Windows
3. **WinFsp integration** for filesystem operations on Windows
4. **Cross-platform filesystem statistics** abstracting Unix syscalls
5. **Dynamic library loading** abstraction for extension support

## Architecture

### Cross-Platform Components

#### Logging (`common/log/`)
- **Unix/Linux**: Uses syslog via `sys_logger.go` 
- **Windows**: Uses Windows Event Log via `winlog_logger.go`
- **Fallback**: File-based logging available on both platforms

#### Filesystem Statistics (`common/fsstat*.go`)
- **Unix/Linux**: Uses `syscall.Statfs_t` via `fsstat_unix.go`
- **Windows**: Uses `GetDiskFreeSpaceEx` API via `fsstat_windows.go`
- **Common Interface**: `FilesystemStat` struct in `fsstat.go`

#### Dynamic Library Loading (`common/dynlib*.go`)
- **Unix/Linux**: Uses `dlopen`/`dlclose` via `dynlib_unix.go`
- **Windows**: Uses `LoadLibrary`/`FreeLibrary` via `dynlib_windows.go`
- **Common Interface**: `DynamicLibrary` struct in `dynlib.go`

#### Utility Functions (`common/util*.go`)
- **Unix/Linux**: Signal-based parent notification via `util_unix.go`
- **Windows**: Alternative IPC mechanisms via `util_windows.go`
- **Common**: Shared functionality in `util.go`

### FUSE Layer

#### Unix/Linux FUSE Support
- Uses libfuse2/libfuse3 via `libfuse_handler.go` and `libfuse2_handler.go`
- Build tags: `!windows` to exclude from Windows builds

#### Windows FUSE Support (WinFsp)
- Uses WinFsp (Windows File System Proxy) via `libfuse_winfsp_handler.go`
- Build tags: `windows` to include only in Windows builds
- **Note**: Current implementation provides the interface structure; full WinFsp integration requires additional development

## Build Tags

The implementation uses Go build tags for platform-specific compilation:

- `//go:build !windows` - Unix/Linux only
- `//go:build windows` - Windows only
- `//go:build fuse2 && !windows` - FUSE2 on Unix/Linux only
- `//go:build !fuse2 && !windows` - FUSE3 on Unix/Linux only

## Windows Requirements

### Prerequisites
1. **WinFsp Installation**: Download and install from [https://github.com/billziss-gh/winfsp](https://github.com/billziss-gh/winfsp)
2. **Go 1.24+**: For cross-compilation support
3. **Windows 10+**: Recommended for WinFsp compatibility

### Mount Points
- **Linux**: Directory paths (e.g., `/mnt/blobfuse`)
- **Windows**: Drive letters (e.g., `X:`, `Y:\`)

## Building for Windows

### Cross-compilation from Linux/Unix
```bash
GOOS=windows GOARCH=amd64 go build -o blobfuse2.exe
```

### Native Windows build
```cmd
go build -o blobfuse2.exe
```

## Current Status

### ✅ Implemented
- Cross-platform logging abstraction
- Cross-platform filesystem statistics
- Cross-platform dynamic library loading
- Windows Event Log integration
- Windows compilation support
- Build tag structure for platform separation

### 🚧 In Progress / TODO
- Complete WinFsp integration
- Windows-specific mount/unmount logic
- Windows service integration
- Windows-specific configuration validation
- Comprehensive Windows testing

### ⚠️ Limitations
- WinFsp integration is currently a stub implementation
- Some Unix-specific features (umask, allow-other) are not applicable on Windows
- Windows testing is limited without full WinFsp implementation

## Testing

### Cross-platform compilation test
```bash
# Test Linux build
go build -o blobfuse2

# Test Windows build (cross-compilation)
GOOS=windows go build -o blobfuse2.exe
```

### Platform-specific functionality
```bash
# Test cross-platform abstractions
go test ./common/...
```

## Usage on Windows

### Installation
1. Install WinFsp from the official repository
2. Download or build blobfuse2.exe for Windows
3. Configure Azure storage credentials

### Mount
```cmd
blobfuse2.exe mount X: --config-file=config.yaml
```

### Unmount
```cmd
blobfuse2.exe unmount X:
```

## Contributing

When adding new platform-specific functionality:

1. Create separate files with appropriate build tags
2. Use the common interface pattern (see `fsstat.go` example)
3. Ensure both platforms compile successfully
4. Add appropriate tests for cross-platform functionality
5. Update this documentation

## Future Work

1. **Complete WinFsp Integration**: Implement full FUSE operation callbacks
2. **Windows Service Support**: Add Windows service installation and management
3. **Windows Installer**: Create MSI package for easy installation
4. **Performance Optimization**: Windows-specific performance tuning
5. **Error Handling**: Windows-specific error codes and messages
6. **Documentation**: Comprehensive Windows-specific documentation