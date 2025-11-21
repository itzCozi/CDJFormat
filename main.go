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
	Use:   "format [device]",
	Short: "Format a USB drive for rekordbox",
	Long: `Format a USB drive to FAT32 with settings optimized for rekordbox.

WARNING: This will erase all data on the selected drive!

Examples:
	cdjf format disk2       (macOS)
	cdjf format E:          (Windows)`,
	Args: cobra.MaximumNArgs(1),
	Run:  formatDrive,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available drives",
	Long:  `List all available drives that can be formatted for rekordbox.`,
	Run:   listDrives,
}

func init() {
	rootCmd.AddCommand(formatCmd)
	rootCmd.AddCommand(listCmd)

	formatCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	formatCmd.Flags().StringP("label", "l", "REKORDBOX", "Volume label for the drive")
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
	cmd := exec.Command("diskutil", "list")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	fmt.Println(string(output))
	fmt.Println("To format a drive, use: cdjf format diskX")
}

func listWindowsDrives() {
	cmd := exec.Command("wmic", "logicaldisk", "get", "name,size,volumename,description")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	fmt.Println(string(output))
	fmt.Println("To format a drive, use: cdjf format X:")
}

func formatDrive(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	label, _ := cmd.Flags().GetString("label")

	var device string
	if len(args) > 0 {
		device = args[0]
	} else {
		fmt.Println("Available drives:")
		listDrives(cmd, args)
		fmt.Println()
		fmt.Print("Enter device to format: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		device = strings.TrimSpace(input)
	}

	if device == "" {
		fmt.Fprintln(os.Stderr, "Error: No device specified")
		os.Exit(1)
	}

	if err := validateDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	label = getUniqueLabel(label, device)

	if !skipConfirm {
		fmt.Println()
		fmt.Println("! WARNING !")
		fmt.Printf("This will ERASE ALL DATA on %s\n", device)
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
	fmt.Println()
	fmt.Println("Your USB drive is now ready for rekordbox.")
	fmt.Println("You can now:")
	fmt.Println("  1. Connect the drive to your computer with rekordbox installed")
	fmt.Println("  2. Open rekordbox and add your music to the drive")
	fmt.Println("  3. Safely eject the drive and use it on CDJ/XDJ players")
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
	formatCmd := exec.Command("diskutil", "eraseDisk", "FAT32", label, "MBR", device)
	output, err := formatCmd.CombinedOutput()
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
	cmd := exec.Command("format", driveLetter+":", "/FS:FAT32", "/V:"+label, "/Q", "/Y")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("format command failed: %v", err)
	}

	return nil
}
