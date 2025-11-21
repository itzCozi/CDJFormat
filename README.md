# CDJFormat

CDJFormat is a command line tool designed to help DJs prepare USB drives for use on standalone systems with rekordbox.

## Features

- 🔍 **List available drives** - See all drives connected to your system
- 💾 **Format drives to FAT32** - Automatically format USB drives with optimal settings for rekordbox
- ✅ **Safety checks** - Confirmation prompts to prevent accidental data loss
- 🎛️ **Cross-platform** - Works on Linux, macOS, and Windows
- 🏷️ **Custom labels** - Set custom volume labels (defaults to "REKORDBOX")

## Installation

### Download Binary

Download the latest release for your platform from the [releases page](https://github.com/itzCozi/CDJFormat/releases).

### Build from Source

Requirements:
- Go 1.20 or later

```bash
git clone https://github.com/itzCozi/CDJFormat.git
cd CDJFormat
go build -o cdjformat
```

On Linux/macOS, you may want to move the binary to your PATH:
```bash
sudo mv cdjformat /usr/local/bin/
```

On Windows, add the directory containing `cdjformat.exe` to your PATH.

## Usage

### List Available Drives

To see all available drives on your system:

```bash
cdjformat list
```

### Format a Drive

⚠️ **WARNING**: Formatting will erase all data on the selected drive!

#### Interactive Mode

Simply run the format command without arguments to be prompted for the device:

```bash
cdjformat format
```

#### Direct Mode

Specify the device directly:

**Linux:**
```bash
sudo cdjformat format /dev/sdb
```

**macOS:**
```bash
sudo cdjformat format disk2
```

**Windows (run as Administrator):**
```powershell
cdjformat format E:
```

### Options

- `-y, --yes` - Skip confirmation prompt
- `-l, --label` - Set a custom volume label (default: "REKORDBOX")

#### Examples

Format with automatic confirmation:
```bash
sudo cdjformat format /dev/sdb -y
```

Format with a custom label:
```bash
sudo cdjformat format /dev/sdb -l "MY_USB"
```

## Why FAT32 for rekordbox?

rekordbox and Pioneer DJ equipment (CDJ/XDJ players) require USB drives to be formatted as FAT32 for compatibility. This tool ensures your drive is properly formatted with the correct settings for optimal performance.

### rekordbox Compatibility

- ✅ FAT32 file system
- ✅ MBR partition table
- ✅ Proper volume label
- ✅ Compatible with CDJ-2000NXS2, CDJ-3000, XDJ-XZ, and other Pioneer DJ equipment

## Requirements

### Permissions

**Linux/macOS:** You need root/administrator privileges to format drives. Use `sudo`:
```bash
sudo cdjformat format /dev/sdb
```

**Windows:** Run your terminal/PowerShell as Administrator.

### Dependencies

**Linux:**
- `mkfs.vfat` (usually in the `dosfstools` package)
- Install with: `sudo apt-get install dosfstools` (Debian/Ubuntu)

**macOS:**
- `diskutil` (built-in)

**Windows:**
- `format` command (built-in)

## Safety Features

- Device validation before formatting
- Clear warning messages
- Confirmation prompts (unless `-y` flag is used)
- Displays device information before formatting

## Troubleshooting

### "Permission denied" error

Make sure you're running the command with administrator/root privileges:
- Linux/macOS: Use `sudo`
- Windows: Run terminal as Administrator

### Drive not showing up

1. Make sure the USB drive is properly connected
2. Run `cdjformat list` to see available drives
3. On Linux, you may need to check `dmesg` or `lsblk` for device names

### Format fails on Linux

Ensure `dosfstools` is installed:
```bash
sudo apt-get install dosfstools
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See LICENSE file for details.

## Disclaimer

This tool will **permanently erase all data** on the selected drive. Always double-check the device name before formatting. The authors are not responsible for any data loss.
