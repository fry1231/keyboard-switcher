package main

import (
	"fmt"
	"os"
	"path/filepath"
	"golang.org/x/sys/windows/registry"
)

const autorunRegPath = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "LanguageSwitcher"

func toggleAutostart() error {
	command := getAutostartCommand()
	if command == "" {
		return fmt.Errorf("failed to get autostart command")
	}

	// Open registry key
	key, err := registry.OpenKey(registry.CURRENT_USER, autorunRegPath, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %v", err)
	}
	defer key.Close()

	// Check if already in autostart
	currentValue, _, err := key.GetStringValue(appName)
	if err == nil && currentValue == command {
		// Already in autostart, remove it
		err = key.DeleteValue(appName)
		if err != nil {
			return fmt.Errorf("failed to remove from autostart: %v", err)
		}
		return nil
	}

	// Add to autostart
	err = key.SetStringValue(appName, command)
	if err != nil {
		return fmt.Errorf("failed to add to autostart: %v", err)
	}
	return nil
}

func isAutostartEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, autorunRegPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("failed to open registry key: %v", err)
	}
	defer key.Close()

	command := getAutostartCommand()
	if command == "" {
		return false, fmt.Errorf("failed to get autostart command")
	}

	value, _, err := key.GetStringValue(appName)
	if err != nil {
		return false, nil // Not in autostart
	}

	return value == command, nil
}

func getAutostartCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exePath, err := filepath.Abs(exe)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`"%s" --autostart`, exePath)
} 