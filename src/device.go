package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type DriveInfo struct {
	Device     string
	Label      string
	Filesystem string
	SizeGB     float64
	FreeGB     float64
	Type       string
	IsSystem   bool
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
					return size / (1024 * 1024 * 1024)
				}
			}
		}
	}
	return 0
}

func resolveTestFilePath(device, fileName string) (string, string, error) {
	mountPoint, err := getDeviceMountPoint(device)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(mountPoint, fileName), mountPoint, nil
}

func getDeviceMountPoint(device string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("diskutil", "info", device)
		output, err := cmd.Output()
		if err != nil {
			return "", err
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

		if mountPoint == "" || strings.EqualFold(mountPoint, "Not mounted") || strings.EqualFold(mountPoint, "Not applicable") {
			return "", fmt.Errorf("device %s is not mounted; please mount it before verifying", device)
		}

		if _, err := os.Stat(mountPoint); err != nil {
			return "", fmt.Errorf("unable to access mount point %s: %w", mountPoint, err)
		}

		return mountPoint, nil

	case "windows":
		driveLetter := strings.TrimSuffix(device, ":")
		if driveLetter == "" {
			return "", fmt.Errorf("invalid drive format: %s", device)
		}
		path := fmt.Sprintf("%s:\\", strings.ToUpper(driveLetter))
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("unable to access %s: %w", path, err)
		}
		return path, nil
	}

	return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
}
