/*
Package systray is a cross-platform Go library to place an icon and menu in the notification area.
*/
package systray

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

var (
	systrayReady = func() {}
	systrayExit  = func() {}
	menuItems    sync.Map // map[uint32]*MenuItem

	currentID = uint32(0)
	quitOnce  sync.Once
)

func init() {
	runtime.LockOSThread()
}

// MenuItem is used to keep track each menu item of systray.
// Don't create it directly, use the one systray.AddMenuItem() returned
type MenuItem struct {
	// onClicked is the callback function which will be called when the menu item is clicked
	onClicked func()

	// id uniquely identify a menu item, not supposed to be modified
	id uint32
	// title is the text shown on menu item
	title string
	// tooltip is the text shown when pointing to menu item
	tooltip string
	// disabled menu item is grayed out and has no effect when clicked
	disabled bool
	// checked menu item has a tick before the title
	checked bool
	// has the menu item a checkbox (Linux)
	isCheckable bool
	// parent item, for sub menus
	parent *MenuItem
}

func (item *MenuItem) String() string {
	if item.parent == nil {
		return fmt.Sprintf("MenuItem[%d, %q]", item.id, item.title)
	}
	return fmt.Sprintf("MenuItem[%d, parent %d, %q]", item.id, item.parent.id, item.title)
}

// Run initializes GUI and starts the event loop, then invokes the onReady
// callback. It blocks until systray.Quit() is called.
func Run(onReady func(), onExit func()) {
	Register(onReady, onExit)
	nativeLoop()
}

// Register initializes GUI and registers the callbacks but relies on the
// caller to run the event loop somewhere else. It's useful if the program
// needs to show other UI elements, for example, webview.
// To overcome some OS weirdness, On macOS versions before Catalina, calling
// this does exactly the same as Run().
func Register(onReady func(), onExit func()) {
	if onReady != nil {
		// Run onReady on separate goroutine to avoid blocking event loop
		readyCh := make(chan interface{})
		go func() {
			<-readyCh
			onReady()
		}()
		systrayReady = func() {
			close(readyCh)
		}
	}

	// unlike onReady, onExit runs in the event loop to make sure it has time to
	// finish before the process terminates
	if onExit != nil {
		systrayExit = onExit
	}

	registerSystray()
}

// Quit the systray
func Quit() {
	quitOnce.Do(quit)
}

type MenuItemOption func(item *MenuItem)

// WithTooltip sets the tooltip for MenuItem
func WithTooltip(tooltip string) MenuItemOption {
	return func(item *MenuItem) {
		item.tooltip = tooltip
	}
}

// WithCheckable sets the MenuItem to be checkable with initial value checked.
// MenuItem is checkable on Windows and OSX by default. This option is required
// for Linux to have a checkable MenuItem.
func WithCheckable(checked bool) MenuItemOption {
	return func(item *MenuItem) {
		item.isCheckable = true
		item.checked = checked
	}
}

// WithParent sets the parent for MenuItem to be created
func WithParent(parent *MenuItem) MenuItemOption {
	return func(item *MenuItem) {
		item.parent = parent
	}
}

// WithDisable disables the MenuItem to be created. MenuItem is enabled by
// default.
func WithDisabled() MenuItemOption {
	return func(item *MenuItem) {
		item.disabled = true
	}
}

// WithOnClickedFunc sets the callback function to call when a MenuItem is
// clicked.
func WithOnClickedFunc(callback func()) MenuItemOption {
	return func(item *MenuItem) {
		item.onClicked = callback
	}
}

// NewMenuItem adds a menu item with the designated title and tooltip.
// It can be safely invoked from different goroutines.
func NewMenuItem(title string, opts ...MenuItemOption) *MenuItem {
	item := &MenuItem{
		id:    atomic.AddUint32(&currentID, 1),
		title: title,
	}

	for _, opt := range opts {
		opt(item)
	}

	item.update()
	return item
}

// SetTitle set the text to display on a menu item
func (item *MenuItem) SetTitle(title string) {
	item.title = title
	item.update()
}

// SetTooltip set the tooltip to show when mouse hover
func (item *MenuItem) SetTooltip(tooltip string) {
	item.tooltip = tooltip
	item.update()
}

// IsDisabled checks if the menu item is disabled
func (item *MenuItem) IsDisabled() bool {
	return item.disabled
}

// Enable a menu item regardless if it's previously enabled or not
func (item *MenuItem) Enable() {
	item.disabled = false
	item.update()
}

// Disable a menu item regardless if it's previously disabled or not
func (item *MenuItem) Disable() {
	item.disabled = true
	item.update()
}

// Hide hides a menu item
func (item *MenuItem) Hide() {
	hideMenuItem(item)
}

// Show shows a previously hidden menu item
func (item *MenuItem) Show() {
	showMenuItem(item)
}

// IsChecked returns if the menu item has a check mark
func (item *MenuItem) IsChecked() bool {
	return item.checked
}

// Check a menu item regardless if it's previously checked or not
func (item *MenuItem) Check() {
	item.checked = true
	item.update()
}

// Uncheck a menu item regardless if it's previously unchecked or not
func (item *MenuItem) Uncheck() {
	item.checked = false
	item.update()
}

// update propagates changes on a menu item to systray
func (item *MenuItem) update() {
	menuItems.LoadOrStore(item.id, item)
	addOrUpdateMenuItem(item)
}

func systrayMenuItemSelected(id uint32) {
	if v, ok := menuItems.Load(id); ok {
		if item, ok := v.(*MenuItem); ok {
			if item.onClicked != nil {
				item.onClicked()
			}
		}
	}
}

// NewSeparator adds a separator bar to the menu
func NewSeparator() {
	addSeparator(atomic.AddUint32(&currentID, 1))
}
