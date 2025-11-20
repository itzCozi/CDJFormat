package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
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
	Use:   "cdjformat",
	Short: "CDJFormat - A tool to prepare USB drives for rekordbox",
	Long: `CDJFormat is a command line tool designed to help DJ's prepare USB drives
for use on standalone systems with rekordbox.

It formats drives to FAT32 with optimal settings for rekordbox compatibility.`,
	Version: version,
}

var formatCmd = &cobra.Command{
	Use:   "format [device]",
	Short: "Format a USB drive for rekordbox",
	Long: `Format a USB drive to FAT32 with settings optimized for rekordbox.

WARNING: This will erase all data on the selected drive!

Example:
  cdjformat format /dev/sdb    (Linux)
  cdjformat format disk2       (macOS)
  cdjformat format E:          (Windows)`,
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
	case "linux":
		listLinuxDrives()
	case "darwin":
		listMacDrives()
	case "windows":
		listWindowsDrives()
	default:
		fmt.Fprintf(os.Stderr, "Unsupported operating system: %s\n", runtime.GOOS)
		os.Exit(1)
	}
}

func listLinuxDrives() {
	// Use lsblk to list block devices
	cmd := exec.Command("lsblk", "-o", "NAME,SIZE,TYPE,MOUNTPOINT,VENDOR,MODEL", "-n")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		fmt.Println("Note: You may need to run this command with sudo")
		return
	}

	fmt.Println("Device\tSize\tType\tMount\tInfo")
	fmt.Println(strings.Repeat("-", 70))

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 && (fields[2] == "disk" || fields[2] == "part") {
			fmt.Println(line)
		}
	}

	fmt.Println()
	fmt.Println("To format a drive, use: cdjformat format /dev/sdX")
}

func listMacDrives() {
	// Use diskutil to list disks
	cmd := exec.Command("diskutil", "list")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	fmt.Println(string(output))
	fmt.Println("To format a drive, use: cdjformat format diskX")
}

func listWindowsDrives() {
	// Use wmic to list drives
	cmd := exec.Command("wmic", "logicaldisk", "get", "name,size,volumename,description")
	output, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing drives: %v\n", err)
		return
	}

	fmt.Println(string(output))
	fmt.Println("To format a drive, use: cdjformat format X:")
}

func formatDrive(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	label, _ := cmd.Flags().GetString("label")

	var device string
	if len(args) > 0 {
		device = args[0]
	} else {
		// Interactive mode: show drives and ask user to select
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

	// Validate device exists
	if err := validateDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Show warning and get confirmation
	if !skipConfirm {
		fmt.Println()
		fmt.Println("⚠️  WARNING ⚠️")
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

	// Perform the format
	fmt.Printf("\nFormatting %s to FAT32...\n", device)

	switch runtime.GOOS {
	case "linux":
		if err := formatLinux(device, label); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting drive: %v\n", err)
			os.Exit(1)
		}
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
	case "linux":
		// Check if device exists
		if _, err := os.Stat(device); os.IsNotExist(err) {
			return fmt.Errorf("device %s does not exist", device)
		}
	case "darwin":
		// Validate disk format (should be diskN)
		if !strings.HasPrefix(device, "disk") {
			return fmt.Errorf("invalid device format. Expected diskN (e.g., disk2)")
		}
	case "windows":
		// Validate drive letter format
		if len(device) < 2 || device[1] != ':' {
			return fmt.Errorf("invalid drive format. Expected X: (e.g., E:)")
		}
	}
	return nil
}

func formatLinux(device, label string) error {
	// Unmount the device first if it's mounted
	fmt.Println("Unmounting device...")
	exec.Command("umount", device).Run() // Ignore errors if not mounted

	// Use mkfs.vfat to format as FAT32
	// -F 32: Force FAT32
	// -n: Volume label
	// -I: Don't ask questions
	fmt.Println("Creating FAT32 filesystem...")
	cmd := exec.Command("mkfs.vfat", "-F", "32", "-n", label, "-I", device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mkfs.vfat failed: %v\nOutput: %s", err, output)
	}

	return nil
}

func formatMac(device, label string) error {
	// Use diskutil to format
	// First unmount
	fmt.Println("Unmounting device...")
	unmountCmd := exec.Command("diskutil", "unmountDisk", device)
	if output, err := unmountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %v\nOutput: %s", err, output)
	}

	// Format as FAT32 (MS-DOS FAT32)
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
	// Use format command
	// /FS:FAT32 - File system
	// /V: - Volume label
	// /Q - Quick format
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
