use std::sync::{Mutex, Once};

use objc_id::{Id, ShareId};
use tauri::{
    AppHandle, Manager, PhysicalPosition, PhysicalSize, Window, Wry,
};

use cocoa::{
    appkit::{CGFloat, NSMainMenuWindowLevel, NSWindow, NSWindowCollectionBehavior},
    base::{id, nil, BOOL, NO, YES},
    foundation::{NSPoint, NSRect},
};
use objc::{
    class,
    declare::ClassDecl,
    msg_send,
    runtime::{self, Class, Object, Protocol, Sel},
    sel, sel_impl, Message,
};
use objc_foundation::INSObject;

#[link(name = "Foundation", kind = "framework")]
extern "C" {
    pub fn NSMouseInRect(aPoint: NSPoint, aRect: NSRect, flipped: BOOL) -> BOOL;
}

#[derive(Default)]
pub struct Store {
    panel: Option<ShareId<RawNSPanel>>,
}

#[derive(Default)]
pub struct State(pub Mutex<Store>);

#[macro_export]
macro_rules! set_state {
    ($app_handle:expr, $field:ident, $value:expr) => {{
        let handle = $app_handle.app_handle();
        handle
            .state::<$crate::spotlight::State>()
            .0
            .lock()
            .unwrap()
            .$field = $value;
    }};
}

#[macro_export]
macro_rules! get_state {
    ($app_handle:expr, $field:ident) => {{
        let handle = $app_handle.app_handle();
        let value = handle
            .state::<$crate::spotlight::State>()
            .0
            .lock()
            .unwrap()
            .$field;

        value
    }};
    ($app_handle:expr, $field:ident, $action:ident) => {{
        let handle = $app_handle.app_handle();
        let value = handle
            .state::<$crate::spotlight::State>()
            .0
            .lock()
            .unwrap()
            .$field
            .$action();

        value
    }};
}

#[macro_export]
macro_rules! panel {
    ($app_handle:expr) => {{
        let handle = $app_handle.app_handle();
        let panel = handle
            .state::<$crate::spotlight::State>()
            .0
            .lock()
            .unwrap()
            .panel
            .clone();

        panel.unwrap()
    }};
}

#[macro_export]
macro_rules! nsstring_to_string {
    ($ns_string:expr) => {{
        use objc::{sel, sel_impl};
        let utf8: id = unsafe { objc::msg_send![$ns_string, UTF8String] };
        let string = if !utf8.is_null() {
            Some(unsafe {
                {
                    std::ffi::CStr::from_ptr(utf8 as *const std::ffi::c_char)
                        .to_string_lossy()
                        .into_owned()
                }
            })
        } else {
            None
        };

        string
    }};
}

static INIT: Once = Once::new();
static PANEL_LABEL: &str = "main";

#[tauri::command]
pub fn init_spotlight_window(app_handle: AppHandle<Wry>, window: Window<Wry>) {
    INIT.call_once(|| {
        set_state!(app_handle, panel, Some(create_spotlight_panel(&window)));
    });
}

/// Positions a given window at the center of the monitor with cursor
fn position_window_at_the_center_of_the_monitor_with_cursor(window: &Window<Wry>) {
    if let Some(monitor) = get_monitor_with_cursor() {
        let display_size = monitor.size.to_logical::<f64>(monitor.scale_factor);
        let display_pos = monitor.position.to_logical::<f64>(monitor.scale_factor);

        let handle: id = window.ns_window().unwrap() as _;
        let win_frame: NSRect = unsafe { handle.frame() };
        let rect = NSRect {
            origin: NSPoint {
                x: (display_pos.x + (display_size.width / 2.0)) - (win_frame.size.width / 2.0),
                y: (display_pos.y + (display_size.height / 2.0)) - (win_frame.size.height / 2.0),
            },
            size: win_frame.size,
        };
        let _: () = unsafe { msg_send![handle, setFrame: rect display: YES] };
    }
}

struct Monitor {
    #[allow(dead_code)]
    pub name: Option<String>,
    pub size: PhysicalSize<u32>,
    pub position: PhysicalPosition<i32>,
    pub scale_factor: f64,
}

/// Gets the Monitor with cursor
fn get_monitor_with_cursor() -> Option<Monitor> {
    objc::rc::autoreleasepool(|| {
        let mouse_location: NSPoint = unsafe { msg_send![class!(NSEvent), mouseLocation] };
        let screens: id = unsafe { msg_send![class!(NSScreen), screens] };
        let screens_iter: id = unsafe { msg_send![screens, objectEnumerator] };
        let mut next_screen: id;

        let frame_with_cursor: Option<NSRect> = loop {
            next_screen = unsafe { msg_send![screens_iter, nextObject] };
            if next_screen == nil {
                break None;
            }

            let frame: NSRect = unsafe { msg_send![next_screen, frame] };
            let is_mouse_in_screen_frame: BOOL =
                unsafe { NSMouseInRect(mouse_location, frame, NO) };
            if is_mouse_in_screen_frame == YES {
                break Some(frame);
            }
        };

        if let Some(frame) = frame_with_cursor {
            let name: id = unsafe { msg_send![next_screen, localizedName] };
            let screen_name = nsstring_to_string!(name);
            let scale_factor: CGFloat = unsafe { msg_send![next_screen, backingScaleFactor] };
            let scale_factor: f64 = scale_factor;

            return Some(Monitor {
                name: screen_name,
                position: PhysicalPosition {
                    x: (frame.origin.x * scale_factor) as i32,
                    y: (frame.origin.y * scale_factor) as i32,
                },
                size: PhysicalSize {
                    width: (frame.size.width * scale_factor) as u32,
                    height: (frame.size.height * scale_factor) as u32,
                },
                scale_factor,
            });
        }

        None
    })
}

extern "C" {
    pub fn object_setClass(obj: id, cls: id) -> id;
}

#[allow(non_upper_case_globals)]
const NSWindowStyleMaskNonActivatingPanel: i32 = 1 << 7;

const CLS_NAME: &str = "RawNSPanel";

pub struct RawNSPanel;

impl RawNSPanel {
    fn get_class() -> &'static Class {
        Class::get(CLS_NAME).unwrap_or_else(Self::define_class)
    }

    fn define_class() -> &'static Class {
        let mut cls = ClassDecl::new(CLS_NAME, class!(NSPanel))
            .unwrap_or_else(|| panic!("Unable to register {} class", CLS_NAME));

        unsafe {
            cls.add_method(
                sel!(canBecomeKeyWindow),
                Self::can_become_key_window as extern "C" fn(&Object, Sel) -> BOOL,
            );
        }

        cls.register()
    }

    /// Returns YES to ensure that RawNSPanel can become a key window
    extern "C" fn can_become_key_window(_: &Object, _: Sel) -> BOOL {
        YES
    }
}
unsafe impl Message for RawNSPanel {}

impl RawNSPanel {
    fn show(&self) {
        self.make_first_responder(Some(self.content_view()));
        self.order_front_regardless();
        self.make_key_window();
    }

    fn is_visible(&self) -> bool {
        let flag: BOOL = unsafe { msg_send![self, isVisible] };
        flag == YES
    }

    fn make_key_window(&self) {
        let _: () = unsafe { msg_send![self, makeKeyWindow] };
    }

    fn order_front_regardless(&self) {
        let _: () = unsafe { msg_send![self, orderFrontRegardless] };
    }

    fn order_out(&self, sender: Option<id>) {
        let _: () = unsafe { msg_send![self, orderOut: sender.unwrap_or(nil)] };
    }

    fn content_view(&self) -> id {
        unsafe { msg_send![self, contentView] }
    }

    fn make_first_responder(&self, sender: Option<id>) {
        if let Some(responder) = sender {
            let _: () = unsafe { msg_send![self, makeFirstResponder: responder] };
        } else {
            let _: () = unsafe { msg_send![self, makeFirstResponder: self] };
        }
    }

    fn set_level(&self, level: i32) {
        let _: () = unsafe { msg_send![self, setLevel: level] };
    }

    fn set_style_mask(&self, style_mask: i32) {
        let _: () = unsafe { msg_send![self, setStyleMask: style_mask] };
    }

    fn set_collection_behaviour(&self, behaviour: NSWindowCollectionBehavior) {
        let _: () = unsafe { msg_send![self, setCollectionBehavior: behaviour] };
    }

    fn set_delegate(&self, delegate: Option<Id<RawNSPanelDelegate>>) {
        if let Some(del) = delegate {
            let _: () = unsafe { msg_send![self, setDelegate: del] };
        } else {
            let _: () = unsafe { msg_send![self, setDelegate: self] };
        }
    }

    /// Create an NSPanel from Tauri's NSWindow
    fn from(ns_window: id) -> Id<Self> {
        let ns_panel: id = unsafe { msg_send![Self::class(), class] };
        unsafe {
            object_setClass(ns_window, ns_panel);
            Id::from_retained_ptr(ns_window as *mut Self)
        }
    }
}

impl INSObject for RawNSPanel {
    fn class() -> &'static runtime::Class {
        RawNSPanel::get_class()
    }
}

#[allow(dead_code)]
const DELEGATE_CLS_NAME: &str = "RawNSPanelDelegate";

#[allow(dead_code)]
struct RawNSPanelDelegate {}

impl RawNSPanelDelegate {
    #[allow(dead_code)]
    fn get_class() -> &'static Class {
        Class::get(DELEGATE_CLS_NAME).unwrap_or_else(Self::define_class)
    }

    #[allow(dead_code)]
    fn define_class() -> &'static Class {
        let mut cls = ClassDecl::new(DELEGATE_CLS_NAME, class!(NSObject))
            .unwrap_or_else(|| panic!("Unable to register {} class", DELEGATE_CLS_NAME));

        cls.add_protocol(
            Protocol::get("NSWindowDelegate").expect("Failed to get NSWindowDelegate protocol"),
        );

        unsafe {
            cls.add_ivar::<id>("panel");

            cls.add_method(
                sel!(setPanel:),
                Self::set_panel as extern "C" fn(&mut Object, Sel, id),
            );

            cls.add_method(
                sel!(windowDidBecomeKey:),
                Self::window_did_become_key as extern "C" fn(&Object, Sel, id),
            );

            cls.add_method(
                sel!(windowDidResignKey:),
                Self::window_did_resign_key as extern "C" fn(&Object, Sel, id),
            );
        }

        cls.register()
    }

    extern "C" fn set_panel(this: &mut Object, _: Sel, panel: id) {
        unsafe { this.set_ivar("panel", panel) };
    }

    extern "C" fn window_did_become_key(_: &Object, _: Sel, _: id) {}

    /// Hide panel when it's no longer the key window
    extern "C" fn window_did_resign_key(this: &Object, _: Sel, _: id) {
        let panel: id = unsafe { *this.get_ivar("panel") };
        let _: () = unsafe { msg_send![panel, orderOut: nil] };
    }
}

unsafe impl Message for RawNSPanelDelegate {}

impl INSObject for RawNSPanelDelegate {
    fn class() -> &'static runtime::Class {
        Self::get_class()
    }
}

impl RawNSPanelDelegate {
    pub fn set_panel_(&self, panel: ShareId<RawNSPanel>) {
        let _: () = unsafe { msg_send![self, setPanel: panel] };
    }
}

fn create_spotlight_panel(window: &Window<Wry>) -> ShareId<RawNSPanel> {
    // Convert NSWindow Object to NSPanel
    let handle: id = window.ns_window().unwrap() as _;
    let panel = RawNSPanel::from(handle);
    let panel = panel.share();

    // Set panel above the main menu window level
    panel.set_level(NSMainMenuWindowLevel + 1);

    // Ensure that the panel can display over the top of fullscreen apps
    panel.set_collection_behaviour(
        NSWindowCollectionBehavior::NSWindowCollectionBehaviorTransient
            | NSWindowCollectionBehavior::NSWindowCollectionBehaviorMoveToActiveSpace
            | NSWindowCollectionBehavior::NSWindowCollectionBehaviorFullScreenAuxiliary,
    );

    // Ensures panel does not activate
    panel.set_style_mask(NSWindowStyleMaskNonActivatingPanel);

    // Setup delegate for an NSPanel to listen for window resign key and hide the panel
    let delegate = RawNSPanelDelegate::new();
    delegate.set_panel_(panel.clone());
    panel.set_delegate(Some(delegate));

    panel
}