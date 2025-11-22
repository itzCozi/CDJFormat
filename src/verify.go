package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func verifyDrive(cmd *cobra.Command, args []string) {
	sizeMB, _ := cmd.Flags().GetInt("size")
	if sizeMB <= 0 {
		fmt.Fprintln(os.Stderr, "Integrity test size must be greater than zero.")
		os.Exit(1)
	}

	testSize := int64(sizeMB) * 1024 * 1024
	fmt.Println("Starting integrity verification. This may take a few minutes per drive depending on speed.")

	failed := false
	for _, device := range args {
		fmt.Printf("\n[%s] Preparing verification...\n", device)

		if err := validateDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Error: %v\n", device, err)
			failed = true
			continue
		}

		if err := ensureRemovableDevice(device); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Error: %v\n", device, err)
			failed = true
			continue
		}

		testFile, mountPoint, err := resolveTestFilePath(device, "cdjf_verify_test.tmp")
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Error: %v\n", device, err)
			failed = true
			continue
		}

		fmt.Printf("[%s] Mount point: %s\n", device, mountPoint)
		fmt.Printf("[%s] Writing %.1f MB test pattern...\n", device, float64(testSize)/(1024*1024))

		result := runIntegrityCheck(testFile, testSize)

		fmt.Printf("[%s] Write speed: %.2f MB/s\n", device, result.WriteMBps)
		fmt.Printf("[%s] Read speed: %.2f MB/s\n", device, result.ReadMBps)

		if result.Success() {
			fmt.Printf("[%s] Integrity check PASSED (%.1f MB verified).\n", device, float64(result.BytesVerified)/(1024*1024))
		} else {
			fmt.Printf("[%s] Integrity check FAILED after %.1f MB.\n", device, float64(result.BytesVerified)/(1024*1024))
			for _, errMsg := range result.Errors {
				fmt.Printf("    %s\n", errMsg)
			}
			failed = true
		}

		logPath, logErr := writeVerifyLog(device, mountPoint, testSize, result)
		if logErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: unable to write verification log: %v\n", device, logErr)
		} else {
			fmt.Printf("[%s] Detailed log saved to %s\n", device, logPath)
		}
	}

	if failed {
		os.Exit(1)
	}
}
