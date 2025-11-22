package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type Profile struct {
	Name                string               `json:"name,omitempty"`
	Label               string               `json:"label,omitempty"`
	ClusterSize         string               `json:"cluster_size,omitempty"`
	BenchmarkThresholds *BenchmarkThresholds `json:"benchmark_thresholds,omitempty"`
}

type profileStore struct {
	Profiles map[string]Profile `json:"profiles"`
}

func profileDisplayName(profile Profile, fallback string) string {
	name := strings.TrimSpace(profile.Name)
	if name != "" {
		return name
	}
	return strings.TrimSpace(fallback)
}

func profileMapKey(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("profile name cannot be empty")
	}
	return strings.ToLower(trimmed), nil
}

func mergedBenchmarkThresholds(custom *BenchmarkThresholds) BenchmarkThresholds {
	thresholds := defaultBenchmarkThresholds
	if custom == nil {
		return thresholds
	}
	if custom.ExtremelySlow > 0 {
		thresholds.ExtremelySlow = custom.ExtremelySlow
	}
	if custom.VerySlow > 0 {
		thresholds.VerySlow = custom.VerySlow
	}
	if custom.SlightlySlow > 0 {
		thresholds.SlightlySlow = custom.SlightlySlow
	}
	if custom.Prompt > 0 {
		thresholds.Prompt = custom.Prompt
	}
	return thresholds
}

func validateBenchmarkThresholds(t BenchmarkThresholds) error {
	if t.ExtremelySlow <= 0 || t.VerySlow <= 0 || t.SlightlySlow <= 0 {
		return fmt.Errorf("benchmark thresholds must be greater than zero")
	}
	if t.ExtremelySlow > t.VerySlow {
		return fmt.Errorf("extremely slow threshold must be less than or equal to very slow threshold")
	}
	if t.VerySlow > t.SlightlySlow {
		return fmt.Errorf("very slow threshold must be less than or equal to slightly slow threshold")
	}
	if t.Prompt <= 0 {
		return fmt.Errorf("prompt threshold must be greater than zero")
	}
	return nil
}

func profileConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			if err != nil {
				return "", fmt.Errorf("unable to resolve config directory: %w", err)
			}
			return "", fmt.Errorf("unable to resolve config directory: %w", homeErr)
		}
		configDir = filepath.Join(home, ".cdjf")
	} else {
		configDir = filepath.Join(configDir, "cdjf")
	}
	return filepath.Join(configDir, "profiles.json"), nil
}

func loadProfileStore() (profileStore, error) {
	path, err := profileConfigPath()
	if err != nil {
		return profileStore{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return profileStore{Profiles: make(map[string]Profile)}, nil
		}
		return profileStore{}, err
	}

	var store profileStore
	if err := json.Unmarshal(data, &store); err != nil {
		return profileStore{}, err
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]Profile)
	}
	return store, nil
}

func saveProfileStore(store profileStore) error {
	path, err := profileConfigPath()
	if err != nil {
		return err
	}
	if store.Profiles == nil {
		store.Profiles = make(map[string]Profile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func loadProfileByName(name string) (Profile, error) {
	key, err := profileMapKey(name)
	if err != nil {
		return Profile{}, err
	}
	store, err := loadProfileStore()
	if err != nil {
		return Profile{}, err
	}
	profile, ok := store.Profiles[key]
	if !ok {
		return Profile{}, fmt.Errorf("profile %q not found", strings.TrimSpace(name))
	}
	if strings.TrimSpace(profile.Name) == "" {
		profile.Name = strings.TrimSpace(name)
	}
	return profile, nil
}

func profileSave(cmd *cobra.Command, args []string) {
	name := args[0]
	key, err := profileMapKey(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store, err := loadProfileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
		os.Exit(1)
	}

	profile := store.Profiles[key]
	profile.Name = strings.TrimSpace(name)

	labelChanged := cmd.Flags().Changed("label")
	clusterChanged := cmd.Flags().Changed("cluster-size")
	extChanged := cmd.Flags().Changed("extremely-slow")
	veryChanged := cmd.Flags().Changed("very-slow")
	slightChanged := cmd.Flags().Changed("slightly-slow")
	promptChanged := cmd.Flags().Changed("prompt")
	resetBench, _ := cmd.Flags().GetBool("reset-benchmarks")

	if !labelChanged && !clusterChanged && !extChanged && !veryChanged && !slightChanged && !promptChanged && !resetBench {
		fmt.Fprintln(os.Stderr, "Specify at least one option to save (e.g. --label, --cluster-size, or a threshold flag).")
		os.Exit(1)
	}

	changed := false

	if labelChanged {
		value, _ := cmd.Flags().GetString("label")
		profile.Label = value
		changed = true
	}

	if clusterChanged {
		value, _ := cmd.Flags().GetString("cluster-size")
		normalized, normErr := normalizeClusterSize(value)
		if normErr != nil {
			fmt.Fprintf(os.Stderr, "Invalid cluster size: %v\n", normErr)
			os.Exit(1)
		}
		profile.ClusterSize = normalized
		changed = true
	}

	if resetBench {
		if extChanged || veryChanged || slightChanged || promptChanged {
			fmt.Fprintln(os.Stderr, "Cannot adjust benchmark thresholds while --reset-benchmarks is provided.")
			os.Exit(1)
		}
		if profile.BenchmarkThresholds != nil {
			profile.BenchmarkThresholds = nil
			changed = true
		}
	} else {
		thresholds := mergedBenchmarkThresholds(profile.BenchmarkThresholds)
		thresholdChanged := false

		if extChanged {
			value, _ := cmd.Flags().GetFloat64("extremely-slow")
			if value <= 0 {
				fmt.Fprintln(os.Stderr, "--extremely-slow must be greater than zero.")
				os.Exit(1)
			}
			thresholds.ExtremelySlow = value
			thresholdChanged = true
		}
		if veryChanged {
			value, _ := cmd.Flags().GetFloat64("very-slow")
			if value <= 0 {
				fmt.Fprintln(os.Stderr, "--very-slow must be greater than zero.")
				os.Exit(1)
			}
			thresholds.VerySlow = value
			thresholdChanged = true
		}
		if slightChanged {
			value, _ := cmd.Flags().GetFloat64("slightly-slow")
			if value <= 0 {
				fmt.Fprintln(os.Stderr, "--slightly-slow must be greater than zero.")
				os.Exit(1)
			}
			thresholds.SlightlySlow = value
			thresholdChanged = true
		}
		if promptChanged {
			value, _ := cmd.Flags().GetFloat64("prompt")
			if value <= 0 {
				fmt.Fprintln(os.Stderr, "--prompt must be greater than zero.")
				os.Exit(1)
			}
			thresholds.Prompt = value
			thresholdChanged = true
		}

		if thresholdChanged {
			if err := validateBenchmarkThresholds(thresholds); err != nil {
				fmt.Fprintf(os.Stderr, "Invalid benchmark thresholds: %v\n", err)
				os.Exit(1)
			}
			profile.BenchmarkThresholds = &BenchmarkThresholds{
				ExtremelySlow: thresholds.ExtremelySlow,
				VerySlow:      thresholds.VerySlow,
				SlightlySlow:  thresholds.SlightlySlow,
				Prompt:        thresholds.Prompt,
			}
			changed = true
		}
	}

	if !changed {
		fmt.Println("No changes to save.")
		return
	}

	store.Profiles[key] = profile
	if err := saveProfileStore(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Profile %q saved.\n", profileDisplayName(profile, name))
}

func profileList(cmd *cobra.Command, args []string) {
	store, err := loadProfileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
		os.Exit(1)
	}

	if len(store.Profiles) == 0 {
		fmt.Println("No profiles saved yet.")
		return
	}

	names := make([]string, 0, len(store.Profiles))
	for key, profile := range store.Profiles {
		names = append(names, profileDisplayName(profile, key))
	}
	sort.Strings(names)

	fmt.Println("Saved profiles:")
	for _, name := range names {
		fmt.Printf("  %s\n", name)
	}
}

func profileShow(cmd *cobra.Command, args []string) {
	name := args[0]
	profile, err := loadProfileByName(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	display := profileDisplayName(profile, name)
	fmt.Printf("Profile %q\n", display)

	if strings.TrimSpace(profile.Label) != "" {
		fmt.Printf("Label: %s\n", profile.Label)
	} else {
		fmt.Println("Label: (default)")
	}

	if strings.TrimSpace(profile.ClusterSize) != "" {
		fmt.Printf("Cluster size: %s\n", profile.ClusterSize)
	} else {
		fmt.Println("Cluster size: (default)")
	}

	thresholds := mergedBenchmarkThresholds(profile.BenchmarkThresholds)
	if profile.BenchmarkThresholds == nil {
		fmt.Println("Benchmark thresholds: default")
	} else {
		fmt.Println("Benchmark thresholds:")
	}
	fmt.Printf("  Extremely slow: %.2f MB/s\n", thresholds.ExtremelySlow)
	fmt.Printf("  Very slow: %.2f MB/s\n", thresholds.VerySlow)
	fmt.Printf("  Slightly slow: %.2f MB/s\n", thresholds.SlightlySlow)
	fmt.Printf("  Prompt: %.2f MB/s\n", thresholds.Prompt)
}

func profileDelete(cmd *cobra.Command, args []string) {
	name := args[0]
	key, err := profileMapKey(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	store, err := loadProfileStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading profiles: %v\n", err)
		os.Exit(1)
	}

	profile, exists := store.Profiles[key]
	if !exists {
		fmt.Fprintf(os.Stderr, "Profile %q not found.\n", strings.TrimSpace(name))
		os.Exit(1)
	}

	delete(store.Profiles, key)
	if err := saveProfileStore(store); err != nil {
		fmt.Fprintf(os.Stderr, "Error deleting profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Profile %q deleted.\n", profileDisplayName(profile, name))
}
