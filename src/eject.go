package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

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
