package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/tomasz-mizak/wpe-webkit-kiosk/cmd/kiosk/internal/config"

	"github.com/spf13/cobra"
)

const apiServiceName = "wpe-webkit-kiosk-api"

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Manage REST API service",
}

var apiStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show API service status",
	RunE: func(cmd *cobra.Command, args []string) error {
		state := "unknown"
		if out, err := exec.Command("systemctl", "show", apiServiceName,
			"--property=ActiveState", "--value").Output(); err == nil {
			state = strings.TrimSpace(string(out))
		}

		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return err
		}

		port := cfg.Get("API_PORT")
		if port == "" {
			port = "8100"
		}

		reachable := "no"
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 2*time.Second)
		if err == nil {
			conn.Close()
			reachable = "yes"
		}

		fmt.Printf("Service:    %s\n", state)
		fmt.Printf("Port:       %s\n", port)
		fmt.Printf("Reachable:  %s\n", reachable)
		return nil
	},
}

var apiTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage API authentication token",
}

var apiTokenShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return err
		}
		token := cfg.Get("API_TOKEN")
		if token == "" {
			fmt.Println("API_TOKEN is not configured. Run: kiosk api token regenerate")
			return nil
		}
		fmt.Println(token)
		return nil
	},
}

var apiTokenRegenerateCmd = &cobra.Command{
	Use:   "regenerate",
	Short: "Generate a new API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := generateToken()
		if err != nil {
			return fmt.Errorf("failed to generate token: %w", err)
		}

		cfg, err := config.Load(config.DefaultPath)
		if err != nil {
			return err
		}

		cfg.Set("API_TOKEN", token)
		if err := cfg.Save(); err != nil {
			return err
		}

		fmt.Printf("New token: %s\n", token)

		if err := exec.Command("sudo", "systemctl", "restart", apiServiceName).Run(); err != nil {
			fmt.Println("Token saved. API service restart failed — restart manually: sudo systemctl restart " + apiServiceName)
		} else {
			fmt.Println("API service restarted.")
		}

		return nil
	},
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func init() {
	apiTokenCmd.AddCommand(apiTokenShowCmd)
	apiTokenCmd.AddCommand(apiTokenRegenerateCmd)
	apiCmd.AddCommand(apiStatusCmd)
	apiCmd.AddCommand(apiTokenCmd)
	rootCmd.AddCommand(apiCmd)
}
