# CDJFormat

CDJFormat is a cross-platform CLI that prepares USB drives for use with rekordbox-enabled players. It wraps the native disk utilities on macOS and Windows with safety rails, speed checks, and workflow extras tailored for DJs.

## At A Glance

- 🔍 **Drive discovery** – Quickly list removable devices ready for formatting.
- 💾 **rekordbox-friendly format** – Create FAT32 volumes with optimal defaults and optional custom labels or cluster sizes.
- ⚙️ **Automation helpers** – Reuse saved profiles, batch-format multiple drives, and auto-eject once finished.
- ✅ **Safety + health checks** – Guardrails against system disks, interactive confirmations, benchmarking, and integrity verification.
- 📝 **Persistent logs** – Save verification reports alongside drive benchmarking output.

## Installation

Requirements:
- Go 1.20 or later

```bash
git clone https://github.com/itzCozi/CDJFormat.git
cd CDJFormat
go build -o cdjf    # macOS / Linux
go build -o cdjf.exe # Windows
```

Alternatively, use the build helper:

```bash
go run ./tools/build
```

Copy the resulting binary somewhere on your `PATH` (for example, `/usr/local/bin` on macOS or `C:\Users\<you>\bin` on Windows) so you can launch it from any terminal.

## Quick Start

1. Plug in the USB drive you want to prepare.
2. List candidate devices:
	```bash
	cdjf list
	```
3. Format the drive (replace `disk2` or `E:` with the identifier from the list output):
	```bash
	# macOS
	cdjf format disk2

	# Windows
	cdjf format E:
	```
4. Follow the prompts to review the adaptive benchmark (it may extend the sample briefly on fast systems), confirm the erase, and optionally eject afterwards.
5. (Recommended) Run an integrity check before loading music:
	```bash
	cdjf verify E:
	```

## Command Reference

### `cdjf list`

Shows removable drives detected on the current system and flags any that appear to be system/internal disks. On macOS it prints a detailed `diskutil` summary; on Windows it displays size, free space, filesystem, and the volume label.

### `cdjf format [device ...]`

Formats one or more drives to FAT32 using rekordbox-friendly defaults. When multiple devices are provided, formatting runs concurrently and labels are auto-suffixed (`REKORDBOX`, `REKORDBOX2`, ...). Before erasing, CDJFormat:

- Validates that each device looks removable and not a system disk.
- Runs an adaptive read/write benchmark (single-drive mode) that can grow the sample up to 256 MB for better accuracy, then warns on slow media. Custom speed thresholds are supported via profiles.
- Prompts for confirmation unless `--yes` is supplied.

Flags:

- `--yes`, `-y` – Skip the confirmation prompt (benchmark still runs unless using multiple devices).
- `--label`, `-l` – Set a custom volume label. CDJFormat avoids duplicates by suffixing the name when needed.
- `--cluster-size` – Windows only; normalize values such as `32K` or `32768`.
- `--profile` – Apply saved defaults, including labels, thresholds, and cluster size.

### `cdjf eject [device]`

Safely ejects the drive after validation. Calls `diskutil eject` on macOS or the Shell COM automation verb on Windows.

### `cdjf info [device]`

Displays drive metadata (size, free space, filesystem, internal/removable status) and automatically runs the benchmark to surface expected performance.

### `cdjf verify [device ...]`

Writes and rereads a test pattern (default 64 MB) to confirm the drive’s health. The command reports read/write speeds, surfaces any corruption, and writes a timestamped log (for example, `cdjf-verify-E-20240214-210455.log`). Use `--size` to change the payload size in megabytes.

### `cdjf profile`

Create reusable presets for formatting sessions. Profiles are stored in `~/.config/cdjf/profiles.json` on macOS/Linux or `%AppData%\cdjf\profiles.json` on Windows.

- `cdjf profile save my-usb --label BOOTH --cluster-size 32K --prompt 4.5`
- `cdjf profile list`
- `cdjf profile show my-usb`
- `cdjf profile delete old-profile`

When a profile is applied via `cdjf format --profile my-usb`, any label/cluster size/threshold values you did not override on the command line are inherited from the profile.

## Safety Notes

- CDJFormat refuses to operate on drives that appear internal/system or non-removable.
- Drives larger than 1 TB are flagged because Pioneer hardware can behave unpredictably with them.
- Always double-check the reported device identifier before approving a format.

## Troubleshooting

- **Permission errors on macOS** – Run the terminal as an administrator or supply your account password when prompted by `diskutil`.
- **Cluster size validation fails** – Use one of the supported values: `512`, `1K`, `2K`, `4K`, `8K`, `16K`, `32K`, or `64K` (case-insensitive, `B` suffix optional).
- **Slow drive warnings** – Adjust thresholds in a profile if you routinely work with slower media and understand the risks.
- **Read speeds look impossibly high** – The adaptive benchmark already stretches to larger samples, but some OS caches can still return inflated read values on the first pass. Re-run once more or disconnect/reconnect the drive to measure a cold read.

## Contributing

Issues and pull requests are welcome. Please review `CONTRIBUTING.md` for style and workflow guidelines.

## License

Released under the MIT License. See `LICENSE` for details.

## Disclaimer

Formatting will **permanently erase all data** on the selected drive(s). Proceed only if you have verified the device identifier and backed up any important content.
