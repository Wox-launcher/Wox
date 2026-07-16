package widget

import woxui "wox/ui/runtime"

// Key gives one stateful or interactive widget a stable identity among its siblings.
type Key string

type semanticBehavior struct {
	automationID   string
	role           woxui.AccessibilityRole
	label          string
	description    string
	value          string
	actions        []woxui.AccessibilityAction
	liveRegion     woxui.AccessibilityLiveRegion
	enabled        bool
	selected       bool
	checked        bool
	expanded       bool
	readOnly       bool
	protected      bool
	hidden         bool
	nativeBoundary bool
	onAction       func(action woxui.AccessibilityAction, value string) error
}

type focusBehavior struct {
	autofocus     bool
	disabled      bool
	onKeyCapture  func(event woxui.KeyEvent) bool
	onKey         func(event woxui.KeyEvent) bool
	onTextInput   func(event woxui.TextInputEvent) bool
	onFocusChange func(focused bool)
	textInput     func(bounds woxui.Rect) woxui.TextInputState
}

type focusScopeBehavior struct {
	modal bool
}

// Keyed assigns identity without changing layout or paint.
type Keyed struct {
	Key   Key
	Child Widget
}

func (w Keyed) layout(ctx context, available constraints) *node {
	if w.Child == nil {
		return &node{key: w.Key, kind: "keyed"}
	}
	child := w.Child.layout(ctx, available)
	child.key = w.Key
	if child.kind == "" {
		child.kind = "keyed"
	}
	return child
}

// Semantics exposes one logical control to accessibility and test automation.
type Semantics struct {
	Key            Key
	AutomationID   string
	Role           woxui.AccessibilityRole
	Label          string
	Description    string
	Value          string
	Actions        []woxui.AccessibilityAction
	LiveRegion     woxui.AccessibilityLiveRegion
	Disabled       bool
	Selected       bool
	Checked        bool
	Expanded       bool
	ReadOnly       bool
	Protected      bool
	Hidden         bool
	NativeBoundary bool
	OnAction       func(action woxui.AccessibilityAction, value string) error
	Child          Widget
}

func (w Semantics) layout(ctx context, available constraints) *node {
	child := layoutBehaviorChild(ctx, available, w.Child)
	if w.Key != "" {
		child.key = w.Key
	}
	if child.kind == "" {
		child.kind = "semantics"
	}
	child.semantic = &semanticBehavior{
		automationID:   w.AutomationID,
		role:           w.Role,
		label:          w.Label,
		description:    w.Description,
		value:          w.Value,
		actions:        append([]woxui.AccessibilityAction(nil), w.Actions...),
		liveRegion:     w.LiveRegion,
		enabled:        !w.Disabled,
		selected:       w.Selected,
		checked:        w.Checked,
		expanded:       w.Expanded,
		readOnly:       w.ReadOnly,
		protected:      w.Protected,
		hidden:         w.Hidden,
		nativeBoundary: w.NativeBoundary,
		onAction:       w.OnAction,
	}
	return child
}

// Focusable lets one retained element own keyboard focus and input callbacks.
type Focusable struct {
	Key           Key
	Autofocus     bool
	Disabled      bool
	OnKeyCapture  func(event woxui.KeyEvent) bool
	OnKey         func(event woxui.KeyEvent) bool
	OnTextInput   func(event woxui.TextInputEvent) bool
	OnFocusChange func(focused bool)
	TextInput     func(bounds woxui.Rect) woxui.TextInputState
	Child         Widget
}

func (w Focusable) layout(ctx context, available constraints) *node {
	child := layoutBehaviorChild(ctx, available, w.Child)
	if w.Key != "" {
		child.key = w.Key
	}
	if child.kind == "" {
		child.kind = "focusable"
	}
	child.focus = &focusBehavior{
		autofocus:     w.Autofocus,
		disabled:      w.Disabled,
		onKeyCapture:  w.OnKeyCapture,
		onKey:         w.OnKey,
		onTextInput:   w.OnTextInput,
		onFocusChange: w.OnFocusChange,
		textInput:     w.TextInput,
	}
	return child
}

// FocusScope bounds traversal and optionally traps focus while a modal surface is visible.
type FocusScope struct {
	Key   Key
	Modal bool
	Child Widget
}

func (w FocusScope) layout(ctx context, available constraints) *node {
	child := layoutBehaviorChild(ctx, available, w.Child)
	if w.Key != "" {
		child.key = w.Key
	}
	if child.kind == "" {
		child.kind = "focus_scope"
	}
	child.scope = &focusScopeBehavior{modal: w.Modal}
	return child
}

// EditableText combines text-field semantics with focus, key, IME, and value actions.
type EditableText struct {
	Key           Key
	AutomationID  string
	Label         string
	Value         string
	ReadOnly      bool
	Protected     bool
	Autofocus     bool
	Disabled      bool
	OnKey         func(event woxui.KeyEvent) bool
	OnTextInput   func(event woxui.TextInputEvent) bool
	OnFocusChange func(focused bool)
	OnSetValue    func(value string) error
	TextInput     func(bounds woxui.Rect) woxui.TextInputState
	Child         Widget
}

func (w EditableText) layout(ctx context, available constraints) *node {
	actions := []woxui.AccessibilityAction{woxui.AccessibilityActionFocus}
	if !w.ReadOnly && !w.Disabled {
		actions = append(actions, woxui.AccessibilityActionSetValue)
	}
	child := Focusable{
		Key:           w.Key,
		Autofocus:     w.Autofocus,
		Disabled:      w.Disabled,
		OnKey:         w.OnKey,
		OnTextInput:   w.OnTextInput,
		OnFocusChange: w.OnFocusChange,
		TextInput:     w.TextInput,
		Child:         w.Child,
	}.layout(ctx, available)
	child.semantic = &semanticBehavior{
		automationID: w.AutomationID,
		role:         woxui.AccessibilityRoleTextField,
		label:        w.Label,
		value:        w.Value,
		actions:      actions,
		enabled:      !w.Disabled,
		readOnly:     w.ReadOnly,
		protected:    w.Protected,
		onAction: func(action woxui.AccessibilityAction, value string) error {
			if action == woxui.AccessibilityActionSetValue && w.OnSetValue != nil {
				return w.OnSetValue(value)
			}
			return nil
		},
	}
	return child
}

func layoutBehaviorChild(ctx context, available constraints, child Widget) *node {
	if child == nil {
		return &node{bounds: woxui.Rect{Width: available.width, Height: available.height}}
	}
	return child.layout(ctx, available)
}
