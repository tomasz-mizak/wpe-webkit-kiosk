package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"

	"github.com/spf13/cobra"
)

type extensionManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type extensionInfo struct {
	DirName  string
	Name     string
	Version  string
	Enabled  bool
}

func getExtensionsDir() string {
	cfg, err := config.Load(config.DefaultPath)
	if err == nil {
		if dir := cfg.Get("EXTENSIONS_DIR"); dir != "" {
			return dir
		}
	}
	return config.DefaultExtensionsDir
}

func listExtensions() ([]extensionInfo, error) {
	dir := getExtensionsDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read extensions directory: %w", err)
	}

	var exts []extensionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		extDir := filepath.Join(dir, entry.Name())
		manifestPath := filepath.Join(extDir, "manifest.json")

		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var m extensionManifest
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}

		if m.Name == "" || m.Version == "" {
			continue
		}

		disabledPath := filepath.Join(extDir, ".disabled")
		_, disErr := os.Stat(disabledPath)
		enabled := os.IsNotExist(disErr)

		exts = append(exts, extensionInfo{
			DirName: entry.Name(),
			Name:    m.Name,
			Version: m.Version,
			Enabled: enabled,
		})
	}

	return exts, nil
}

func findExtension(name string, exts []extensionInfo) (*extensionInfo, error) {
	for i := range exts {
		if exts[i].DirName == name || exts[i].Name == name {
			return &exts[i], nil
		}
	}

	var names []string
	for _, e := range exts {
		names = append(names, e.DirName)
	}
	return nil, fmt.Errorf("extension %q not found (available: %v)", name, names)
}

var extensionCmd = &cobra.Command{
	Use:   "extension",
	Short: "Manage kiosk extensions",
}

var extensionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all extensions",
	RunE: func(cmd *cobra.Command, args []string) error {
		exts, err := listExtensions()
		if err != nil {
			return err
		}

		if len(exts) == 0 {
			fmt.Println("No extensions found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tSTATUS")
		for _, e := range exts {
			status := "disabled"
			if e.Enabled {
				status = "enabled"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.DirName, e.Version, status)
		}
		return w.Flush()
	},
}

var extensionEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an extension",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exts, err := listExtensions()
		if err != nil {
			return err
		}

		ext, err := findExtension(args[0], exts)
		if err != nil {
			return err
		}

		if ext.Enabled {
			fmt.Printf("Extension %q is already enabled.\n", ext.DirName)
			return nil
		}

		dir := getExtensionsDir()
		disabledPath := filepath.Join(dir, ext.DirName, ".disabled")
		if err := os.Remove(disabledPath); err != nil {
			return fmt.Errorf("cannot enable extension: %w", err)
		}

		fmt.Printf("Extension %q enabled.\n", ext.DirName)
		fmt.Println("Restart the kiosk to apply: kiosk restart")
		return nil
	},
}

var extensionDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an extension",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		exts, err := listExtensions()
		if err != nil {
			return err
		}

		ext, err := findExtension(args[0], exts)
		if err != nil {
			return err
		}

		if !ext.Enabled {
			fmt.Printf("Extension %q is already disabled.\n", ext.DirName)
			return nil
		}

		dir := getExtensionsDir()
		disabledPath := filepath.Join(dir, ext.DirName, ".disabled")
		f, err := os.Create(disabledPath)
		if err != nil {
			return fmt.Errorf("cannot disable extension: %w", err)
		}
		f.Close()

		fmt.Printf("Extension %q disabled.\n", ext.DirName)
		fmt.Println("Restart the kiosk to apply: kiosk restart")
		return nil
	},
}

func init() {
	extensionCmd.AddCommand(extensionListCmd)
	extensionCmd.AddCommand(extensionEnableCmd)
	extensionCmd.AddCommand(extensionDisableCmd)
	rootCmd.AddCommand(extensionCmd)
}
