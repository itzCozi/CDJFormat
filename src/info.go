package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

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
	fmt.Println(benchmarkSummary(result, defaultBenchmarkThresholds))
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

	if isSystemDrive(device) {
		fmt.Println("\n  WARNING: This appears to be a SYSTEM DRIVE")
		fmt.Println("  Formatting this drive is NOT RECOMMENDED")
	}
}
