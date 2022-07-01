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
	menuItems    sync.Map // map[uint32]*menuItem

	currentID = uint32(0)
	quitOnce  sync.Once
)

func init() {
	runtime.LockOSThread()
}

// menuItem is used to keep track each menu item of systray.
type menuItem struct {
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
	parent *menuItem
}

func (item *menuItem) String() string {
	if item.parent == nil {
		return fmt.Sprintf("menuItem[%d, %q]", item.id, item.title)
	}
	return fmt.Sprintf("menuItem[%d, parent %d, %q]", item.id, item.parent.id, item.title)
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
		systrayReady = func() {
			go onReady()
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

type MenuItemOption func(item *menuItem)

// WithTooltip sets the tooltip for menuItem
func WithTooltip(tooltip string) MenuItemOption {
	return func(item *menuItem) {
		item.tooltip = tooltip
	}
}

// WithCheckable sets the menuItem to be checkable with initial value checked.
// menuItem is checkable on Windows and OSX by default. This option is required
// for Linux to have a checkable menuItem.
func WithCheckable(checked bool) MenuItemOption {
	return func(item *menuItem) {
		item.isCheckable = true
		item.checked = checked
	}
}

// WithParent sets the parent for menuItem to be created
func WithParent(parent *menuItem) MenuItemOption {
	return func(item *menuItem) {
		item.parent = parent
	}
}

// WithDisable disables the menuItem to be created. menuItem is enabled by
// default.
func WithDisabled() MenuItemOption {
	return func(item *menuItem) {
		item.disabled = true
	}
}

// WithOnClickedFunc sets the callback function to call when a menuItem is
// clicked.
func WithOnClickedFunc(callback func()) MenuItemOption {
	return func(item *menuItem) {
		item.onClicked = callback
	}
}

// NewMenuItem adds a menu item with the designated title and tooltip.
// It can be safely invoked from different goroutines.
func NewMenuItem(title string, opts ...MenuItemOption) *menuItem {
	item := &menuItem{
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
func (item *menuItem) SetTitle(title string) {
	item.title = title
	item.update()
}

// SetTooltip set the tooltip to show when mouse hover
func (item *menuItem) SetTooltip(tooltip string) {
	item.tooltip = tooltip
	item.update()
}

// IsDisabled checks if the menu item is disabled
func (item *menuItem) IsDisabled() bool {
	return item.disabled
}

// Enable a menu item regardless if it's previously enabled or not
func (item *menuItem) Enable() {
	item.disabled = false
	item.update()
}

// Disable a menu item regardless if it's previously disabled or not
func (item *menuItem) Disable() {
	item.disabled = true
	item.update()
}

// Hide hides a menu item
func (item *menuItem) Hide() {
	hideMenuItem(item)
}

// Show shows a previously hidden menu item
func (item *menuItem) Show() {
	showMenuItem(item)
}

// IsChecked returns if the menu item has a check mark
func (item *menuItem) IsChecked() bool {
	return item.checked
}

// Check a menu item regardless if it's previously checked or not
func (item *menuItem) Check() {
	item.checked = true
	item.update()
}

// Uncheck a menu item regardless if it's previously unchecked or not
func (item *menuItem) Uncheck() {
	item.checked = false
	item.update()
}

// update propagates changes on a menu item to systray
func (item *menuItem) update() {
	menuItems.LoadOrStore(item.id, item)
	addOrUpdateMenuItem(item)
}

func systrayMenuItemSelected(id uint32) {
	if v, ok := menuItems.Load(id); ok {
		if item, ok := v.(*menuItem); ok {
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
