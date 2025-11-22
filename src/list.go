package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

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
