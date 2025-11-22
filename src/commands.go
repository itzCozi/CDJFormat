package main

import "github.com/spf13/cobra"

var version = "0.1.0"

var rootCmd = &cobra.Command{
	Use:   "CDJF",
	Short: "CDJF - Prepare USB drives for rekordbox (macOS & Windows)",
	Long: `CDJF is a command line tool designed to help DJs prepare USB drives
for use on standalone systems with rekordbox.

It formats drives to FAT32 with optimal settings for rekordbox compatibility on macOS and Windows.`,
	Version: version,
}

var formatCmd = &cobra.Command{
	Use:   "format [device...]",
	Short: "Format a USB drive for rekordbox",
	Long: `Format one or more USB drives to FAT32 with settings optimized for rekordbox.

WARNING: This will erase all data on the selected drive(s)!

Examples:
	cdjf format disk2          (macOS - single drive)
	cdjf format E:             (Windows - single drive)
	cdjf format F: G: H:       (Windows - multiple drives)`,
	Args: cobra.MinimumNArgs(0),
	Run:  formatDrive,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available drives",
	Long:  `List all available drives that can be formatted for rekordbox.`,
	Run:   listDrives,
}

var ejectCmd = &cobra.Command{
	Use:   "eject [device]",
	Short: "Eject a drive",
	Long: `Safely eject a drive from the system.

Examples:
	cdjf eject disk2       (macOS)
	cdjf eject E:          (Windows)`,
	Args: cobra.ExactArgs(1),
	Run:  ejectDrive,
}

var infoCmd = &cobra.Command{
	Use:   "info [device]",
	Short: "Show drive information",
	Long: `Display detailed information about a drive including capacity, free space, filesystem type, and performance statistics.

Examples:
	cdjf info disk2       (macOS)
	cdjf info E:          (Windows)`,
	Args: cobra.ExactArgs(1),
	Run:  showDriveInfo,
}

var verifyCmd = &cobra.Command{
	Use:   "verify [device...]",
	Short: "Run read/write integrity checks on a drive",
	Long: `Verify the health of one or more drives by writing and reading a test pattern.

Run this after formatting to confirm the drive is ready for loading music.

Examples:
	cdjf verify disk2       (macOS)
	cdjf verify E:          (Windows)
	cdjf verify F: G:       (Windows - multiple drives)`,
	Args: cobra.MinimumNArgs(1),
	Run:  verifyDrive,
}

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage CDJF format profiles",
	Long:  "Create, update, view, and delete reusable formatting profiles.",
}

var profileSaveCmd = &cobra.Command{
	Use:   "save [name]",
	Short: "Create or update a profile",
	Args:  cobra.ExactArgs(1),
	Run:   profileSave,
}

var profileListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved profiles",
	Args:  cobra.NoArgs,
	Run:   profileList,
}

var profileShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Args:  cobra.ExactArgs(1),
	Run:   profileShow,
}

var profileDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a saved profile",
	Args:  cobra.ExactArgs(1),
	Run:   profileDelete,
}

func init() {
	rootCmd.AddCommand(formatCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(ejectCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(profileCmd)

	profileCmd.AddCommand(profileSaveCmd)
	profileCmd.AddCommand(profileListCmd)
	profileCmd.AddCommand(profileShowCmd)
	profileCmd.AddCommand(profileDeleteCmd)

	formatCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")
	formatCmd.Flags().StringP("label", "l", "REKORDBOX", "Volume label for the drive")
	formatCmd.Flags().String("profile", "", "Apply settings from a saved profile")
	formatCmd.Flags().String("cluster-size", "", "Cluster size to use when formatting (Windows only, e.g. 32K)")
	verifyCmd.Flags().IntP("size", "s", 64, "Size of the integrity test file in megabytes")

	profileSaveCmd.Flags().String("label", "", "Set the default volume label")
	profileSaveCmd.Flags().String("cluster-size", "", "Set the cluster size (Windows only, e.g. 32K)")
	profileSaveCmd.Flags().Float64("extremely-slow", 0, "Threshold under which drives are classified as extremely slow (MB/s)")
	profileSaveCmd.Flags().Float64("very-slow", 0, "Threshold under which drives are classified as very slow (MB/s)")
	profileSaveCmd.Flags().Float64("slightly-slow", 0, "Threshold under which drives are classified as slightly slow (MB/s)")
	profileSaveCmd.Flags().Float64("prompt", 0, "Threshold under which the formatter will prompt before continuing (MB/s)")
	profileSaveCmd.Flags().Bool("reset-benchmarks", false, "Reset benchmark thresholds to defaults")
}
