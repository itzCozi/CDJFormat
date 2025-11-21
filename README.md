# CDJFormat

CDJFormat is a command line tool designed to help DJs prepare USB drives for use on standalone systems with rekordbox.

## TODO
Make the performance benchmark accurate dude we cant be giving people false numbers
split main.go into multiple files for better organization
add a progress bar for formatting and benchmarking operations

## Features

- 🔍 **List available drives** - See all drives connected to your system
- 💾 **Format drives to FAT32** - Automatically format USB drives with optimal settings for rekordbox
- ✅ **Safety checks** - Confirmation prompts to prevent accidental data loss
- 🎛️ **Platforms** - Supports macOS and Windows
- 🏷️ **Custom labels** - Set custom volume labels (defaults to "REKORDBOX")

## Documentation

For detailed documentation, visit [cdjf.ar0.eu](https://cdjf.ar0.eu)

## Installation

Requirements:
- Go 1.20 or later

```bash
git clone https://github.com/itzCozi/CDJFormat.git
cd CDJFormat
# MacOS
go build -o cdjf

# Windows
go build -o cdjf.exe
```

Alternatively, use the build helper:
```bash
go run ./tools/build
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

See LICENSE file for details.

## Disclaimer

This tool will **permanently erase all data** on the selected drive. Always double-check the device name before formatting. We are not responsible for any data loss.
