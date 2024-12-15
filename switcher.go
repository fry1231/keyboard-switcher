package main


import (
	"unsafe"
	"golang.org/x/sys/windows"
	"fmt"
	"os"
	"time"
	"os/signal"
	
	"github.com/moutend/go-hook/pkg/keyboard"
	"github.com/moutend/go-hook/pkg/types"
)

const (
	WM_INPUTLANGCHANGEREQUEST = 0x0050
)


func keyMatch(vkCode uint32, keyName string) bool {
	if keyName == "LeftAlt" {
		return vkCode == VK_LMENU
	}
	if keyName == "Shift" {
		return vkCode == VK_SHIFT || vkCode == VK_LSHIFT || vkCode == VK_RSHIFT
	}
	if keyName == "Ctrl" {
		return vkCode == VK_CONTROL || vkCode == VK_LCONTROL || vkCode == VK_RCONTROL
	}
	return false
}

func getCurrentKeyboardLayout() uintptr {
	hwnd, _, _ := getForegroundWindow.Call()
	threadID, _, _ := user32.NewProc("GetWindowThreadProcessId").Call(hwnd, 0)
	layout, _, _ := user32.NewProc("GetKeyboardLayout").Call(threadID)
	return layout
}

func getKeyboardLayouts() []uintptr {
	count, _, _ := getKeyboardLayoutList.Call(0, 0)
	if count == 0 {
		return nil
	}

	layouts := make([]windows.Handle, count)
	getKeyboardLayoutList.Call(
		uintptr(count),
		uintptr(unsafe.Pointer(&layouts[0])),
	)

	// Convert to uintptr slice
	result := make([]uintptr, count)
	for i, layout := range layouts {
		result[i] = uintptr(layout)
	}

	return result
}

func switchLanguage() {
	layouts := getKeyboardLayouts()
	currentLayout := getCurrentKeyboardLayout()

	// Find next layout
	nextLayout := layouts[0]
	for i, layout := range layouts {
		if layout == currentLayout {
			nextLayout = layouts[(i+1)%len(layouts)]
			break
		}
	}

	hwnd, _, _ := getForegroundWindow.Call()

	postMessage.Call(
		hwnd,
		WM_INPUTLANGCHANGEREQUEST,
		0,
		nextLayout,
	)
}


func watcherTask(initialConfig *Config) {
	layouts := getKeyboardLayouts()
	if len(layouts) < 2 {
		showError(fmt.Errorf("Less than 2 keyboard layouts installed"))
		return
	}

	currentConfig := initialConfig
	updateKeyNames := func(config *Config) {
		if config.SwitchOnAlt {
				firstKeyName = "LeftAlt"
				secondKeyName = "Shift"
			} else {
				firstKeyName = "Ctrl"
				secondKeyName = "Shift"
			}
	}
	
	updateKeyNames(currentConfig)

	keyboardChan := make(chan types.KeyboardEvent)
	if err := keyboard.Install(nil, keyboardChan); err != nil {
		showError(fmt.Errorf("Error installing keyboard hook: %v", err))
		return
	}
	defer keyboard.Uninstall()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for {
		select {
		case <-signalChan:
			return
		case newConfig := <-configChan:
			currentConfig = &newConfig
			updateKeyNames(currentConfig)
		case k := <-keyboardChan:
			isKeyDown := (k.Message == types.WM_KEYDOWN) || (k.Message == types.WM_SYSKEYDOWN)
			isKeyUp := (k.Message == types.WM_KEYUP) || (k.Message == types.WM_SYSKEYUP)
			vkCode := uint32(k.VKCode)

			if isKeyDown {
				if keyMatch(vkCode, firstKeyName) {
					firstPressed = true
					firstPressTime = time.Now()
					if secondPressed || time.Since(secondPressTime) <= timeThreshold {
						switchLanguage()
					}
				} else if keyMatch(vkCode, secondKeyName) {
					secondPressed = true 
					secondPressTime = time.Now()
					if firstPressed || time.Since(firstPressTime) <= timeThreshold {
						switchLanguage()
					}
				}
			} else if isKeyUp {
				if keyMatch(vkCode, firstKeyName) {
					firstPressed = false
				} else if keyMatch(vkCode, secondKeyName) {
					secondPressed = false
				}
			}
		}
	}
}