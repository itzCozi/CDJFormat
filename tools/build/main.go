// Build helper for cdjf (macOS & Windows only).
// Usage:
//
//	go run ./tools/build              # stripped build
//	go run ./tools/build -verbose     # unstripped
//
// Cross-compiling between macOS and Windows may require extra toolchains.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	verbose := flag.Bool("verbose", false, "build without -s -w")
	flag.Parse()

	goos := envOr("GOOS", runtime.GOOS)
	goarch := envOr("GOARCH", runtime.GOARCH)
	out := "cdjf"
	if goos == "windows" {
		out += ".exe"
	}

	projectRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to determine working directory: %v\n", err)
		os.Exit(1)
	}

	packagePath := filepath.Join(projectRoot, "src")
	if _, statErr := os.Stat(packagePath); statErr != nil {
		fmt.Fprintf(os.Stderr, "Package path %s not found: %v\n", packagePath, statErr)
		os.Exit(1)
	}

	fmt.Printf("Building cdjf for %s/%s -> %s\n", goos, goarch, out)
	args := []string{"build"}
	if !*verbose {
		args = append(args, "-trimpath", "-ldflags", "-s -w")
	}
	args = append(args, "-o", out, "./src")

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Build failed: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Output file missing after build\n")
		os.Exit(2)
	}
	fmt.Printf("Build succeeded. Output: %s (%d bytes)\n", out, info.Size())
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
