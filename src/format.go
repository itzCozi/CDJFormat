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

	"github.com/spf13/cobra"
)

func formatDrive(cmd *cobra.Command, args []string) {
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	label, _ := cmd.Flags().GetString("label")
	clusterSizeInput, _ := cmd.Flags().GetString("cluster-size")
	profileName, _ := cmd.Flags().GetString("profile")

	clusterSize := strings.TrimSpace(clusterSizeInput)
	thresholds := defaultBenchmarkThresholds

	if profileName != "" {
		profile, err := loadProfileByName(profileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading profile %q: %v\n", profileName, err)
			os.Exit(1)
		}

		displayName := profileDisplayName(profile, profileName)
		fmt.Printf("Applying profile %q\n", displayName)

		if profile.BenchmarkThresholds != nil {
			thresholds = mergedBenchmarkThresholds(profile.BenchmarkThresholds)
		}

		if !cmd.Flags().Changed("label") && strings.TrimSpace(profile.Label) != "" {
			label = profile.Label
		}

		if clusterSize == "" && strings.TrimSpace(profile.ClusterSize) != "" {
			clusterSize = profile.ClusterSize
		}
	}

	if clusterSize != "" {
		normalized, err := normalizeClusterSize(clusterSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		clusterSize = normalized
	}

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

		size := getDriveSize(device)
		if size > 1024 {
			fmt.Printf("  WARNING: Drive %s is %.1f GB (over 1TB)\n", device, size)
			fmt.Println("   Large drives may not perform well on Pioneer CDJ/XDJ hardware.")
		}
	}

	if !skipConfirm && len(devices) == 1 {
		fmt.Printf("\nBenchmarking %s to check performance...\n", devices[0])
		result := benchmarkDrive(devices[0])
		fmt.Println(benchmarkSummary(result, thresholds))
		if thresholds.Prompt > 0 && result.WriteMBps > 0 && result.WriteMBps < thresholds.Prompt {
			fmt.Print("   Do you want to proceed anyway? (Y/n): ")
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
		fmt.Print("Are you sure you want to continue? (Y/n): ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "yes" && response != "y" {
			fmt.Println("Format cancelled.")
			return
		}
	}

	if len(devices) == 1 {
		formatSingleDrive(devices[0], label, clusterSize)
	} else {
		fmt.Printf("\nFormatting %d drives concurrently...\n\n", len(devices))
		formatMultipleDrives(devices, label, clusterSize)
	}
}

func formatSingleDrive(device, label, clusterSize string) {
	if err := ensureRemovableDevice(device); err != nil {
		fmt.Fprintf(os.Stderr, "Refusing to format %s: %v\n", device, err)
		os.Exit(1)
	}
	label = getUniqueLabel(label, device)

	fmt.Printf("\nFormatting %s to FAT32...\n", device)

	switch runtime.GOOS {
	case "darwin":
		if err := formatMac(device, label, clusterSize); err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting drive: %v\n", err)
			os.Exit(1)
		}
	case "windows":
		if err := formatWindows(device, label, clusterSize); err != nil {
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
	fmt.Printf("  4. (Recommended) Run 'cdjf verify %s' to confirm the drive's health before loading music.\n", device)
}

func formatMultipleDrives(devices []string, baseLabel, clusterSize string) {
	var wg sync.WaitGroup
	results := make(chan string, len(devices))

	for i, device := range devices {
		wg.Add(1)
		go func(dev string, idx int) {
			defer wg.Done()

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
				err = formatMac(dev, label, clusterSize)
			case "windows":
				err = formatWindows(dev, label, clusterSize)
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
	fmt.Println("For extra peace of mind, run 'cdjf verify <drive>' on each drive before loading music.")
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

func normalizeClusterSize(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	upper := strings.ToUpper(trimmed)
	upper = strings.TrimSuffix(upper, "B")

	allowed := map[string]string{
		"512":   "512",
		"1K":    "1K",
		"1024":  "1K",
		"2K":    "2K",
		"2048":  "2K",
		"4K":    "4K",
		"4096":  "4K",
		"8K":    "8K",
		"8192":  "8K",
		"16K":   "16K",
		"16384": "16K",
		"32K":   "32K",
		"32768": "32K",
		"64K":   "64K",
		"65536": "64K",
	}

	if canonical, ok := allowed[upper]; ok {
		return canonical, nil
	}

	return "", fmt.Errorf("invalid cluster size %q; supported values: 512, 1K, 2K, 4K, 8K, 16K, 32K, 64K", value)
}

func formatMac(device, label, clusterSize string) error {
	if err := ensureRemovableDevice(device); err != nil {
		return err
	}
	if clusterSize != "" {
		fmt.Println("Note: custom cluster size is not currently supported on macOS; using default size.")
	}
	fmt.Println("Unmounting device...")
	unmountCmd := exec.Command("diskutil", "unmountDisk", device)
	if output, err := unmountCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to unmount: %v\nOutput: %s", err, output)
	}

	fmt.Println("Creating FAT32 filesystem...")

	formatCmd := exec.Command("diskutil", "eraseDisk", "FAT32", label, "MBR", device)
	stdout, err := formatCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("diskutil stdout: %v", err)
	}
	stderr, err := formatCmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("diskutil stderr: %v", err)
	}

	if err := formatCmd.Start(); err != nil {
		return fmt.Errorf("diskutil failed to start: %v", err)
	}

	progress := NewProgressBar("Format", 100)
	defer progress.Stop()

	var wg sync.WaitGroup
	var readErr error
	var mu sync.Mutex
	captureErr := func(e error) {
		if e != nil {
			mu.Lock()
			if readErr == nil {
				readErr = e
			}
			mu.Unlock()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		captureErr(streamCommandOutput(stdout, macFormatOutputHandler(progress)))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		captureErr(streamCommandOutput(stderr, func(line string) {
			printProgressMessage(line)
		}))
	}()

	waitErr := formatCmd.Wait()
	wg.Wait()

	if readErr != nil {
		return fmt.Errorf("diskutil output error: %v", readErr)
	}
	if waitErr != nil {
		return fmt.Errorf("diskutil failed: %v", waitErr)
	}

	progress.Finish()
	return nil
}

func formatWindows(device, label, clusterSize string) error {
	if err := ensureRemovableDevice(device); err != nil {
		return err
	}
	driveLetter := strings.TrimSuffix(device, ":")

	fmt.Println("Creating FAT32 filesystem...")

	args := []string{driveLetter + ":", "/FS:FAT32", "/V:" + label, "/Q", "/Y"}
	if clusterSize != "" {
		args = append(args, "/A:"+clusterSize)
	}

	cmd := exec.Command("format", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("format stdout: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("format stderr: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("format command failed to start: %v", err)
	}

	progress := NewProgressBar("Format", 100)
	defer progress.Stop()

	var wg sync.WaitGroup
	var readErr error
	var mu sync.Mutex
	captureErr := func(e error) {
		if e != nil {
			mu.Lock()
			if readErr == nil {
				readErr = e
			}
			mu.Unlock()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		captureErr(streamCommandOutput(stdout, windowsFormatOutputHandler(progress)))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		captureErr(streamCommandOutput(stderr, func(line string) {
			printProgressMessage(line)
		}))
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	if readErr != nil {
		return fmt.Errorf("format command output error: %v", readErr)
	}
	if waitErr != nil {
		return fmt.Errorf("format command failed: %v", waitErr)
	}

	progress.Finish()
	return nil
}

func streamCommandOutput(r io.Reader, handle func(string)) error {
	reader := bufio.NewReader(r)
	var buf strings.Builder

	flush := func() {
		if buf.Len() == 0 {
			return
		}
		line := strings.TrimSpace(buf.String())
		buf.Reset()
		if line != "" {
			handle(line)
		}
	}

	for {
		b, err := reader.ReadByte()
		if err != nil {
			flush()
			if err == io.EOF {
				return nil
			}
			return err
		}

		if b == '\n' || b == '\r' {
			flush()
			continue
		}

		buf.WriteByte(b)
	}
}

func printProgressMessage(line string) {
	if line == "" {
		return
	}
	clearWidth := len(line) + 32
	if clearWidth < 80 {
		clearWidth = 80
	}
	fmt.Printf("\r%s\r%s\n", strings.Repeat(" ", clearWidth), line)
}

func macFormatOutputHandler(pb *ProgressBar) func(string) {
	steps := []struct {
		pattern  string
		progress int64
	}{
		{"started erase", 5},
		{"unmounting", 15},
		{"creating the partition map", 35},
		{"waiting for partitions", 55},
		{"formatting", 75},
		{"initialization complete", 90},
		{"finished", 100},
	}

	var last int64

	return func(line string) {
		lower := strings.ToLower(line)
		for _, step := range steps {
			if strings.Contains(lower, step.pattern) {
				if step.progress > last {
					pb.Set(step.progress)
					last = step.progress
				}
				break
			}
		}
		printProgressMessage(line)
	}
}

func windowsFormatOutputHandler(pb *ProgressBar) func(string) {
	percentRegex := regexp.MustCompile(`(?i)(\d{1,3})\s*percent`)
	var last int64

	return func(line string) {
		if matches := percentRegex.FindStringSubmatch(line); matches != nil {
			if value, err := strconv.Atoi(matches[1]); err == nil {
				progress := int64(value)
				if progress > last {
					last = progress
				}
				pb.Set(progress)
			}
			return
		}

		lower := strings.ToLower(line)
		if strings.Contains(lower, "format complete") && last < 100 {
			last = 100
			pb.Set(100)
			return
		}

		printProgressMessage(line)
	}
}
