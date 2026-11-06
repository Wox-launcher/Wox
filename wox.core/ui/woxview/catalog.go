package woxview

import (
	"fmt"
	"reflect"

	widget "wox/ui/widget"
)

// nodeSpec describes one catalog entry. Adding a component means adding a props type,
// a build function in its own node_*.go file, and one entry here.
type nodeSpec struct {
	newProps func() any
	children bool
	build    func(b *builder, node *Node, id string) widget.Widget
}

var catalog map[NodeType]nodeSpec

// init populates the catalog; a package-level literal would form an initialization cycle
// because build functions recurse through the catalog.
func init() {
	catalog = map[NodeType]nodeSpec{
		NodeView:        {newProps: func() any { return &BoxProps{} }, children: true, build: buildBox},
		NodeRow:         {newProps: func() any { return &RowProps{} }, children: true, build: buildRow},
		NodeColumn:      {newProps: func() any { return &BoxProps{} }, children: true, build: buildBox},
		NodeStack:       {newProps: func() any { return &BoxProps{} }, children: true, build: buildStack},
		NodeScroll:      {newProps: func() any { return &ScrollProps{} }, children: true, build: buildScroll},
		NodeLazyList:    {newProps: func() any { return &LazyListProps{} }, children: true, build: buildLazyList},
		NodeGrid:        {newProps: func() any { return &GridProps{} }, children: true, build: buildGrid},
		NodeSpacer:      {newProps: func() any { return &SpacerProps{} }, build: buildSpacer},
		NodeText:        {newProps: func() any { return &TextProps{} }, children: true, build: buildText},
		NodeSpan:        {newProps: func() any { return &SpanProps{} }},
		NodeMarkdown:    {newProps: func() any { return &MarkdownProps{} }, build: buildMarkdown},
		NodeImage:       {newProps: func() any { return &ImageProps{} }, build: buildImage},
		NodeMorphPath:   {newProps: func() any { return &MorphPathProps{} }, build: buildMorphPath},
		NodeButton:      {newProps: func() any { return &ButtonProps{} }, build: buildButton},
		NodeIconButton:  {newProps: func() any { return &IconButtonProps{} }, build: buildIconButton},
		NodeTextField:   {newProps: func() any { return &TextFieldProps{} }, build: buildTextField},
		NodeSwitch:      {newProps: func() any { return &ToggleProps{} }, build: buildSwitch},
		NodeCheckbox:    {newProps: func() any { return &ToggleProps{} }, build: buildCheckbox},
		NodeSlider:      {newProps: func() any { return &SliderProps{} }, build: buildSlider},
		NodeProgress:    {newProps: func() any { return &ProgressProps{} }, build: buildProgress},
		NodeLoading:     {newProps: func() any { return &LoadingProps{} }, build: buildLoading},
		NodeDivider:     {newProps: func() any { return &DividerProps{} }, build: buildDivider},
		NodeForm:        {newProps: func() any { return &FormProps{} }, children: true, build: buildForm},
		NodeSubmit:      {newProps: func() any { return &ButtonProps{} }, build: buildSubmit},
		NodeSelect:      {newProps: func() any { return &ChoiceProps{} }, build: buildChoice},
		NodeList:        {newProps: func() any { return &ChoiceProps{} }, build: buildChoice},
		NodeRadioGroup:  {newProps: func() any { return &ChoiceProps{} }, build: buildChoice},
		NodeTabs:        {newProps: func() any { return &TabsProps{} }, build: buildTabs},
		NodeMultiSelect: {newProps: func() any { return &MultiSelectProps{} }, build: buildMultiSelect},
		NodeTags:        {newProps: func() any { return &MultiSelectProps{} }, build: buildMultiSelect},
		NodeTag:         {newProps: func() any { return &TagProps{} }, build: buildTag},
		NodeBadge:       {newProps: func() any { return &TagProps{} }, build: buildTag},
		NodeNumberField: {newProps: func() any { return &NumberFieldProps{} }, build: buildNumberField},
		NodeHotkey:      {newProps: func() any { return &HotkeyProps{} }, build: buildHotkey},
		NodeTable:       {newProps: func() any { return &TableProps{} }, build: buildTable},
		NodeCode:        {newProps: func() any { return &CodeProps{} }, build: buildCode},
		NodeLink:        {newProps: func() any { return &LinkProps{} }, build: buildLink},
	}
}

// handlerOwner keeps validation and callback ownership in the component's contract.
type handlerOwner interface{ handlers() []namedHandler }
type namedHandler struct {
	name    string
	handler *Handler
}

// fieldOwner describes values that participate in a form snapshot.
type fieldOwner interface {
	fieldName() string
	initialValue() any
	fieldRevision() uint64
}

// Styled is embedded by every props type so parents can read layout hints such as flex and stack offsets.
type Styled struct {
	CommonProps
	InteractionStyles
	Style Style `json:"style,omitempty"`
}

// CommonProps carries node interactions independently of its layout or inline style contract.
type CommonProps struct {
	Enter         *PresenceAnimation `json:"enter,omitempty"`
	Exit          *PresenceAnimation `json:"exit,omitempty"`
	Transition    *Transition        `json:"transition,omitempty"`
	Animation     *Animation         `json:"animation,omitempty"`
	InputRegion   bool               `json:"inputRegion,omitempty"`
	Tooltip       string             `json:"tooltip,omitempty"`
	OnHoverChange *Handler           `json:"onHoverChange,omitempty"`
	OnDoubleTap   *Handler           `json:"onDoubleTap,omitempty"`
	OnContextMenu *Handler           `json:"onContextMenu,omitempty"`
	OnFocusChange *Handler           `json:"onFocusChange,omitempty"`
	ContextMenu   []ContextMenuItem  `json:"contextMenu,omitempty"`
	Shortcuts     []Shortcut         `json:"shortcuts,omitempty"`
}

func (s CommonProps) handlers() []namedHandler {
	handlers := []namedHandler{{"onHoverChange", s.OnHoverChange}, {"onDoubleTap", s.OnDoubleTap}, {"onContextMenu", s.OnContextMenu}, {"onFocusChange", s.OnFocusChange}}
	for i, item := range s.ContextMenu {
		handlers = append(handlers, namedHandler{fmt.Sprintf("contextMenu[%d].onPress", i), item.OnPress})
	}
	for i, item := range s.Shortcuts {
		handlers = append(handlers, namedHandler{fmt.Sprintf("shortcuts[%d].onPress", i), item.OnPress})
	}
	return handlers
}

func (s Styled) styleOf() Style                { return s.Style }
func (s *Styled) setStyle(style Style)         { s.Style = style }
func (s CommonProps) commonProps() CommonProps { return s }

// prepare is the default validation for props without extra rules.
func (s *Styled) prepare() error { return s.Style.validate() }

type styled interface {
	styleOf() Style
}

// preparer validates props and precomputes immutable derived data (such as parsed Markdown)
// so the UI thread only maps nodes to widgets.
type preparer interface {
	prepare() error
}

// NewTree validates a tree assembled in Go, such as one compiled from another declarative format.
// It applies the same rules as Parse, so the result can be rendered the same way.
func NewTree(root *Node) (*Tree, error) {
	if root == nil {
		return nil, invalid("root", "woxview tree has no root")
	}
	count := 0
	handlers := map[HandlerID]struct{}{}
	if err := prepareNode(root, 1, &count, "root", handlers); err != nil {
		return nil, err
	}
	if err := validateSubmitPlacement(root, false, "root/"+string(root.Type)); err != nil {
		return nil, err
	}
	return &Tree{Version: SchemaVersion, Root: root, handlerSet: handlers}, nil
}

// validateSubmitPlacement reports the actual orphan submit node without changing its diagnostic.
func validateSubmitPlacement(node *Node, inForm bool, path string) error {
	if node.Type == NodeSubmit && !inForm {
		return invalid(path, "submit must be inside a form")
	}
	for index, child := range node.Children {
		if err := validateSubmitPlacement(child, inForm || node.Type == NodeForm, fmt.Sprintf("%s[%d]/%s", path, index, child.Type)); err != nil {
			return err
		}
	}
	return nil
}

// prepareNode validates each catalog entry and collects immutable callback ownership.
func prepareNode(node *Node, depth int, count *int, path string, handlers map[HandlerID]struct{}) error {
	if depth > MaxDepth {
		return invalid(path, "%s: woxview tree deeper than %d", path, MaxDepth)
	}
	*count++
	if *count > MaxNodes {
		return invalid(path, "woxview tree has more than %d nodes", MaxNodes)
	}
	spec, ok := catalog[node.Type]
	if !ok {
		return invalid(path+".type", "%s: unknown node type %q", path, node.Type)
	}
	path = path + "/" + string(node.Type)
	if depth == 1 && node.Type == NodeSpan {
		return invalid(path, "span must be a direct child of text")
	}
	if len(node.Key) > 256 {
		return invalid(path+".key", "%s: key longer than 256 bytes", path)
	}
	if node.Props == nil {
		node.Props = spec.newProps()
	}
	if reflect.TypeOf(node.Props) != reflect.TypeOf(spec.newProps()) {
		return invalid(path+".props", "%s: props type %T does not match node type", path, node.Props)
	}
	// A typed nil is not an omitted props object and must not reach its methods.
	if reflect.ValueOf(node.Props).IsNil() {
		return invalid(path+".props", "%s: nil props", path)
	}
	if p, ok := node.Props.(interface{ commonProps() CommonProps }); ok {
		if err := p.commonProps().Exit.validate(); err != nil {
			return invalid(path+".props.exit", "%s props: %w", path, err)
		}
		if err := p.commonProps().Enter.validate(); err != nil {
			return invalid(path+".props.enter", "%s props: %w", path, err)
		}
		if err := p.commonProps().Animation.validate(); err != nil {
			return invalid(path+".props.animation", "%s props: %w", path, err)
		}
		if err := p.commonProps().Transition.validate(node.Type); err != nil {
			return invalid(path+".props.transition", "%s props: %w", path, err)
		}
		if err := p.commonProps().validateCommon(); err != nil {
			return invalid(path+".props", "%s props: %w", path, err)
		}
	}
	if owner, ok := node.Props.(handlerOwner); ok {
		for _, h := range owner.handlers() {
			if err := validateHandler(h.name, h.handler); err != nil {
				return invalid(path+".props."+h.name, "%s props: %w", path, err)
			}
			if h.handler != nil {
				handlers[h.handler.ID] = struct{}{}
			}
		}
	}
	if p, ok := node.Props.(preparer); ok {
		if err := p.prepare(); err != nil {
			return invalid(path+".props", "%s props: %w", path, err)
		}
	}
	if p, ok := node.Props.(interface{ interactionStyles() InteractionStyles }); ok {
		if err := p.interactionStyles().validate(); err != nil {
			return invalid(path+".props", "%s props: %w", path, err)
		}
	}
	if len(node.Children) > 0 && !spec.children {
		return invalid(path+".children", "%s: %s does not accept children", path, node.Type)
	}
	keys := make(map[string]bool)
	for _, child := range node.Children {
		if child != nil && child.Key != "" {
			if keys[child.Key] {
				return invalid(path+".children", "%s: duplicate child key %q", path, child.Key)
			}
			keys[child.Key] = true
		}
	}
	for index, child := range node.Children {
		if child == nil {
			return invalid(fmt.Sprintf("%s[%d]", path, index), "%s[%d]: nil child", path, index)
		}
		if node.Type == NodeText && child.Type != NodeSpan {
			return invalid(path+".children", "%s: text only accepts span children", path)
		}
		if child.Type == NodeSpan && node.Type != NodeText {
			return invalid(fmt.Sprintf("%s[%d]/span", path, index), "span must be a direct child of text")
		}
		if node.Type == NodeLazyList && child.Key == "" {
			return invalid(fmt.Sprintf("%s[%d].key", path, index), "%s[%d]: lazy-list items require a key", path, index)
		}
		if err := prepareNode(child, depth+1, count, fmt.Sprintf("%s[%d]", path, index), handlers); err != nil {
			return err
		}
	}
	if node.Type == NodeForm {
		if err := validateForm(node); err != nil {
			return invalid(path, "%s: %w", path, err)
		}
	}
	return nil
}

func styleOf(node *Node) Style {
	if s, ok := node.Props.(styled); ok {
		return s.styleOf()
	}
	return Style{}
}
