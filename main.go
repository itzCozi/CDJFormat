package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

var (
	diskIDRegex = regexp.MustCompile(`/dev/(disk\d+)`)
	sizeRegex   = regexp.MustCompile(`([\d.]+)\s*(GB|MB|TB|Bytes)`)
)

var version = "0.1.0"

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "CDJF",
	Short: "CDJF - Prepare USB drives for rekordbox (macOS & Windows)",
	Long: `CDJF is a command line tool designed to help DJs prepare USB drives
for use on standalone systems with rekordbox.

It formats drives to FAT32 with optimal settings for rekordbox compatibility on macOS and Windows.`,
	Version: version,
}

var formatCmd = &cobra.Command{
	Use:   "format [device...]",
	Short: "Format a USB drive for rekordbox",
	Long: `Format one or more USB drives to FAT32 with settings optimized for rekordbox.

WARNING: This will erase all data on the selected drive(s)!

Examples:
	cdjf format disk2          (macOS - single drive)
	cdjf format E:             (Windows - single drive)
	cdjf format F: G: H:       (Windows - multiple drives)`,
	Args: cobra.MinimumNArgs(0),
	Run:  formatDrive,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available drives",
	Long:  `List all available drives that can be formatted for rekordbox.`,
	Run:   listDrives,
}

var ejectCmd = &cobra.Command{
	Use:   "eject [device]",
	Short: "Eject a drive",
	Long: `Safely eject a drive from the system.

Examples:
	cdjf eject disk2       (macOS)
	cdjf eject E:          (Windows)`,
	Args: cobra.ExactArgs(1),
	Run:  ejectDrive,
}

var infoCmd = &cobra.Command{
	Use:   "info [device]",
	Short: "Show drive information",
	Long: `Display detailed information about a drive including capacity, free space, filesystem type, and performance statistics.

Examples:
	cdjf info disk2       (macOS)
	cdjf info E:          (Windows)`,
	Args: cobra.ExactArgs(1),
	Run:  showDriveInfo,
}

func init() {
	rootCmd.AddCommand(formatCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(ejectCmd)
	rootCmd.AddCommand(infoCmd)

	formatCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	formatCmd.Flags().StringP("label", "l", "REKORDBOX", "Volume label for the drive")
}

type DriveInfo struct {
	Device     string
	Label      string
	Filesystem string
	SizeGB     float64
	FreeGB     float64
	Type       string
	IsSystem   bool
}

// BenchmarkResult holds drive performance metrics
type BenchmarkResult struct {
	WriteMBps float64
	ReadMBps  float64
}

func benchmarkSeverity(speed float64) string {
	switch {
	case speed <= 0:
		return "Unable to benchmark drive."
	case speed < 2:
		return "WARNING: Drive appears to be extremely slow."
	case speed < 3:
		return "WARNING: Drive appears to be very slow."
	case speed < 6:
		return "WARNING: Drive appears to be slightly slow."
	default:
		return "Performance is OK."
	}
}

func benchmarkSummary(result BenchmarkResult) string {
	if result.WriteMBps <= 0 && result.ReadMBps <= 0 {
		return benchmarkSeverity(result.WriteMBps)
	}

	lines := []string{benchmarkSeverity(result.WriteMBps)}

	if result.WriteMBps > 0 {
		lines = append(lines, fmt.Sprintf("  Write Speed: %.2f MB/s", result.WriteMBps))
	} else {
		lines = append(lines, "  Write Speed: unavailable")
	}

	if result.ReadMBps > 0 {
		lines = append(lines, fmt.Sprintf("  Read Speed: %.2f MB/s", result.ReadMBps))
	} else {
		lines = append(lines, "  Read Speed: unavailable")
	}

	return strings.Join(lines, "\n")
}

// ProgressWriter wraps output and shows progress
type ProgressWriter struct {
	total   int64
	current int64
	start   time.Time
}

func (pw *ProgressWriter) Update(n int64) {
	pw.current += n
	elapsed := time.Since(pw.start).Seconds()
	if elapsed > 0 && pw.current > 0 && pw.total > 0 {
		bytesPerSecond := float64(pw.current) / elapsed
		speed := bytesPerSecond / (1024 * 1024) // MB/s
		percent := float64(pw.current) * 100 / float64(pw.total)

		remainingBytes := float64(pw.total - pw.current)
		remainingSeconds := remainingBytes / bytesPerSecond
		remaining := time.Duration(remainingSeconds * float64(time.Second))

		fmt.Printf("\rProgress: %.1f%% | Speed: %.2f MB/s | Remaining: %s     ",
			percent, speed, formatDuration(remaining))
	}
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "calculating..."
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func listDrives(cmd *cobra.Command, args []string) {
	fmt.Println("Available drives:")
	fmt.Println()

	switch runtime.GOOS {
	case "darwin":
		listMacDrives()
	case "windows":
		listWindowsDrives()
	default:
		fmt.Fprintf(os.Stderr, "Unsupported operating system: %s\n", runtime.GOOS)
		os.Exit(1)
	}
}

func listMacDrives() {
	listCmd := exec.Command("diskutil", "list")
	basicOutput, _ := listCmd.Output()

	fmt.Println(string(basicOutput))

	fmt.Println()
	detailTitle := "Detailed drive information:"
	fmt.Println(detailTitle)
	fmt.Println(strings.Repeat("-", len(detailTitle)))

	infoCmd := exec.Command("diskutil", "list", "external", "physical")
	externalOutput, err := infoCmd.Output()
	if err == nil {
		lines := strings.Split(string(externalOutput), "\n")
		for _, line := range lines {
			if strings.Contains(line, "/dev/disk") {
				diskID := extractDiskID(line)
				if diskID != "" {
					showMacDriveDetails(diskID)
				}
			}
		}
	}

	fmt.Println("\nTo format a drive, use: cdjf format diskX")
}

func extractDiskID(line string) string {
	matches := diskIDRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func showMacDriveDetails(diskID string) {
	cmd := exec.Command("diskutil", "info", diskID)
	output, err := cmd.Output()
	if err != nil {
		return
	}

	info := parseMacDiskInfo(output)
	if info.Type == "" {
		return
	}

	systemWarning := ""
	if info.IsSystem {
		systemWarning = " [SYSTEM]"
	}

	fmt.Printf("%-20s %-10s %-10s %8.1f GB%s\n",
		info.Type, diskID, info.Filesystem, info.SizeGB, systemWarning)
}

func parseMacDiskInfo(output []byte) DriveInfo {
	info := DriveInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Device / Media Name:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Type = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "File System Personality:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Filesystem = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Disk Size:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				sizeStr := strings.TrimSpace(parts[1])
				info.SizeGB = parseSizeToGB(sizeStr)
			}
		} else if strings.Contains(line, "Volume Name:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				info.Label = strings.TrimSpace(parts[1])
			}
		} else if strings.Contains(line, "Internal:") && strings.Contains(line, "Yes") {
			info.IsSystem = true
		}
	}

	return info
}

func listWindowsDrives() {
	cmd := exec.Command("wmic", "logicaldisk", "get", "DeviceID,DriveType,FileSystem,FreeSpace,Size,VolumeName", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	lines := strings.Split(string(output), "\n")

	foundRemovable := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node,") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		deviceID := strings.TrimSpace(parts[1])
		driveType := strings.TrimSpace(parts[2])
		filesystem := strings.TrimSpace(parts[3])
		freeStr := strings.TrimSpace(parts[4])
		sizeStr := strings.TrimSpace(parts[5])
		label := ""
		if len(parts) > 6 {
			label = strings.TrimSpace(parts[6])
		}

		if driveType != "2" {
			continue
		}

		sizeGB := bytesToGB(sizeStr)
		freeGB := bytesToGB(freeStr)

		if sizeGB <= 0 {
			continue
		}

		typeLabel := driveTypeLabel(driveType)

		fmt.Printf("%-12s %-6s %-10s %9.1fGB %9.1fGB   %-20s\n",
			typeLabel, deviceID, filesystem, sizeGB, freeGB, label)
		foundRemovable = true

		if sizeGB > 1024 && driveType == "2" {
			fmt.Println("    WARNING: Drive over 1TB - may not perform well on Pioneer hardware")
		}
	}

	if !foundRemovable {
		fmt.Println("No removable drives found...")
	}

	fmt.Println()
	fmt.Println("To format a drive, use: cdjf format X:")
	fmt.Println("For multiple drives: cdjf format F: G: H:")
}

func bytesToGB(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	bytes, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return bytes / (1024 * 1024 * 1024)
}

func driveTypeLabel(code string) string {
	switch code {
	case "1":
		return "NoRoot"
	case "2":
		return "Removable"
	case "3":
		return "Local"
	case "4":
		return "Network"
	case "5":
		return "CDROM"
	case "6":
		return "RAMDisk"
	default:
		return "Unknown"
	}
}

func formatDrive(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	label, _ := cmd.Flags().GetString("label")

	var devices []string

	if len(args) > 0 {
		devices = args
	} else {
		fmt.Println("Available drives:")
		listDrives(cmd, args)
		fmt.Println()
		fmt.Print("Enter device(s) to format (space-separated for multiple): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		deviceStr := strings.TrimSpace(input)
		if deviceStr == "" {
			fmt.Fprintln(os.Stderr, "Error: No device specified")
			os.Exit(1)
		}
		devices = strings.Fields(deviceStr)
	}

	if len(devices) == 0 {
		fmt.Fprintln(os.Stderr, "Error: No device specified")
		os.Exit(1)
	}

	for _, device := range devices {
		if err := validateDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "Error with device %s: %v\n", device, err)
			os.Exit(1)
		}

		if err := ensureRemovableDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "Error with device %s: %v\n", device, err)
			os.Exit(1)
		}

		// Check size and warn if over 1TB
		size := getDriveSize(device)
		if size > 1024 {
			fmt.Printf("  WARNING: Drive %s is %.1f GB (over 1TB)\n", device, size)
			fmt.Println("   Large drives may not perform well on Pioneer CDJ/XDJ hardware.")
		}
	}

	if !skipConfirm && len(devices) == 1 {
		fmt.Printf("\nBenchmarking %s to check performance...\n", devices[0])
		result := benchmarkDrive(devices[0])
		fmt.Println(benchmarkSummary(result))
		if result.WriteMBps > 0 && result.WriteMBps < 5.0 {
			fmt.Print("   Do you want to proceed anyway? (yes/no): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "yes" && response != "y" {
				fmt.Println("Format cancelled.")
				return
			}
		}
	}

	if !skipConfirm {
		fmt.Println()
		fmt.Println("! WARNING !")
		if len(devices) == 1 {
			fmt.Printf("This will ERASE ALL DATA on %s\n", devices[0])
		} else {
			fmt.Printf("This will ERASE ALL DATA on %d drives: %s\n", len(devices), strings.Join(devices, ", "))
		}
		fmt.Println()
		fmt.Print("Are you sure you want to continue? (yes/no): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "yes" && response != "y" {
			fmt.Println("Format cancelled.")
			return
		}
	}

	if len(devices) == 1 {
		// Single drive - format directly
		formatSingleDrive(devices[0], label)
	} else {
		// Multiple drives - use threading
		fmt.Printf("\nFormatting %d drives concurrently...\n\n", len(devices))
		formatMultipleDrives(devices, label)
	}
}

func formatSingleDrive(device, label string) {
	if err := ensureRemovableDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Refusing to format %s: %v\n", device, err)
		os.Exit(1)
	}
	label = getUniqueLabel(label, device)

	fmt.Printf("\nFormatting %s to FAT32...\n", device)

	switch runtime.GOOS {
	case "darwin":
		if err := formatMac(device, label); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting drive: %v\n", err)
			os.Exit(1)
		}
	case "windows":
		if err := formatWindows(device, label); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting drive: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unsupported operating system: %s\n", runtime.GOOS)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Format completed successfully!")

	fmt.Println()
	fmt.Print("Do you want to eject the newly formatted drive? (Y/n): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		if err := ejectDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "Error ejecting drive: %v\n", err)
		} else {
			fmt.Println("Drive ejected successfully!")
		}
	}

	fmt.Println()
	fmt.Println("Your USB drive is now ready for rekordbox.")
	fmt.Println("You can now:")
	fmt.Println("  1. Connect the drive to your computer with rekordbox installed")
	fmt.Println("  2. Open rekordbox and add your music to the drive")
	fmt.Println("  3. Safely eject the drive and use it on CDJ/XDJ players")
}

func formatMultipleDrives(devices []string, baseLabel string) {
	var wg sync.WaitGroup
	results := make(chan string, len(devices))

	for i, device := range devices {
		wg.Add(1)
		go func(dev string, idx int) {
			defer wg.Done()

			// Create unique label for each drive
			label := baseLabel
			if idx > 0 {
				label = fmt.Sprintf("%s%d", baseLabel, idx+1)
			}
			label = getUniqueLabel(label, dev)

			fmt.Printf("[%s] Starting format...\n", dev)

			if err := ensureRemovableDevice(dev); err != nil {
				results <- fmt.Sprintf("[%s] FAILED: %v", dev, err)
				return
			}

			var err error
			switch runtime.GOOS {
			case "darwin":
				err = formatMac(dev, label)
			case "windows":
				err = formatWindows(dev, label)
			}

			if err != nil {
				results <- fmt.Sprintf("[%s] FAILED: %v", dev, err)
			} else {
				results <- fmt.Sprintf("[%s] SUCCESS", dev)
			}
		}(device, i)
	}

	wg.Wait()
	close(results)

	fmt.Println("\n=== Format Results ===")
	for result := range results {
		fmt.Println(result)
	}

	// Ask about ejecting all drives
	fmt.Println()
	fmt.Print("Do you want to eject all newly formatted drives? (Y/n): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		for _, device := range devices {
			if err := ejectDevice(device); err != nil {
				fmt.Printf("[%s] Error ejecting: %v\n", device, err)
			} else {
				fmt.Printf("[%s] Ejected successfully\n", device)
			}
		}
	}

	fmt.Println()
	fmt.Println("All drives are now ready for rekordbox.")
}

func validateDevice(device string) error {
	switch runtime.GOOS {
	case "darwin":
		if !strings.HasPrefix(device, "disk") {
			return fmt.Errorf("invalid device format. Expected diskN (e.g., disk2)")
		}
	case "windows":
		if len(device) < 2 || device[1] != ':' {
			return fmt.Errorf("invalid drive format. Expected X: (e.g., E:)")
		}
	}
	return nil
}

func ensureRemovableDevice(device string) error {
	if isSystemDrive(device) {
		return fmt.Errorf("%s appears to be a system/internal drive. Operation blocked for safety", device)
	}

	if !isRemovableDrive(device) {
		return fmt.Errorf("%s is not detected as a removable USB drive. Only removable drives are supported", device)
	}

	return nil
}

func parseSizeToGB(sizeStr string) float64 {
	matches := sizeRegex.FindStringSubmatch(sizeStr)
	if len(matches) >= 3 {
		size, _ := strconv.ParseFloat(matches[1], 64)
		unit := matches[2]
		switch unit {
		case "TB":
			return size * 1024
		case "GB":
			return size
		case "MB":
			return size / 1024
		case "Bytes":
			return size / (1024 * 1024 * 1024)
		}
	}
	return 0
}

func isSystemDrive(device string) bool {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "info", device)
		output, err := cmd.Output()
		if err != nil {
			return false
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Internal:") && strings.Contains(line, "Yes") {
				return true
			}
			if strings.Contains(line, "System Image:") && strings.Contains(line, "Yes") {
				return true
			}
		}
		return false

	case "windows":
		driveLetter := strings.TrimSuffix(device, ":")
		driveType, err := windowsDriveType(device)
		if err == nil && driveType == "3" {
			return true
		}
		if strings.EqualFold(driveLetter, "C") {
			return true
		}
		return false
	}
	return false
}

func isRemovableDrive(device string) bool {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "info", device)
		output, err := cmd.Output()
		if err != nil {
			return false
		}

		lines := strings.Split(string(output), "\n")
		internal := false
		removable := false
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Internal:") && strings.Contains(line, "Yes") {
				internal = true
			}
			if strings.HasPrefix(line, "Removable Media:") && strings.Contains(line, "Yes") {
				removable = true
			}
			if strings.HasPrefix(line, "Ejectable:") && strings.Contains(line, "Yes") {
				removable = true
			}
			if strings.HasPrefix(line, "External:") && strings.Contains(line, "Yes") {
				removable = true
			}
		}
		if internal {
			return false
		}
		return removable

	case "windows":
		driveType, err := windowsDriveType(device)
		if err != nil {
			return false
		}
		return driveType == "2"
	}
	return false
}

func windowsDriveType(device string) (string, error) {
	driveLetter := strings.TrimSuffix(device, ":")
	if driveLetter == "" {
		return "", fmt.Errorf("invalid drive letter")
	}

	cmd := exec.Command("wmic", "logicaldisk", "where", fmt.Sprintf("name='%s:'", driveLetter), "get", "drivetype")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "DriveType") {
			continue
		}
		return line, nil
	}

	return "", fmt.Errorf("drive type not found")
}

func getDriveSize(device string) float64 {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "info", device)
		output, err := cmd.Output()
		if err != nil {
			return 0
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "Disk Size:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return parseSizeToGB(parts[1])
				}
			}
		}

	case "windows":
		driveLetter := strings.TrimSuffix(device, ":")
		cmd := exec.Command("wmic", "logicaldisk", "where", fmt.Sprintf("name='%s:'", driveLetter), "get", "size")
		output, err := cmd.Output()
		if err != nil {
			return 0
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && line != "Size" {
				size, err := strconv.ParseFloat(line, 64)
				if err == nil {
					return size / (1024 * 1024 * 1024) // Convert bytes to GB
				}
			}
		}
	}
	return 0
}

func benchmarkDrive(device string) BenchmarkResult {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "info", device)
		output, err := cmd.Output()
		if err != nil {
			return BenchmarkResult{}
		}

		lines := strings.Split(string(output), "\n")
		var mountPoint string
		for _, line := range lines {
			if strings.Contains(line, "Mount Point:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					mountPoint = strings.TrimSpace(parts[1])
					break
				}
			}
		}

		if mountPoint == "" || mountPoint == "Not applicable" {
			return BenchmarkResult{}
		}

		return runIOMeasure(fmt.Sprintf("%s/benchmark_test.tmp", mountPoint))

	case "windows":
		driveLetter := strings.TrimSuffix(device, ":")
		if driveLetter == "" {
			return BenchmarkResult{}
		}
		return runIOMeasure(fmt.Sprintf("%s:\\benchmark_test.tmp", driveLetter))
	}

	return BenchmarkResult{}
}

func runIOMeasure(testFile string) BenchmarkResult {
	const testSize = 10 * 1024 * 1024 // 10 MB sample file

	result := BenchmarkResult{}
	chunk := make([]byte, 1024*1024)

	file, err := os.Create(testFile)
	if err != nil {
		return result
	}
	defer os.Remove(testFile)

	// Create progress bar for write test
	writeBar := progressbar.NewOptions(testSize,
		progressbar.OptionSetDescription("Write test"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowBytes(true),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)

	writeStart := time.Now()
	bytesWritten := 0
	for bytesWritten < testSize {
		toWrite := len(chunk)
		remaining := testSize - bytesWritten
		if remaining < toWrite {
			toWrite = remaining
		}

		n, writeErr := file.Write(chunk[:toWrite])
		if writeErr != nil {
			file.Close()
			return result
		}
		bytesWritten += n
		writeBar.Add(n)
	}

	if syncErr := file.Sync(); syncErr != nil {
		file.Close()
		return result
	}

	if closeErr := file.Close(); closeErr != nil {
		return result
	}

	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed > 0 {
		result.WriteMBps = float64(testSize) / writeElapsed / (1024 * 1024)
	}

	readFile, err := os.Open(testFile)
	if err != nil {
		return result
	}
	defer readFile.Close()

	// Create progress bar for read test
	readBar := progressbar.NewOptions(testSize,
		progressbar.OptionSetDescription("Read test"),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowBytes(true),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)

	readStart := time.Now()
	totalRead := 0
	for {
		n, readErr := readFile.Read(chunk)
		if n > 0 {
			totalRead += n
			readBar.Add(n)
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return result
		}

		if n == 0 {
			break
		}
	}

	readElapsed := time.Since(readStart).Seconds()
	if readElapsed > 0 && totalRead > 0 {
		result.ReadMBps = float64(totalRead) / readElapsed / (1024 * 1024)
	}

	return result
}

func ejectDevice(device string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "eject", device)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("eject failed: %v\nOutput: %s", err, output)
		}
		return nil

	case "windows":
		driveLetter := strings.TrimSuffix(device, ":")

		psCmd := fmt.Sprintf("(New-Object -comObject Shell.Application).NameSpace(17).ParseName('%s:').InvokeVerb('Eject')", driveLetter)
		cmd := exec.Command("powershell", "-Command", psCmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("eject failed: %v\nOutput: %s", err, output)
		}
		return nil
	}

	return fmt.Errorf("unsupported operating system")
}

func ejectDrive(cmd *cobra.Command, args []string) {
	device := args[0]

	if err := validateDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ensureRemovableDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Ejecting %s...\n", device)

	if err := ejectDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Drive ejected successfully!")
	fmt.Println("It is now safe to remove the drive.")
}

func showDriveInfo(cmd *cobra.Command, args []string) {
	device := args[0]

	if err := validateDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ensureRemovableDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	infoTitle := fmt.Sprintf("Drive Information for %s", device)
	fmt.Println(infoTitle)
	fmt.Println(strings.Repeat("=", len(infoTitle)))

	switch runtime.GOOS {
	case "darwin":
		showMacDriveInfo(device)
	case "windows":
		showWindowsDriveInfo(device)
	}

	fmt.Println()
	perfTitle := "Performance Test:"
	fmt.Println(perfTitle)
	fmt.Println(strings.Repeat("-", len(perfTitle)))
	fmt.Println("Running benchmark...")
	result := benchmarkDrive(device)
	fmt.Println(benchmarkSummary(result))
}

func showMacDriveInfo(device string) {
	cmd := exec.Command("diskutil", "info", device)
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting drive info: %v\n", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	relevantFields := []string{
		"Device / Media Name:",
		"Volume Name:",
		"File System Personality:",
		"Disk Size:",
		"Volume Free Space:",
		"Volume Used Space:",
		"Internal:",
		"Removable Media:",
	}

	for _, line := range lines {
		for _, field := range relevantFields {
			if strings.Contains(line, field) {
				fmt.Println(line)
				break
			}
		}
	}
}

func showWindowsDriveInfo(device string) {
	driveLetter := strings.TrimSuffix(device, ":")
	cmd := exec.Command("wmic", "logicaldisk", "where", fmt.Sprintf("name='%s:'", driveLetter),
		"get", "description,filesystem,freespace,size,volumename,drivetype")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting drive info: %v\n", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		headers := strings.Fields(lines[0])
		values := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(lines[1]), -1)

		for i, header := range headers {
			if i < len(values) {
				value := values[i]

				if header == "Size" || header == "FreeSpace" {
					if size, err := strconv.ParseFloat(value, 64); err == nil {
						sizeGB := size / (1024 * 1024 * 1024)
						value = fmt.Sprintf("%.2f GB", sizeGB)
					}
				}

				// Interpret drive type
				if header == "DriveType" {
					switch value {
					case "2":
						value = "Removable"
					case "3":
						value = "Fixed/Local"
					case "4":
						value = "Network"
					case "5":
						value = "CD-ROM"
					}
				}

				fmt.Printf("%-20s: %s\n", header, value)
			}
		}
	}

	// Check if system drive
	if isSystemDrive(device) {
		fmt.Println("\n  WARNING: This appears to be a SYSTEM DRIVE")
		fmt.Println("  Formatting this drive is NOT RECOMMENDED")
	}
}

func getExistingLabels(excludeDevice string) map[string]bool {
	labels := make(map[string]bool)

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "list", "-plist")
		output, err := cmd.Output()
		if err != nil {
			return labels
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "VolumeName") {
				continue
			}
		}
	case "windows":
		cmd := exec.Command("wmic", "logicaldisk", "get", "name,volumename")
		output, err := cmd.Output()
		if err != nil {
			return labels
		}

		lines := strings.Split(string(output), "\n")
		for i, line := range lines {
			if i == 0 || strings.TrimSpace(line) == "" {
				continue
			}

			parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(line), 2)
			if len(parts) >= 2 {
				driveLetter := strings.TrimSuffix(parts[0], ":")
				volumeName := strings.TrimSpace(parts[1])

				if excludeDevice != "" && strings.HasPrefix(excludeDevice, driveLetter) {
					continue
				}

				if volumeName != "" {
					labels[strings.ToUpper(volumeName)] = true
				}
			}
		}
	}

	return labels
}

func getUniqueLabel(baseLabel, device string) string {
	existingLabels := getExistingLabels(device)

	if !existingLabels[strings.ToUpper(baseLabel)] {
		return baseLabel
	}

	for i := 2; i <= 99; i++ {
		candidate := baseLabel + strconv.Itoa(i)
		if !existingLabels[strings.ToUpper(candidate)] {
			fmt.Printf("Label '%s' already exists, using '%s' instead\n", baseLabel, candidate)
			return candidate
		}
	}

	return baseLabel
}

func formatMac(device, label string) error {
	if err := ensureRemovableDevice(device); err != nil {
		return err
	}
	fmt.Println("Unmounting device...")
	unmountCmd := exec.Command("diskutil", "unmountDisk", device)
	if output, err := unmountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %v\nOutput: %s", err, output)
	}

	fmt.Println("Creating FAT32 filesystem...")

	// Show progress indicator
	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Formatting "+device),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
	)

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				bar.Add(1)
			}
		}
	}()

	formatCmd := exec.Command("diskutil", "eraseDisk", "FAT32", label, "MBR", device)
	output, err := formatCmd.CombinedOutput()
	done <- true
	bar.Finish()
	fmt.Println()

	if err != nil {
		return fmt.Errorf("diskutil failed: %v\nOutput: %s", err, output)
	}

	fmt.Println(string(output))
	return nil
}

func formatWindows(device, label string) error {
	if err := ensureRemovableDevice(device); err != nil {
		return err
	}
	// Use format command with device defaults
	// /FS:FAT32 - File system
	// /V: - Volume label
	// /Q - Quick format (faster, doesn't scan for bad sectors)
	// /Y - Don't ask for confirmation
	driveLetter := strings.TrimSuffix(device, ":")

	fmt.Println("Creating FAT32 filesystem...")

	bar := progressbar.NewOptions(-1,
		progressbar.OptionSetDescription("Formatting "+device),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100*time.Millisecond),
	)

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				bar.Add(1)
			}
		}
	}()

	cmd := exec.Command("format", driveLetter+":", "/FS:FAT32", "/V:"+label, "/Q", "/Y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	done <- true
	bar.Finish()
	fmt.Println()

	if err != nil {
		return fmt.Errorf("format command failed: %v", err)
	}

	return nil
}
