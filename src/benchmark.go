package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type BenchmarkResult struct {
	WriteMBps float64
	ReadMBps  float64
}

type IntegrityResult struct {
	BenchmarkResult
	BytesWritten  int64
	BytesVerified int64
	Errors        []string
}

func (r IntegrityResult) Success() bool {
	return len(r.Errors) == 0
}

type BenchmarkThresholds struct {
	ExtremelySlow float64 `json:"extremely_slow,omitempty"`
	VerySlow      float64 `json:"very_slow,omitempty"`
	SlightlySlow  float64 `json:"slightly_slow,omitempty"`
	Prompt        float64 `json:"prompt,omitempty"`
}

var defaultBenchmarkThresholds = BenchmarkThresholds{
	ExtremelySlow: 2,
	VerySlow:      3,
	SlightlySlow:  6,
	Prompt:        5,
}

func benchmarkSeverity(speed float64, thresholds BenchmarkThresholds) string {
	switch {
	case speed <= 0:
		return "Unable to benchmark drive."
	case thresholds.ExtremelySlow > 0 && speed < thresholds.ExtremelySlow:
		return "WARNING: Drive appears to be extremely slow."
	case thresholds.VerySlow > 0 && speed < thresholds.VerySlow:
		return "WARNING: Drive appears to be very slow."
	case thresholds.SlightlySlow > 0 && speed < thresholds.SlightlySlow:
		return "WARNING: Drive appears to be slightly slow."
	default:
		return "Performance is OK."
	}
}

func benchmarkSummary(result BenchmarkResult, thresholds BenchmarkThresholds) string {
	if result.WriteMBps <= 0 && result.ReadMBps <= 0 {
		return benchmarkSeverity(result.WriteMBps, thresholds)
	}

	lines := []string{benchmarkSeverity(result.WriteMBps, thresholds)}

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

func benchmarkDrive(device string) BenchmarkResult {
	testFile, _, err := resolveTestFilePath(device, "cdjf_benchmark_test.tmp")
	if err != nil {
		return BenchmarkResult{}
	}
	return runIOMeasure(testFile)
}

func runIOMeasure(testFile string) BenchmarkResult {
	const (
		mib               = int64(1024 * 1024)
		chunkSize         = 4 * mib
		minSampleDuration = 400 * time.Millisecond
		initialSampleSize = 32 * mib
		maxSampleSize     = 256 * mib
	)

	result := BenchmarkResult{}
	chunk := make([]byte, chunkSize)

	_ = os.Remove(testFile)
	file, err := os.Create(testFile)
	if err != nil {
		return result
	}
	defer os.Remove(testFile)

	currentSampleTarget := initialSampleSize
	fmt.Printf("  Running write benchmark (minimum %.0f MB sample)...\n", float64(initialSampleSize)/float64(mib))
	writeBar := NewProgressBar("Write", currentSampleTarget)
	defer writeBar.Stop()

	writeStart := time.Now()
	var bytesWritten int64
	for {
		remainingTarget := currentSampleTarget - bytesWritten
		if remainingTarget <= 0 {
			break
		}

		toWrite := chunk
		if remainingTarget < int64(len(chunk)) {
			toWrite = chunk[:int(remainingTarget)]
		}

		n, writeErr := file.Write(toWrite)
		if n > 0 {
			bytesWritten += int64(n)
			writeBar.Add(int64(n))
		}
		if writeErr != nil {
			file.Close()
			return result
		}
		if int64(n) != int64(len(toWrite)) {
			file.Close()
			return result
		}

		if bytesWritten >= currentSampleTarget {
			elapsed := time.Since(writeStart)
			if elapsed >= minSampleDuration || currentSampleTarget >= maxSampleSize {
				break
			}

			nextTarget := currentSampleTarget * 2
			if nextTarget > maxSampleSize {
				nextTarget = maxSampleSize
			}
			currentSampleTarget = nextTarget
			writeBar.UpdateTotal(currentSampleTarget)
			fmt.Printf("  Extending write sample to %.0f MB to improve accuracy...\n", float64(currentSampleTarget)/float64(mib))
		}
	}

	if syncErr := file.Sync(); syncErr != nil {
		file.Close()
		return result
	}

	if closeErr := file.Close(); closeErr != nil {
		return result
	}

	writeDuration := time.Since(writeStart)
	if writeDuration > 0 && bytesWritten > 0 {
		result.WriteMBps = float64(bytesWritten) / writeDuration.Seconds() / (1024 * 1024)
	}
	writeBar.Finish()

	readFile, err := os.Open(testFile)
	if err != nil {
		return result
	}
	defer readFile.Close()

	fmt.Println("  Running read benchmark...")
	readBar := NewProgressBar("Read", bytesWritten)
	defer readBar.Stop()

	readStart := time.Now()
	var totalRead int64
	for {
		n, readErr := readFile.Read(chunk)
		if n > 0 {
			totalRead += int64(n)
			readBar.Add(int64(n))
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

	readDuration := time.Since(readStart)
	if readDuration > 0 && totalRead > 0 {
		result.ReadMBps = float64(totalRead) / readDuration.Seconds() / (1024 * 1024)
	}
	readBar.Finish()

	if writeDuration < minSampleDuration {
		fmt.Println("  Write benchmark completed very quickly even at the maximum payload; reported write speed may understate sustained performance.")
	}
	if readDuration < minSampleDuration {
		fmt.Println("  Read benchmark completed very quickly; reported read speed may benefit from OS caching.")
	}

	return result
}

func runIntegrityCheck(testFile string, testSize int64) IntegrityResult {
	const chunkSize = 1024 * 1024

	result := IntegrityResult{}
	chunk := make([]byte, chunkSize)
	expected := make([]byte, chunkSize)

	file, err := os.Create(testFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("create test file: %v", err))
		return result
	}
	defer os.Remove(testFile)

	writeBar := NewProgressBar("Write", testSize)
	defer writeBar.Stop()

	var bytesWritten int64
	writeStart := time.Now()
	for bytesWritten < testSize {
		remaining := testSize - bytesWritten
		toWrite := chunkSize
		if remaining < int64(toWrite) {
			toWrite = int(remaining)
		}

		fillPattern(chunk[:toWrite], bytesWritten)
		n, writeErr := file.Write(chunk[:toWrite])
		offset := bytesWritten
		if n > 0 {
			writeBar.Add(int64(n))
			bytesWritten += int64(n)
		}
		if writeErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write at offset %d: %v", offset, writeErr))
			file.Close()
			result.BytesWritten = bytesWritten
			return result
		}
		if n != toWrite {
			result.Errors = append(result.Errors, fmt.Sprintf("short write at offset %d (expected %d wrote %d)", offset, toWrite, n))
			file.Close()
			result.BytesWritten = bytesWritten
			return result
		}
	}

	if syncErr := file.Sync(); syncErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("sync: %v", syncErr))
		file.Close()
		result.BytesWritten = bytesWritten
		return result
	}

	if closeErr := file.Close(); closeErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("close after write: %v", closeErr))
		result.BytesWritten = bytesWritten
		return result
	}

	writeElapsed := time.Since(writeStart).Seconds()
	if writeElapsed > 0 && bytesWritten > 0 {
		result.WriteMBps = float64(bytesWritten) / writeElapsed / (1024 * 1024)
	}
	result.BytesWritten = bytesWritten
	writeBar.Finish()

	readFile, err := os.Open(testFile)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("reopen for read: %v", err))
		return result
	}
	defer readFile.Close()

	verifyBar := NewProgressBar("Verify", bytesWritten)
	defer verifyBar.Stop()

	var bytesVerified int64
	readStart := time.Now()
	for {
		n, readErr := readFile.Read(chunk)
		if n > 0 {
			fillPattern(expected[:n], bytesVerified)
			if !bytes.Equal(chunk[:n], expected[:n]) {
				result.Errors = append(result.Errors, fmt.Sprintf("data mismatch at offset %d", bytesVerified))
				bytesVerified += int64(n)
				verifyBar.Add(int64(n))
				break
			}
			bytesVerified += int64(n)
			verifyBar.Add(int64(n))
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			result.Errors = append(result.Errors, fmt.Sprintf("read error after %d bytes: %v", bytesVerified, readErr))
			break
		}
	}

	readElapsed := time.Since(readStart).Seconds()
	if readElapsed > 0 && bytesVerified > 0 {
		result.ReadMBps = float64(bytesVerified) / readElapsed / (1024 * 1024)
	}
	result.BytesVerified = bytesVerified
	verifyBar.Finish()

	return result
}

func fillPattern(buf []byte, offset int64) {
	for i := range buf {
		buf[i] = byte((offset + int64(i)) & 0xFF)
	}
}

func writeVerifyLog(device, mountPoint string, testSize int64, result IntegrityResult) (string, error) {
	timestamp := time.Now()
	fileName := fmt.Sprintf("cdjf-verify-%s-%s.log", sanitizeDeviceName(device), timestamp.Format("20060102-150405"))

	file, err := os.Create(fileName)
	if err != nil {
		return "", err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	fmt.Fprintf(writer, "CDJF Integrity Verification Report\n")
	fmt.Fprintf(writer, "Timestamp: %s\n", timestamp.Format(time.RFC3339))
	fmt.Fprintf(writer, "Device: %s\n", device)
	fmt.Fprintf(writer, "Mount point: %s\n", mountPoint)
	fmt.Fprintf(writer, "Test size: %.1f MB\n", float64(testSize)/(1024*1024))
	fmt.Fprintf(writer, "Bytes written: %.1f MB\n", float64(result.BytesWritten)/(1024*1024))
	fmt.Fprintf(writer, "Bytes verified: %.1f MB\n", float64(result.BytesVerified)/(1024*1024))
	fmt.Fprintf(writer, "Write speed: %.2f MB/s\n", result.WriteMBps)
	fmt.Fprintf(writer, "Read speed: %.2f MB/s\n", result.ReadMBps)
	if result.Success() {
		fmt.Fprintln(writer, "Status: PASS - No integrity issues detected.")
	} else {
		fmt.Fprintln(writer, "Status: FAIL")
		for _, errMsg := range result.Errors {
			fmt.Fprintf(writer, "Error: %s\n", errMsg)
		}
	}

	if err := writer.Flush(); err != nil {
		return "", err
	}

	return fileName, nil
}

func sanitizeDeviceName(device string) string {
	replacer := strings.NewReplacer(
		":", "",
		"/", "_",
		"\\", "_",
		" ", "_",
	)
	cleaned := strings.Trim(replacer.Replace(strings.TrimSpace(device)), "_")
	if cleaned == "" {
		return "drive"
	}
	return cleaned
}
