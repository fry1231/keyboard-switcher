package main

import (
	"fmt"
	"os"
	"time"
	"encoding/json"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	getKeyboardLayoutList  = user32.NewProc("GetKeyboardLayoutList")
	postMessage            = user32.NewProc("PostMessageW")
	getForegroundWindow    = user32.NewProc("GetForegroundWindow")
	createMutex           = kernel32.NewProc("CreateMutexW")
	firstPressed           bool
	secondPressed          bool
	firstPressTime         time.Time
	secondPressTime        time.Time
	timeThreshold          = 50 * time.Millisecond
	firstKeyName           string
	secondKeyName          string
	configChan            = make(chan Config)
)

const (
	LANG_TOGGLE               = 0x0001
	VK_MENU                   = 0x12
	VK_LMENU                  = 0xA4
	VK_RMENU                  = 0xA5
	VK_SHIFT                  = 0x10
	VK_LSHIFT                 = 0xA0
	VK_RSHIFT                 = 0xA1
	VK_CONTROL                = 0x11
	VK_LCONTROL               = 0xA2
	VK_RCONTROL               = 0xA3

	SIZE_W = 322
    SIZE_H = 228
)

func showError(err error) {
	walk.MsgBox(nil, "Error", err.Error(), walk.MsgBoxIconError)
}


type Config struct {
	SwitchOnAlt bool
}

func (c *Config) SaveToFile() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	
	return os.WriteFile(configPath, data, 0644)
}

func LoadConfig() (*Config, error) {
	execPath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(filepath.Dir(execPath), "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &Config{true}, nil
		}
		return nil, err
	}
	
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	return &config, nil
}


type MyMainWindow struct {
	*walk.MainWindow
	notifyIcon *walk.NotifyIcon
}

func main() {
	// Ensure there is only one instance of the program
	// Create a named mutex
	mutexName, err := syscall.UTF16PtrFromString("Global\\WinLangSwitcherMutex")
	if err != nil {
		showError(fmt.Errorf("Error creating mutex name: %v", err))
		return
	}

	handle, _, errNo := createMutex.Call(0, 1, uintptr(unsafe.Pointer(mutexName)))
	if handle == 0 {
		showError(fmt.Errorf("Failed to create mutex"))
		return
	}
	defer windows.CloseHandle(windows.Handle(handle))

	// Check if the mutex already existed
	if errNo == syscall.ERROR_ALREADY_EXISTS {
		showError(fmt.Errorf("Another instance is already running"))
		return
	}

	icon, err := walk.Resources.Icon("APP")
	if err != nil {
		showError(err)
		return
	}

	currentConfig, err := LoadConfig()
	if err != nil {
		execPath, _ := os.Executable()
		workDir, _ := os.Getwd()
		showError(fmt.Errorf("Error loading config: %v\nExecutable path: %s\nWorking directory: %s\nUsing default configuration", 
			err, execPath, workDir))
		currentConfig = &Config{true}
	}

	go watcherTask(currentConfig)

	var autostartButton *walk.PushButton
	var shortcutsButton *walk.PushButton
	var shortcutLabel *walk.TextLabel

	updateAutostartButtonText := func() {
		enabled, err := isAutostartEnabled()
		if err != nil {
			fmt.Printf("Error checking autostart status: %v\n", err)
			return
		}
		if enabled {
			autostartButton.SetText("Remove from Autostart")
		} else {
			autostartButton.SetText("Add to Autostart")
		}
	}

	updateShortcutLabel := func() {
		keyIndex, err := getSystemKeyboardShortcutsStatus()
		if err != nil {
			fmt.Printf("Error getting system keyboard shortcuts status: %v\n", err)
			shortcutLabel.SetText("System shortcut: Unknown")
			return
		}
		if keyIndex == 1 {
			shortcutLabel.SetText("System shortcut: LeftAlt+Shift")
		} else if keyIndex == 2 {
			shortcutLabel.SetText("System shortcut: Ctrl+Shift")
		} else if keyIndex == 3 {
			shortcutLabel.SetText("System shortcut: Disabled")
		} else if keyIndex == 4 {
			shortcutLabel.SetText("System shortcut: ` (Grave accent)")
		}
	}

	isAutostart := isStartedFromAutostart()

	mw := new(MyMainWindow)

	MainWindow{
		Title:   "WIN Language Switcher",
		MinSize: Size{SIZE_W, SIZE_H},
		Size:    Size{SIZE_W, SIZE_H},
		MaxSize: Size{SIZE_W, SIZE_H},
		Layout:  VBox{},
		AssignTo: &mw.MainWindow,
		Icon:     icon,
		DataBinder: DataBinder{
			DataSource: currentConfig,
			AutoSubmit: true,
			OnSubmitted: func() {
				configChan <- *currentConfig
				if err := currentConfig.SaveToFile(); err != nil {
					showError(fmt.Errorf("Error saving config: %v", err))
				}
			},
		},
		Children: []Widget{
			RadioButtonGroup{
				DataMember: "SwitchOnAlt",
				Buttons: []RadioButton{
					RadioButton{
						Name:  "aRB",
						Text:  "LeftAlt+Shift",
						Value: true,
					},
					RadioButton{
						Name:  "bRB",
						Text:  "Ctrl+Shift",
						Value: false,
					},
				},
			},
			TextLabel{
				AssignTo: &shortcutLabel,
				Text:     "System shortcut: Alt+Shift",
			},
			PushButton{
				AssignTo: &shortcutsButton,
				Text:     "Disable System Shortcuts",
				MinSize:  Size{200, 30},
				MaxSize:  Size{200, 30},
				OnClicked: func() {
					err := disableSystemKeyboardShortcuts()
					if err != nil {
						walk.MsgBox(nil, "Error", 
							fmt.Sprintf("Failed to toggle system shortcuts: %v", err),
							walk.MsgBoxIconError)
						return
					}
					updateShortcutLabel()
				},
			},
			PushButton{
				AssignTo: &autostartButton,
				Text:     "Add to Autostart",
				MinSize:  Size{200, 30},
				MaxSize:  Size{200, 30},
				OnClicked: func() {
					err := toggleAutostart()
					if err != nil {
						walk.MsgBox(nil, "Error", 
							fmt.Sprintf("Failed to toggle autostart: %v", err),
							walk.MsgBoxIconError)
						return
					}
					updateAutostartButtonText()
				},
			},
			PushButton{
				Text: "Minimize to tray",
				MinSize: Size{200, 30},
				MaxSize: Size{200, 30},
				OnClicked: func() {
					mw.Hide()
				},
			},
		},
	}.Create()
	
	// Make window unresizable
    defaultStyle := win.GetWindowLong(mw.Handle(), win.GWL_STYLE) // Gets current style
    newStyle := defaultStyle &^ win.WS_THICKFRAME                 // Remove WS_THICKFRAME
    win.SetWindowLong(mw.Handle(), win.GWL_STYLE, newStyle)

    xScreen := win.GetSystemMetrics(win.SM_CXSCREEN);
    yScreen := win.GetSystemMetrics(win.SM_CYSCREEN);
    win.SetWindowPos(
        mw.Handle(),
        0,
        (xScreen - SIZE_W)/2,
        (yScreen - SIZE_H)/2,
        SIZE_W,
        SIZE_H,
        win.SWP_FRAMECHANGED,
    )
    win.ShowWindow(mw.Handle(), win.SW_SHOW);

	updateAutostartButtonText()
	updateShortcutLabel()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		showError(err)
		return
	}
	defer ni.Dispose()

	ni.SetIcon(icon)
	ni.SetVisible(true)

	ni.MouseUp().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			mw.Show()
			mw.SetFocus()
		}
	})

	if isAutostart {
		mw.Hide()
	}

	mw.Run()
}