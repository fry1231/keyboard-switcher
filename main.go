package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/moutend/go-hook/pkg/keyboard"
	"github.com/moutend/go-hook/pkg/types"
	"golang.org/x/sys/windows"
)

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	getKeyboardLayoutList  = user32.NewProc("GetKeyboardLayoutList")
	postMessage            = user32.NewProc("PostMessageW")
	getForegroundWindow    = user32.NewProc("GetForegroundWindow")
	firstPressed           bool
	secondPressed          bool
	firstPressTime         time.Time
	secondPressTime        time.Time
	timeThreshold          = 50 * time.Millisecond
	firstKeyName           string
	secondKeyName          string
)

const (
	WM_INPUTLANGCHANGEREQUEST = 0x0050
	HWND_BROADCAST            = 0xffff
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

func createGUI() (string, string) {
	myApp := app.New()
	window := myApp.NewWindow("Keyboard Layout Switcher")

	// Create radio buttons for first key
	firstKeySelect := widget.NewRadioGroup([]string{"LeftAlt", "Ctrl"}, nil)
	firstKeySelect.SetSelected("LeftAlt")

	// Create radio buttons for second key
	secondKeySelect := widget.NewRadioGroup([]string{"Shift"}, nil)
	secondKeySelect.SetSelected("Shift")

	// Create start button
	startBtn := widget.NewButton("Start", nil)

	// Channel to get the selected values
	resultChan := make(chan struct{})

	var selectedFirst, selectedSecond string

	startBtn.OnTapped = func() {
		selectedFirst = firstKeySelect.Selected
		selectedSecond = secondKeySelect.Selected
		window.Close()
		resultChan <- struct{}{}
	}

	// Layout
	content := container.NewVBox(
		widget.NewLabel("Select first key:"),
		firstKeySelect,
		widget.NewLabel("Select second key:"),
		secondKeySelect,
		startBtn,
	)

	window.SetContent(content)
	window.Resize(fyne.NewSize(300, 200))
	
	// Show window in a goroutine
	go window.ShowAndRun()

	// Wait for selection
	<-resultChan

	return selectedFirst, selectedSecond
}

func main() {
	layouts := getKeyboardLayouts()
	if len(layouts) < 2 {
		fmt.Println("Less than 2 keyboard layouts installed")
		return
	}

	// Get key combination from GUI
	firstKeyName, secondKeyName = createGUI()

	fmt.Printf("Selected combination: %s + %s\n", firstKeyName, secondKeyName)
	fmt.Println("Language switcher started")
	fmt.Println("Press Ctrl+C to exit.")

	keyboardChan := make(chan types.KeyboardEvent)
	if err := keyboard.Install(nil, keyboardChan); err != nil {
		fmt.Printf("Error installing keyboard hook: %v\n", err)
		return
	}
	defer keyboard.Uninstall()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	for {
		select {
		case <-signalChan:
			return
		case k := <-keyboardChan:
			fmt.Printf("Key event - Message: %v, VKC         ode: %v (0x%X)\n", k.Message, k.VKCode, k.VKCode)
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
	fmt.Printf("Current layout: 0x%X\n", currentLayout)
	fmt.Printf("Available layouts: %v\n", layouts)

	// Find next layout
	nextLayout := layouts[0]
	for i, layout := range layouts {
		if layout == currentLayout {
			nextLayout = layouts[(i+1)%len(layouts)]
			break
		}
	}

	fmt.Printf("Switching to layout: 0x%X\n", nextLayout)

	hwnd, _, _ := getForegroundWindow.Call()

	ret, _, err := postMessage.Call(
		hwnd,
		WM_INPUTLANGCHANGEREQUEST,
		0,
		nextLayout,
	)
	fmt.Printf("PostMessage result: %v, error: %v\n", ret, err)
}