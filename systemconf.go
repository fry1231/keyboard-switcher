package main

import (
	"os"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

const (
	KEYBOARD_LAYOUTS_KEY = `Keyboard Layout\Toggle`
	LANGUAGE_HOTKEY_CURRENT_VALUE = "Hotkey"
	HOTKEY_CURRENT_VALUE = "Language Hotkey"
)


func isStartedFromAutostart() bool {
	args := os.Args
	if len(args) > 1 {
		return args[1] == "--autostart"
	}
	return false
}

func disableSystemKeyboardShortcuts() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, KEYBOARD_LAYOUTS_KEY, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer key.Close()
	
	err = key.SetStringValue(HOTKEY_CURRENT_VALUE, "3")
	err2 := key.SetStringValue(LANGUAGE_HOTKEY_CURRENT_VALUE, "3")
	if err != nil {
		return err
	}
	if err2 != nil {
		return err2
	}
	return nil
}

func getSystemKeyboardShortcutsStatus() (int8, error) {
	// 1 - Alt, 2 - Ctrl, 3 - Disabled, 4 - ` (Grave accent)
	key, err := registry.OpenKey(registry.CURRENT_USER, KEYBOARD_LAYOUTS_KEY, registry.READ)
	if err != nil {
		return 0, err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(HOTKEY_CURRENT_VALUE)
	if err != nil {
		return 0, err
	}
	
	val, _ := strconv.ParseInt(value, 10, 8)
	return int8(val), nil
}