//! App-owned semantic bridge. No scanner runtime or private-source dependencies.
use accesskit::{
    Action, ActionData, ActionHandler, ActionRequest, ActivationHandler, DeactivationHandler, Live,
    Node, NodeId, Rect, Role, TextPosition, TextSelection, Toggled, TreeId, TreeInfo, TreeUpdate,
};
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet, VecDeque};
use std::ffi::{c_char, c_void, CStr, CString};
use std::sync::{Arc, Mutex};

#[derive(Clone, Deserialize, Default)]
#[serde(default)]
struct Bounds {
    #[serde(rename = "X")]
    x: f64,
    #[serde(rename = "Y")]
    y: f64,
    #[serde(rename = "Width")]
    width: f64,
    #[serde(rename = "Height")]
    height: f64,
}
#[derive(Clone, Deserialize, Default)]
#[serde(default)]
struct SemanticNode {
    id: u64,
    parent: u64,
    role: String,
    name: String,
    value: String,
    description: String,
    placeholder: String,
    bounds: Bounds,
    disabled: bool,
    checked: bool,
    expanded: bool,
    selected: bool,
    live: String,
    actions: Vec<String>,
    selection: Option<SemanticSelection>,
}
#[derive(Clone, Deserialize, Serialize)]
struct SemanticSelection {
    anchor: usize,
    focus: usize,
}
#[derive(Clone, Deserialize)]
struct Snapshot {
    nodes: Vec<SemanticNode>,
    focus: u64,
    window_focused: bool,
}
impl Default for Snapshot {
    fn default() -> Self {
        Self {
            nodes: vec![SemanticNode {
                id: 1,
                role: "window".into(),
                name: "Scanner".into(),
                ..Default::default()
            }],
            focus: 1,
            window_focused: false,
        }
    }
}
#[derive(Serialize)]
struct SemanticAction {
    node_id: u64,
    kind: &'static str,
    value: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    selection: Option<SemanticSelection>,
}
struct Shared {
    snapshot: Mutex<Snapshot>,
    actions: Mutex<VecDeque<SemanticAction>>,
}
#[derive(Clone)]
struct Handler(Arc<Shared>);
impl ActivationHandler for Handler {
    fn request_initial_tree(&mut self) -> Option<TreeUpdate> {
        Some(self.0.snapshot.lock().unwrap().tree())
    }
}
impl DeactivationHandler for Handler {
    fn deactivate_accessibility(&mut self) {}
}
impl ActionHandler for Handler {
    fn do_action(&mut self, req: ActionRequest) {
        let mut kind = match req.action {
            Action::Click => "click",
            Action::Focus => "focus",
            Action::SetValue => "set_value",
            Action::Expand => "expand",
            Action::Collapse => "collapse",
            Action::SetTextSelection => "set_text_selection",
            _ => return,
        };
        let snapshot = self.0.snapshot.lock().unwrap();
        let Some(node) = snapshot.nodes.iter().find(|n| n.id == req.target_node.0) else {
            return;
        };
        if kind == "click"
            && !node.actions.iter().any(|a| a == "click")
            && node.actions.iter().any(|a| a == "select")
        {
            kind = "select";
        }
        if node.disabled || !node.actions.iter().any(|a| a == kind) {
            return;
        }
        let mut selection = None;
        if kind == "set_text_selection" {
            let Some(ActionData::SetTextSelection(ref range)) = req.data else {
                return;
            };
            let run = NodeId(node.id | (1u64 << 63));
            let count = character_lengths(&node.value).len();
            if range.anchor.node != run
                || range.focus.node != run
                || range.anchor.character_index > count
                || range.focus.character_index > count
            {
                return;
            }
            selection = Some(SemanticSelection {
                anchor: range.anchor.character_index,
                focus: range.focus.character_index,
            });
        }
        let value = match req.data {
            Some(ActionData::Value(v)) => v.to_string(),
            _ => String::new(),
        };
        let mut queue = self.0.actions.lock().unwrap();
        if queue.len() < 256 {
            queue.push_back(SemanticAction {
                node_id: node.id,
                kind,
                value,
                selection,
            });
        }
    }
}
fn role(value: &str) -> Option<Role> {
    Some(match value {
        "window" => Role::Window,
        "group" => Role::Group,
        "button" => Role::Button,
        "text" => Role::Label,
        "entry" => Role::TextInput,
        "checkbox" => Role::CheckBox,
        "combobox" => Role::ComboBox,
        "option" => Role::ListBoxOption,
        "tree" => Role::Tree,
        "treeitem" => Role::TreeItem,
        "status" => Role::Label,
        _ => return None,
    })
}
fn character_lengths(text: &str) -> Vec<u8> {
    let mut chars = text.chars().peekable();
    let mut lengths = Vec::new();
    while let Some(c) = chars.next() {
        if c == '\r' && chars.peek() == Some(&'\n') {
            chars.next();
            lengths.push(2)
        } else {
            lengths.push(c.len_utf8() as u8)
        }
    }
    lengths
}
impl Snapshot {
    fn validate(&self) -> bool {
        let nodes: BTreeMap<_, _> = self.nodes.iter().map(|n| (n.id, n)).collect();
        if nodes.len() != self.nodes.len()
            || nodes
                .get(&1)
                .is_none_or(|n| n.parent != 0 || n.role != "window")
        {
            return false;
        }
        if self.focus != 0 && !nodes.contains_key(&self.focus) {
            return false;
        }
        for node in &self.nodes {
            if let Some(range) = &node.selection {
                let count = character_lengths(&node.value).len();
                if node.role != "entry" || range.anchor > count || range.focus > count {
                    return false;
                }
            }
            if node.id >= (1u64 << 63) {
                return false;
            }
            if node.id == 0
                || node.id >= (1u64 << 63)
                || role(&node.role).is_none()
                || [
                    node.bounds.x,
                    node.bounds.y,
                    node.bounds.width,
                    node.bounds.height,
                ]
                .iter()
                .any(|v| !v.is_finite())
                || node.bounds.width < 0.
                || node.bounds.height < 0.
            {
                return false;
            }
            let mut id = node.id;
            let mut seen = BTreeSet::new();
            while id != 1 {
                if !seen.insert(id) {
                    return false;
                };
                let Some(n) = nodes.get(&id) else {
                    return false;
                };
                id = n.parent;
            }
        }
        true
    }
    fn tree(&self) -> TreeUpdate {
        let nodes = self
            .nodes
            .iter()
            .flat_map(|n| {
                let mut node = Node::new(role(&n.role).unwrap());
                let text = if matches!(n.role.as_str(), "text" | "status") && n.value.is_empty() {
                    n.name.clone()
                } else {
                    n.value.clone()
                };
                node.set_value(text.clone());
                // Native macOS Label AXValue reads the label, whereas AT-SPI Text
                // reads its TextRun. Both must expose the visible status itself.
                if matches!(n.role.as_str(), "text" | "status") {
                    node.set_label(text.clone());
                    let context = if n.name != text { n.name.as_str() } else { "" };
                    let description = [context, n.description.as_str()]
                        .into_iter()
                        .filter(|part| !part.is_empty())
                        .collect::<Vec<_>>()
                        .join(". ");
                    if !description.is_empty() {
                        node.set_description(description);
                    }
                } else {
                    node.set_label(n.name.clone());
                    if !n.description.is_empty() {
                        node.set_description(n.description.clone());
                    }
                }
                if !n.placeholder.is_empty() {
                    node.set_placeholder(n.placeholder.clone())
                }
                if n.bounds.width > 0. && n.bounds.height > 0. {
                    node.set_bounds(Rect::new(
                        n.bounds.x,
                        n.bounds.y,
                        n.bounds.x + n.bounds.width,
                        n.bounds.y + n.bounds.height,
                    ));
                }
                let text_run = matches!(n.role.as_str(), "entry" | "text" | "status");
                let run_id = NodeId(n.id | (1u64 << 63));
                let mut children = self
                    .nodes
                    .iter()
                    .filter(|c| c.parent == n.id)
                    .map(|c| NodeId(c.id))
                    .collect::<Vec<_>>();
                if text_run {
                    children.push(run_id);
                }
                node.set_children(children);
                if n.disabled {
                    node.set_disabled()
                }
                if n.role == "checkbox" {
                    node.set_toggled(if n.checked {
                        Toggled::True
                    } else {
                        Toggled::False
                    });
                }
                if n.role == "treeitem" {
                    node.set_selected(n.selected);
                }
                if n.actions.iter().any(|a| a == "expand" || a == "collapse") {
                    node.set_expanded(n.expanded);
                }
                if n.role == "option" {
                    node.set_selected(n.selected);
                }
                if n.live == "polite" {
                    node.set_live(Live::Polite);
                }
                for a in &n.actions {
                    match a.as_str() {
                        "click" | "select" => node.add_action(Action::Click),
                        "focus" => node.add_action(Action::Focus),
                        "set_value" => node.add_action(Action::SetValue),
                        "expand" => node.add_action(Action::Expand),
                        "collapse" => node.add_action(Action::Collapse),
                        "set_text_selection" => node.add_action(Action::SetTextSelection),
                        _ => {}
                    }
                }
                if let Some(range) = &n.selection {
                    node.set_text_selection(TextSelection {
                        anchor: TextPosition {
                            node: run_id,
                            character_index: range.anchor,
                        },
                        focus: TextPosition {
                            node: run_id,
                            character_index: range.focus,
                        },
                    });
                }
                let mut result = vec![(NodeId(n.id), node)];
                if text_run {
                    let mut run = Node::new(Role::TextRun);
                    run.set_value(text.clone());
                    run.set_character_lengths(character_lengths(&text));
                    result.push((run_id, run));
                }
                result
            })
            .collect();
        TreeUpdate {
            nodes,
            tree: Some(TreeInfo::new(NodeId(1))),
            tree_id: TreeId::ROOT,
            focus: NodeId(if self.focus == 0 { 1 } else { self.focus }),
        }
    }
}

fn native_snapshot(mut snapshot: Snapshot, scale: f64, focused: bool) -> Option<Snapshot> {
    if !scale.is_finite() || scale <= 0. {
        return None;
    }
    for node in &mut snapshot.nodes {
        node.bounds.x *= scale;
        node.bounds.y *= scale;
        node.bounds.width *= scale;
        node.bounds.height *= scale;
    }
    snapshot.window_focused = focused;
    snapshot.validate().then_some(snapshot)
}
struct Bridge {
    shared: Arc<Shared>,
    platform: Mutex<Platform>,
}
#[cfg(target_os = "macos")]
type Platform = accesskit_macos::SubclassingAdapter;
#[cfg(target_os = "windows")]
type Platform = accesskit_windows::Adapter;
#[cfg(target_os = "linux")]
type Platform = accesskit_unix::Adapter;

#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_new(window: *mut c_void) -> *mut c_void {
    std::panic::catch_unwind(|| {
        let shared = Arc::new(Shared {
            snapshot: Mutex::new(Snapshot::default()),
            actions: Mutex::new(VecDeque::new()),
        });
        let handler = Handler(shared.clone());
        #[cfg(target_os = "macos")]
        let platform = unsafe {
            accesskit_macos::SubclassingAdapter::for_window(window, handler.clone(), handler)
        };
        #[cfg(target_os = "windows")]
        let platform =
            accesskit_windows::Adapter::new(accesskit_windows::HWND(window), false, handler);
        #[cfg(target_os = "linux")]
        let platform = {
            let _ = window;
            accesskit_unix::Adapter::new(handler.clone(), handler.clone(), handler)
        };
        Box::into_raw(Box::new(Bridge {
            shared,
            platform: Mutex::new(platform),
        }))
        .cast()
    })
    .unwrap_or(std::ptr::null_mut())
}

#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_update(
    bridge: *mut c_void,
    json: *const c_char,
    backing_scale: f64,
    native_focused: i32,
) -> i32 {
    if bridge.is_null() || json.is_null() {
        return -1;
    }
    std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        let bridge = unsafe { &*bridge.cast::<Bridge>() };
        let Ok(snapshot) =
            serde_json::from_slice::<Snapshot>(unsafe { CStr::from_ptr(json) }.to_bytes())
        else {
            return -1;
        };
        let Some(snapshot) = native_snapshot(snapshot, backing_scale, native_focused != 0) else {
            return -1;
        };
        let focused = snapshot.window_focused;
        *bridge.shared.snapshot.lock().unwrap() = snapshot.clone();
        let mut platform = bridge.platform.lock().unwrap();
        #[cfg(target_os = "linux")]
        {
            platform.update_if_active(|| snapshot.tree());
            platform.update_window_focus_state(focused);
        }
        #[cfg(target_os = "macos")]
        {
            let events = platform.update_if_active(|| snapshot.tree());
            let focus = platform.update_view_focus_state(focused);
            drop(platform);
            if let Some(e) = events {
                e.raise()
            }
            if let Some(e) = focus {
                e.raise()
            }
        }
        #[cfg(target_os = "windows")]
        {
            let events = platform.update_if_active(|| snapshot.tree());
            let focus = platform.update_window_focus_state(focused);
            drop(platform);
            if let Some(e) = events {
                e.raise()
            }
            if let Some(e) = focus {
                e.raise()
            }
        }
        0
    }))
    .unwrap_or(-1)
}
#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_poll(bridge: *mut c_void) -> *mut c_char {
    if bridge.is_null() {
        return std::ptr::null_mut();
    }
    let bridge = unsafe { &*bridge.cast::<Bridge>() };
    let actions: Vec<_> = bridge.shared.actions.lock().unwrap().drain(..).collect();
    CString::new(serde_json::to_string(&actions).unwrap())
        .unwrap()
        .into_raw()
}
#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_string_free(value: *mut c_char) {
    if !value.is_null() {
        drop(unsafe { CString::from_raw(value) })
    }
}
#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_free(bridge: *mut c_void) {
    if !bridge.is_null() {
        drop(unsafe { Box::from_raw(bridge.cast::<Bridge>()) })
    }
}

#[cfg(target_os = "windows")]
#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_getobject(
    bridge: *mut c_void,
    wparam: usize,
    lparam: isize,
    handled: *mut i32,
) -> isize {
    let bridge = unsafe { &*bridge.cast::<Bridge>() };
    let mut handler = Handler(bridge.shared.clone());
    let mut platform = bridge.platform.lock().unwrap();
    let response = platform.handle_wm_getobject(
        accesskit_windows::WPARAM(wparam),
        accesskit_windows::LPARAM(lparam),
        &mut handler,
    );
    drop(platform);
    if let Some(response) = response {
        unsafe { *handled = 1 };
        let value: accesskit_windows::LRESULT = response.into();
        value.0
    } else {
        0
    }
}

#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_bounds(
    bridge: *mut c_void,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) {
    #[cfg(target_os = "linux")]
    if !bridge.is_null() {
        let bridge = unsafe { &*bridge.cast::<Bridge>() };
        bridge.platform.lock().unwrap().set_root_window_bounds(
            Rect::new(x, y, x + width, y + height),
            Rect::new(x, y, x + width, y + height),
        );
    }
    #[cfg(not(target_os = "linux"))]
    let _ = (bridge, x, y, width, height);
}

#[cfg(target_os = "macos")]
#[no_mangle]
/// # Safety
/// Native window pointers must be live and native operations run on their owning
/// thread. Bridge pointers must originate from `scanner_a11y_new`, remain live,
/// and have serialized access. String pointers must be valid NUL-terminated UTF-8;
/// returned strings are freed only with `scanner_a11y_string_free`.
pub unsafe extern "C" fn scanner_a11y_macos_focus_class(name: *const c_char) {
    static PATCH: std::sync::Once = std::sync::Once::new();
    if !name.is_null() {
        let name = unsafe { CStr::from_ptr(name) }.to_string_lossy();
        PATCH.call_once(|| unsafe { accesskit_macos::add_focus_forwarder_to_window_class(&name) });
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn text_selection_is_unicode_scoped_and_validated() {
        let mut snapshot = Snapshot::default();
        snapshot.nodes.push(SemanticNode {
            id: 2,
            parent: 1,
            role: "entry".into(),
            value: "aé🙂z".into(),
            selection: Some(SemanticSelection {
                anchor: 3,
                focus: 1,
            }),
            actions: vec!["set_text_selection".into()],
            ..Default::default()
        });
        assert!(snapshot.validate());
        let tree = snapshot.tree();
        let node = &tree
            .nodes
            .iter()
            .find(|(id, _)| *id == NodeId(2))
            .unwrap()
            .1;
        assert_eq!(node.text_selection().unwrap().anchor.character_index, 3);
        assert_eq!(
            node.text_selection().unwrap().focus.node,
            NodeId(2 | (1u64 << 63))
        );
        let shared = Arc::new(Shared {
            snapshot: Mutex::new(snapshot.clone()),
            actions: Mutex::new(VecDeque::new()),
        });
        let mut handler = Handler(shared.clone());
        let request = |run, focus| ActionRequest {
            action: Action::SetTextSelection,
            target_tree: TreeId::ROOT,
            target_node: NodeId(2),
            data: Some(ActionData::SetTextSelection(TextSelection {
                anchor: TextPosition {
                    node: NodeId(run),
                    character_index: 1,
                },
                focus: TextPosition {
                    node: NodeId(run),
                    character_index: focus,
                },
            })),
        };
        handler.do_action(request(2 | (1u64 << 63), 3));
        assert_eq!(shared.actions.lock().unwrap().len(), 1);
        handler.do_action(request(3 | (1u64 << 63), 2));
        handler.do_action(request(2 | (1u64 << 63), 5));
        assert_eq!(shared.actions.lock().unwrap().len(), 1);
        snapshot.nodes[1].selection.as_mut().unwrap().focus = 5;
        assert!(!snapshot.validate());
    }
    #[test]
    fn rejects_disconnected_duplicate_or_cycle() {
        let mut s = Snapshot::default();
        assert!(s.validate());
        s.nodes.push(s.nodes[0].clone());
        assert!(!s.validate());
        s.nodes.pop();
        s.nodes.push(SemanticNode {
            id: 2,
            parent: 3,
            role: "button".into(),
            ..Default::default()
        });
        assert!(!s.validate());
        s.nodes.push(SemanticNode {
            id: 3,
            parent: 2,
            role: "group".into(),
            ..Default::default()
        });
        assert!(!s.validate());
    }
    #[test]
    fn rejects_unknown_focus_and_bad_geometry() {
        let mut s = Snapshot {
            focus: 2,
            ..Default::default()
        };
        assert!(!s.validate());
        s.focus = 1;
        s.nodes[0].bounds.width = f64::NAN;
        assert!(!s.validate());
    }
    #[test]
    fn action_dispatch_is_supported_and_bounded() {
        let mut s = Snapshot::default();
        s.nodes.push(SemanticNode {
            id: 2,
            parent: 1,
            role: "button".into(),
            actions: vec!["click".into()],
            ..Default::default()
        });
        let shared = Arc::new(Shared {
            snapshot: Mutex::new(s),
            actions: Mutex::new(VecDeque::new()),
        });
        let mut handler = Handler(shared.clone());
        for _ in 0..300 {
            handler.do_action(ActionRequest {
                action: Action::Click,
                target_tree: TreeId::ROOT,
                target_node: NodeId(2),
                data: None,
            });
        }
        assert_eq!(shared.actions.lock().unwrap().len(), 256);
        shared.snapshot.lock().unwrap().nodes[1].disabled = true;
        shared.actions.lock().unwrap().clear();
        handler.do_action(ActionRequest {
            action: Action::Click,
            target_tree: TreeId::ROOT,
            target_node: NodeId(2),
            data: None,
        });
        assert!(shared.actions.lock().unwrap().is_empty());
    }
    #[test]
    fn value_and_selection_requests_preserve_app_semantics() {
        let mut snapshot = Snapshot::default();
        snapshot.nodes.push(SemanticNode {
            id: 2,
            parent: 1,
            role: "entry".into(),
            actions: vec!["set_value".into()],
            ..Default::default()
        });
        snapshot.nodes.push(SemanticNode {
            id: 3,
            parent: 1,
            role: "option".into(),
            actions: vec!["select".into()],
            ..Default::default()
        });
        let shared = Arc::new(Shared {
            snapshot: Mutex::new(snapshot),
            actions: Mutex::new(VecDeque::new()),
        });
        let mut handler = Handler(shared.clone());
        handler.do_action(ActionRequest {
            action: Action::SetValue,
            target_tree: TreeId::ROOT,
            target_node: NodeId(2),
            data: Some(ActionData::Value("synthetic path".into())),
        });
        handler.do_action(ActionRequest {
            action: Action::Click,
            target_tree: TreeId::ROOT,
            target_node: NodeId(3),
            data: None,
        });
        let actions = shared.actions.lock().unwrap();
        assert_eq!(actions.len(), 2);
        assert_eq!(actions[0].value, "synthetic path");
        assert_eq!(actions[1].kind, "select");
    }
    #[test]
    fn status_label_and_text_run_expose_actual_warning_with_context() {
        let mut snapshot = Snapshot::default();
        snapshot.nodes.push(SemanticNode {
            id: 2,
            parent: 1,
            role: "status".into(),
            name: "Scan status".into(),
            value: "Finished with warnings: inaccessible folder".into(),
            description: "Partial results".into(),
            ..Default::default()
        });
        snapshot.nodes.push(SemanticNode {
            id: 3,
            parent: 1,
            role: "entry".into(),
            name: "Folder path".into(),
            value: "/synthetic".into(),
            ..Default::default()
        });
        let tree = snapshot.tree();
        let node = |id| {
            &tree
                .nodes
                .iter()
                .find(|(key, _)| *key == NodeId(id))
                .unwrap()
                .1
        };
        assert_eq!(
            node(2).label(),
            Some("Finished with warnings: inaccessible folder")
        );
        assert_eq!(
            node(2).value(),
            Some("Finished with warnings: inaccessible folder")
        );
        assert_eq!(node(2).description(), Some("Scan status. Partial results"));
        assert_eq!(node(2 | (1u64 << 63)).value(), node(2).value());
        assert_eq!(node(3).label(), Some("Folder path"));
        assert_eq!(node(3).value(), Some("/synthetic"));
    }
    #[test]
    fn offscreen_nodes_have_no_fabricated_bounds() {
        let snapshot = Snapshot::default();
        assert_eq!(snapshot.tree().nodes[0].1.bounds(), None);
    }
    #[test]
    fn entries_have_text_runs_and_only_expandable_nodes_have_expanded_state() {
        let mut snapshot = Snapshot::default();
        snapshot.nodes.push(SemanticNode {
            id: 2,
            parent: 1,
            role: "entry".into(),
            value: "Δ path".into(),
            ..Default::default()
        });
        snapshot.nodes.push(SemanticNode {
            id: 3,
            parent: 1,
            role: "combobox".into(),
            actions: vec!["expand".into()],
            ..Default::default()
        });
        snapshot.nodes.push(SemanticNode {
            id: 4,
            parent: 1,
            role: "treeitem".into(),
            ..Default::default()
        });
        let tree = snapshot.tree();
        assert_eq!(
            tree.nodes
                .iter()
                .find(|n| n.0 == NodeId(2))
                .unwrap()
                .1
                .children(),
            &[NodeId(2 | (1u64 << 63))]
        );
        assert_eq!(
            tree.nodes
                .iter()
                .find(|n| n.0 == NodeId(2 | (1u64 << 63)))
                .unwrap()
                .1
                .character_lengths(),
            &[2, 1, 1, 1, 1, 1]
        );
        assert_eq!(
            tree.nodes
                .iter()
                .find(|n| n.0 == NodeId(3))
                .unwrap()
                .1
                .is_expanded(),
            Some(false)
        );
        assert_eq!(
            tree.nodes
                .iter()
                .find(|n| n.0 == NodeId(4))
                .unwrap()
                .1
                .is_expanded(),
            None
        );
    }
    #[test]
    fn rejects_reserved_text_run_id_namespace() {
        let mut s = Snapshot::default();
        s.nodes.push(SemanticNode {
            id: 1u64 << 63,
            parent: 1,
            role: "button".into(),
            ..Default::default()
        });
        assert!(!s.validate());
    }
    #[test]
    fn utf8_character_lengths_keep_crlf_as_one_character() {
        let text = "Δ\r\nA";
        let lengths = character_lengths(text);
        assert_eq!(lengths, [2, 2, 1]);
        assert_eq!(
            lengths.iter().map(|n| *n as usize).sum::<usize>(),
            text.len()
        );
    }
    #[test]
    fn native_backing_transform_preserves_ax_logical_bounds_at_one_and_two_x() {
        let mut snapshot = Snapshot::default();
        snapshot.nodes[0].bounds.width = 100.;
        snapshot.nodes[0].bounds.x = 12.;
        for factor in [1., 2.] {
            let transformed = native_snapshot(snapshot.clone(), factor, true).unwrap();
            assert_eq!(transformed.nodes[0].bounds.width / factor, 100.);
            assert_eq!(transformed.nodes[0].bounds.x / factor, 12.);
            assert!(transformed.window_focused);
        }
        assert!(native_snapshot(snapshot, 0., false).is_none());
    }
    #[test]
    fn full_tree_has_stable_focus_and_children() {
        let s = Snapshot::default();
        let tree = s.tree();
        assert_eq!(tree.focus, NodeId(1));
        assert_eq!(tree.nodes.len(), 1);
    }
}
