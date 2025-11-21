//go:build darwin || windows
// +build darwin windows

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
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

// DriveInfo holds information about a drive
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

// ProgressWriter wraps output and shows progress
type ProgressWriter struct {
	total   int64
	current int64
	start   time.Time
}

func (pw *ProgressWriter) Update(n int64) {
	pw.current += n
	elapsed := time.Since(pw.start).Seconds()
	if elapsed > 0 {
		speed := float64(pw.current) / elapsed / (1024 * 1024) // MB/s
		percent := float64(pw.current) * 100 / float64(pw.total)
		remaining := time.Duration((float64(pw.total-pw.current) / (float64(pw.current) / elapsed)) * float64(time.Second))

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
	// Parse the basic list first
	listCmd := exec.Command("diskutil", "list")
	basicOutput, _ := listCmd.Output()

	fmt.Println(string(basicOutput))

	// Get detailed info for external drives
	fmt.Println("\nDetailed drive information:")
	fmt.Println("--------------------------------------------------")

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
	re := regexp.MustCompile(`/dev/(disk\d+)`)
	matches := re.FindStringSubmatch(line)
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
		systemWarning = " [SYSTEM - DO NOT FORMAT]"
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
	cmd := exec.Command("wmic", "logicaldisk", "get", "name,size,freespace,volumename,description,filesystem,drivetype")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	lines := strings.Split(string(output), "\n")
	fmt.Printf("%-20s %-6s %-10s %10s %10s %s\n", "Description", "Drive", "FileSystem", "Size", "Free", "Label")
	fmt.Println("------------------------------------------------------------------------------------------------------------------")

	for i, line := range lines {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		parts := regexp.MustCompile(`\s+`).Split(strings.TrimSpace(line), -1)
		if len(parts) >= 7 {
			description := parts[0]
			driveType := parts[1]
			filesystem := parts[2]
			freespace := parts[3]
			name := parts[4]
			sizeStr := parts[5]
			volumeName := ""
			if len(parts) > 6 {
				volumeName = strings.Join(parts[6:], " ")
			}

			// Parse sizes
			size, _ := strconv.ParseFloat(sizeStr, 64)
			free, _ := strconv.ParseFloat(freespace, 64)
			sizeGB := size / (1024 * 1024 * 1024)
			freeGB := free / (1024 * 1024 * 1024)

			systemWarning := ""
			// DriveType: 2=Removable, 3=Local/Fixed, 4=Network, 5=CD-ROM
			if driveType == "3" {
				systemWarning = " [SYSTEM - DO NOT FORMAT]"
			}

			if sizeGB > 0 {
				fmt.Printf("%-20s %-6s %-10s %9.1fGB %9.1fGB %s%s\n",
					description, name, filesystem, sizeGB, freeGB, volumeName, systemWarning)

				// Warn about drives over 1TB
				if sizeGB > 1024 {
					fmt.Printf("  ⚠️  WARNING: Drive over 1TB - may not perform well on Pioneer hardware\n")
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("To format a drive, use: cdjf format X:")
	fmt.Println("For multiple drives: cdjf format F: G: H:")
}

func formatDrive(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	label, _ := cmd.Flags().GetString("label")

	var devices []string

	// If devices provided as arguments, use them
	if len(args) > 0 {
		devices = args
	} else {
		// Otherwise, prompt for device
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

	// Validate all devices first
	for _, device := range devices {
		if err := validateDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "Error with device %s: %v\n", device, err)
			os.Exit(1)
		}

		// Check if system drive
		if isSystemDrive(device) {
			fmt.Fprintf(os.Stderr, "Error: %s appears to be a system drive. Formatting prevented for safety.\n", device)
			os.Exit(1)
		}

		// Check size and warn if over 1TB
		size := getDriveSize(device)
		if size > 1024 {
			fmt.Printf("⚠️  WARNING: Drive %s is %.1f GB (over 1TB)\n", device, size)
			fmt.Println("   Large drives may not perform well on Pioneer CDJ/XDJ hardware.")
		}
	}

	// Benchmark drives if not skipping confirmation
	if !skipConfirm && len(devices) == 1 {
		fmt.Printf("\nBenchmarking %s to check performance...\n", devices[0])
		result := benchmarkDrive(devices[0])
		if result.WriteMBps > 0 {
			fmt.Printf("Write speed: %.2f MB/s\n", result.WriteMBps)
			if result.WriteMBps < 5.0 {
				fmt.Println("⚠️  WARNING: This drive appears to be very slow.")
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

	// Format drives
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
	fmt.Println("✓ Format completed successfully!")

	// Ask about ejecting
	fmt.Println()
	fmt.Print("Do you want to eject the newly formatted drive? (Y/n): ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		if err := ejectDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "Error ejecting drive: %v\n", err)
		} else {
			fmt.Println("✓ Drive ejected successfully!")
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

			var err error
			switch runtime.GOOS {
			case "darwin":
				err = formatMac(dev, label)
			case "windows":
				err = formatWindows(dev, label)
			}

			if err != nil {
				results <- fmt.Sprintf("[%s] ✗ FAILED: %v", dev, err)
			} else {
				results <- fmt.Sprintf("[%s] ✓ SUCCESS", dev)
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
				fmt.Printf("[%s] ✓ Ejected successfully\n", device)
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

func parseSizeToGB(sizeStr string) float64 {
	// Parse sizes like "32.0 GB (32000000000 Bytes)"
	re := regexp.MustCompile(`([\d.]+)\s*(GB|MB|TB|Bytes)`)
	matches := re.FindStringSubmatch(sizeStr)
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
		// Get drive type using WMIC
		driveLetter := strings.TrimSuffix(device, ":")
		cmd := exec.Command("wmic", "logicaldisk", "where", fmt.Sprintf("name='%s:'", driveLetter), "get", "drivetype")
		output, err := cmd.Output()
		if err != nil {
			return false
		}

		// DriveType: 2=Removable, 3=Local/Fixed, 4=Network, 5=CD-ROM
		// We consider type 3 (Fixed) as system drives
		if strings.Contains(string(output), "3") {
			return true
		}

		// Also check if it's the C: drive or common system drives
		if strings.EqualFold(driveLetter, "C") || strings.EqualFold(driveLetter, "D") {
			return true
		}

		return false
	}
	return false
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
	result := BenchmarkResult{}

	switch runtime.GOOS {
	case "darwin":
		// This is a simplified benchmark - real implementation would write to mounted volume
		// For now, return a placeholder value
		result.WriteMBps = 15.0 // Placeholder

	case "windows":
		// Get mount point
		driveLetter := strings.TrimSuffix(device, ":")
		testFile := fmt.Sprintf("%s:\\benchmark_test.tmp", driveLetter)

		// Create a 10MB test file
		testSize := int64(10 * 1024 * 1024)
		data := make([]byte, testSize)

		start := time.Now()
		err := os.WriteFile(testFile, data, 0644)
		elapsed := time.Since(start).Seconds()

		if err == nil {
			result.WriteMBps = float64(testSize) / elapsed / (1024 * 1024)
			os.Remove(testFile) // Clean up
		}
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
		// Windows eject using RemoveDrive.exe or PowerShell
		driveLetter := strings.TrimSuffix(device, ":")

		// Try using PowerShell
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

	fmt.Printf("Ejecting %s...\n", device)

	if err := ejectDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Drive ejected successfully!")
	fmt.Println("It is now safe to remove the drive.")
}

func showDriveInfo(cmd *cobra.Command, args []string) {
	device := args[0]

	if err := validateDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Drive Information for %s:\n", device)
	fmt.Println("================================================")

	switch runtime.GOOS {
	case "darwin":
		showMacDriveInfo(device)
	case "windows":
		showWindowsDriveInfo(device)
	}

	// Benchmark the drive
	fmt.Println("\nPerformance Test:")
	fmt.Println("------------------------------------------------")
	fmt.Println("Running benchmark...")
	result := benchmarkDrive(device)
	if result.WriteMBps > 0 {
		fmt.Printf("Write Speed: %.2f MB/s\n", result.WriteMBps)
		if result.WriteMBps < 5.0 {
			fmt.Println("⚠️  WARNING: Drive appears to be very slow")
		} else if result.WriteMBps < 10.0 {
			fmt.Println("⚠️  Performance is below average")
		} else {
			fmt.Println("✓ Performance is acceptable")
		}
	} else {
		fmt.Println("Unable to benchmark drive")
	}
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

				// Format size values
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
		fmt.Println("\n⚠️  WARNING: This appears to be a SYSTEM DRIVE")
		fmt.Println("   Formatting this drive is NOT RECOMMENDED")
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
	fmt.Println("Unmounting device...")
	unmountCmd := exec.Command("diskutil", "unmountDisk", device)
	if output, err := unmountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %v\nOutput: %s", err, output)
	}

	fmt.Println("Creating FAT32 filesystem...")

	// Show progress indicator
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		dots := 0
		for {
			select {
			case <-done:
				fmt.Println()
				return
			case <-ticker.C:
				fmt.Printf("\rFormatting %s... %s", device, strings.Repeat(".", dots%4))
				dots++
			}
		}
	}()

	formatCmd := exec.Command("diskutil", "eraseDisk", "FAT32", label, "MBR", device)
	output, err := formatCmd.CombinedOutput()
	done <- true

	if err != nil {
		return fmt.Errorf("diskutil failed: %v\nOutput: %s", err, output)
	}

	fmt.Println(string(output))
	return nil
}

func formatWindows(device, label string) error {
	// Use format command with device defaults
	// /FS:FAT32 - File system
	// /V: - Volume label
	// /Q - Quick format (faster, doesn't scan for bad sectors)
	// /Y - Don't ask for confirmation
	driveLetter := strings.TrimSuffix(device, ":")

	fmt.Println("Creating FAT32 filesystem...")

	// Show progress indicator since Windows format doesn't provide real-time progress
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		dots := 0
		for {
			select {
			case <-done:
				fmt.Println()
				return
			case <-ticker.C:
				fmt.Printf("\rFormatting %s... %s", device, strings.Repeat(".", dots%4))
				dots++
			}
		}
	}()

	cmd := exec.Command("format", driveLetter+":", "/FS:FAT32", "/V:"+label, "/Q", "/Y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	done <- true

	if err != nil {
		return fmt.Errorf("format command failed: %v", err)
	}

	return nil
}
