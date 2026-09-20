//go:build windows

package selection

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
	"wox/util"

	"github.com/go-ole/go-ole"
)

var (
	CLSID_CUIAutomation = ole.NewGUID("ff48dba4-60ef-4201-aa87-54103eef594e")
	IID_IUIAutomation   = ole.NewGUID("30cbe57d-d9d0-452a-ab13-7ac5ac4825ee")
)

// Helper type alias for pattern interface generic usage
type IUnknown = ole.IUnknown

const (
	UIA_TextPatternId = 10014

	oleSFalse       = 0x00000001
	rpcEChangedMode = 0x80010106

	uiaNameLogMaxRunes = 80

	uiaControlTypeEdit     int32 = 50004
	uiaControlTypeText     int32 = 50020
	uiaControlTypeCustom   int32 = 50025
	uiaControlTypeDocument int32 = 50030
	uiaControlTypeWindow   int32 = 50032
	uiaControlTypePane     int32 = 50033
)

type IUIAutomation struct {
	IUnknown
}

type IUIAutomationElement struct {
	IUnknown
}

type IUIAutomationTreeWalker struct {
	IUnknown
}

type IUIAutomationTextPattern struct {
	IUnknown
}

type IUIAutomationTextRange struct {
	IUnknown
}

type IUIAutomationElementArray struct {
	IUnknown
}

type IUIAutomationTextRangeArray struct {
	IUnknown
}

func (v *IUIAutomation) VTable() *IUIAutomationVtbl {
	return (*IUIAutomationVtbl)(unsafe.Pointer(v.RawVTable))
}

type IUIAutomationVtbl struct {
	ole.IUnknownVtbl
	CompareElements                           uintptr
	CompareRuntimeIds                         uintptr
	GetRootElement                            uintptr
	ElementFromHandle                         uintptr
	ElementFromPoint                          uintptr
	GetFocusedElement                         uintptr
	GetRootElementBuildCache                  uintptr
	ElementFromHandleBuildCache               uintptr
	ElementFromPointBuildCache                uintptr
	GetFocusedElementBuildCache               uintptr
	CreateTreeWalker                          uintptr
	Get_ControlViewWalker                     uintptr
	Get_ContentViewWalker                     uintptr
	Get_RawViewWalker                         uintptr
	Get_RawViewCondition                      uintptr
	Get_ControlViewCondition                  uintptr
	Get_ContentViewCondition                  uintptr
	CreateCacheRequest                        uintptr
	CreateTrueCondition                       uintptr
	CreateFalseCondition                      uintptr
	CreatePropertyCondition                   uintptr
	CreatePropertyConditionEx                 uintptr
	CreateAndCondition                        uintptr
	CreateAndConditionFromArray               uintptr
	CreateAndConditionFromNativeArray         uintptr
	CreateOrCondition                         uintptr
	CreateOrConditionFromArray                uintptr
	CreateOrConditionFromNativeArray          uintptr
	CreateNotCondition                        uintptr
	AddAutomationEventHandler                 uintptr
	RemoveAutomationEventHandler              uintptr
	AddPropertyChangedEventHandlerNativeArray uintptr
	AddPropertyChangedEventHandler            uintptr
	RemovePropertyChangedEventHandler         uintptr
	AddStructureChangedEventHandler           uintptr
	RemoveStructureChangedEventHandler        uintptr
	AddFocusChangedEventHandler               uintptr
	RemoveFocusChangedEventHandler            uintptr
	RemoveAllEventHandlers                    uintptr
	IntNativeArrayToSafeArray                 uintptr
	IntSafeArrayToNativeArray                 uintptr
	RectToVariant                             uintptr
	VariantToRect                             uintptr
	SafeArrayToRectNativeArray                uintptr
	CreateProxyFactoryEntry                   uintptr
	Get_ProxyFactoryMapping                   uintptr
	GetPropertyProgrammaticName               uintptr
	GetPatternProgrammaticName                uintptr
	PollForPotentialSupportedPatterns         uintptr
	PollForPotentialSupportedProperties       uintptr
	CheckNotSupported                         uintptr
	Get_ReservedNotSupportedValue             uintptr
	Get_ReservedMixedAttributeValue           uintptr
	ElementFromIAccessible                    uintptr
	ElementFromIAccessibleBuildCache          uintptr
}

func (v *IUIAutomation) GetFocusedElement(element **IUIAutomationElement) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetFocusedElement,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(element)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (v *IUIAutomation) GetRawViewWalker(walker **IUIAutomationTreeWalker) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().Get_RawViewWalker,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(walker)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (v *IUIAutomationTreeWalker) VTable() *IUIAutomationTreeWalkerVtbl {
	return (*IUIAutomationTreeWalkerVtbl)(unsafe.Pointer(v.RawVTable))
}

type IUIAutomationTreeWalkerVtbl struct {
	ole.IUnknownVtbl
	GetParentElement uintptr
}

func (v *IUIAutomationTreeWalker) GetParentElement(element *IUIAutomationElement, parent **IUIAutomationElement) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetParentElement,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(element)),
		uintptr(unsafe.Pointer(parent)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (v *IUIAutomationElement) VTable() *IUIAutomationElementVtbl {
	return (*IUIAutomationElementVtbl)(unsafe.Pointer(v.RawVTable))
}

type IUIAutomationElementVtbl struct {
	ole.IUnknownVtbl
	SetFocus                  uintptr
	GetRuntimeId              uintptr
	FindFirst                 uintptr
	FindAll                   uintptr
	FindFirstBuildCache       uintptr
	FindAllBuildCache         uintptr
	BuildUpdatedCache         uintptr
	GetCurrentPropertyValue   uintptr
	GetCurrentPropertyValueEx uintptr
	GetCachedPropertyValue    uintptr
	GetCachedPropertyValueEx  uintptr
	GetCurrentPatternAs       uintptr
	GetCachedPatternAs        uintptr
	GetCurrentPattern         uintptr
	GetCachedPattern          uintptr
	// GetCachedParent and GetCachedChildren sit between GetCachedPattern and
	// get_CurrentProcessId in IUIAutomationElement. Skipping them shifts every
	// later getter, so get_CurrentName would call get_CurrentControlType and
	// treat the control-type id as a BSTR.
	GetCachedParent                 uintptr
	GetCachedChildren               uintptr
	Get_CurrentProcessId            uintptr
	Get_CurrentControlType          uintptr
	Get_CurrentLocalizedControlType uintptr
	Get_CurrentName                 uintptr
	Get_CurrentAcceleratorKey       uintptr
	Get_CurrentAccessKey            uintptr
	Get_CurrentHasKeyboardFocus     uintptr
	Get_CurrentIsKeyboardFocusable  uintptr
	Get_CurrentIsEnabled            uintptr
	Get_CurrentAutomationId         uintptr
	Get_CurrentClassName            uintptr
	Get_CurrentHelpText             uintptr
	Get_CurrentCulture              uintptr
	Get_CurrentIsControlElement     uintptr
	Get_CurrentIsContentElement     uintptr
	Get_CurrentIsPassword           uintptr
	Get_CurrentNativeWindowHandle   uintptr
	Get_CurrentItemType             uintptr
	Get_CurrentIsOffscreen          uintptr
	Get_CurrentOrientation          uintptr
	Get_CurrentFrameworkId          uintptr
	Get_CurrentIsRequiredForForm    uintptr
	Get_CurrentItemStatus           uintptr
	Get_CurrentBoundingRectangle    uintptr
	Get_CurrentLabeledBy            uintptr
	Get_CurrentAriaRole             uintptr
	Get_CurrentAriaProperties       uintptr
	Get_CurrentIsDataValidForForm   uintptr
	Get_CurrentControllerFor        uintptr
	Get_CurrentDescribedBy          uintptr
	Get_CurrentFlowsTo              uintptr
	Get_CurrentProviderDescription  uintptr
	Get_CachedProcessId             uintptr
	Get_CachedControlType           uintptr
	Get_CachedLocalizedControlType  uintptr
	Get_CachedName                  uintptr
	Get_CachedAcceleratorKey        uintptr
	Get_CachedAccessKey             uintptr
	Get_CachedHasKeyboardFocus      uintptr
	Get_CachedIsKeyboardFocusable   uintptr
	Get_CachedIsEnabled             uintptr
	Get_CachedAutomationId          uintptr
	Get_CachedClassName             uintptr
	Get_CachedHelpText              uintptr
	Get_CachedCulture               uintptr
	Get_CachedIsControlElement      uintptr
	Get_CachedIsContentElement      uintptr
	Get_CachedIsPassword            uintptr
	Get_CachedNativeWindowHandle    uintptr
	Get_CachedItemType              uintptr
	Get_CachedIsOffscreen           uintptr
	Get_CachedOrientation           uintptr
	Get_CachedFrameworkId           uintptr
	Get_CachedIsRequiredForForm     uintptr
	Get_CachedItemStatus            uintptr
	Get_CachedBoundingRectangle     uintptr
	Get_CachedLabeledBy             uintptr
	Get_CachedAriaRole              uintptr
	Get_CachedAriaProperties        uintptr
	Get_CachedIsDataValidForForm    uintptr
	Get_CachedControllerFor         uintptr
	Get_CachedDescribedBy           uintptr
	Get_CachedFlowsTo               uintptr
	Get_CachedProviderDescription   uintptr
	GetClickablePoint               uintptr
}

func (v *IUIAutomationElement) GetCurrentPattern(patternId int32, pattern **IUnknown) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetCurrentPattern,
		uintptr(unsafe.Pointer(v)),
		uintptr(patternId),
		uintptr(unsafe.Pointer(pattern)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (v *IUIAutomationElement) getCurrentBSTR(method uintptr) string {
	if v == nil || method == 0 {
		return ""
	}
	var bstr *uint16
	hr, _, _ := syscall.SyscallN(method, uintptr(unsafe.Pointer(v)), uintptr(unsafe.Pointer(&bstr)))
	if hr != 0 || !isPlausibleBSTR(bstr) {
		return ""
	}
	defer ole.SysFreeString((*int16)(unsafe.Pointer(bstr)))
	return ole.BstrToString(bstr)
}

// isPlausibleBSTR rejects values that COM property getters can write when the
// vtable slot is an integer out-param (for example a control type id) rather
// than a BSTR. SysStringLen reads four bytes before the pointer, so treating
// 0xC354 as a BSTR access-violates at 0xC350.
func isPlausibleBSTR(p *uint16) bool {
	return uintptr(unsafe.Pointer(p)) > 0xffff
}

func (v *IUIAutomationElement) currentControlType() int32 {
	if v == nil {
		return 0
	}
	var id int32
	hr, _, _ := syscall.SyscallN(
		v.VTable().Get_CurrentControlType,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&id)),
	)
	if hr != 0 {
		return 0
	}
	return id
}

func (v *IUIAutomationTextPattern) VTable() *IUIAutomationTextPatternVtbl {
	return (*IUIAutomationTextPatternVtbl)(unsafe.Pointer(v.RawVTable))
}

type IUIAutomationTextPatternVtbl struct {
	ole.IUnknownVtbl
	RangeFromPoint             uintptr
	RangeFromChild             uintptr
	GetSelection               uintptr
	GetVisibleRanges           uintptr
	Get_DocumentRange          uintptr
	Get_SupportedTextSelection uintptr
}

func (v *IUIAutomationTextPattern) GetSelection(ranges **IUIAutomationTextRangeArray) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetSelection,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(ranges)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

type IUIAutomationTextRangeArrayVtbl struct {
	ole.IUnknownVtbl
	Get_Length uintptr
	GetElement uintptr
}

func (v *IUIAutomationTextRangeArray) VTable() *IUIAutomationTextRangeArrayVtbl {
	return (*IUIAutomationTextRangeArrayVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IUIAutomationTextRangeArray) GetLength(length *int32) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().Get_Length,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(length)))
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

func (v *IUIAutomationTextRangeArray) GetElement(index int32, element **IUIAutomationTextRange) error {
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetElement,
		uintptr(unsafe.Pointer(v)),
		uintptr(index),
		uintptr(unsafe.Pointer(element)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	return nil
}

type IUIAutomationTextRangeVtbl struct {
	ole.IUnknownVtbl
	Clone                 uintptr
	Compare               uintptr
	CompareEndpoints      uintptr
	ExpandToEnclosingUnit uintptr
	FindAttribute         uintptr
	FindText              uintptr
	GetAttributeValue     uintptr
	GetBoundingRectangles uintptr
	GetEnclosingElement   uintptr
	GetText               uintptr
	Move                  uintptr
	MoveEndpointByUnit    uintptr
	MoveEndpointByRange   uintptr
	Select                uintptr
	AddToSelection        uintptr
	RemoveFromSelection   uintptr
	ScrollIntoView        uintptr
	GetChildren           uintptr
}

func (v *IUIAutomationTextRange) VTable() *IUIAutomationTextRangeVtbl {
	return (*IUIAutomationTextRangeVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IUIAutomationTextRange) GetText(maxLength int32, text *string) error {
	var bstr *uint16
	hr, _, _ := syscall.SyscallN(
		v.VTable().GetText,
		uintptr(unsafe.Pointer(v)),
		uintptr(maxLength),
		uintptr(unsafe.Pointer(&bstr)),
	)
	if hr != 0 {
		return ole.NewError(hr)
	}
	if bstr != nil {
		defer ole.SysFreeString((*int16)(unsafe.Pointer(bstr)))
		*text = ole.BstrToString(bstr)
	}
	return nil
}

// getSelectedFromOS tries to get the selected text using UI Automation first,
// and falls back to clipboard method if it fails.
func getSelectedFromOS(ctx context.Context) (Selection, error) {
	text, err := getSelectedByUIA(ctx)
	if err == nil && text != "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("UIA Success: %s", text))
		return Selection{
			Type: SelectionTypeText,
			Text: text,
		}, nil
	}

	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("UIA Failed: %v", err))
	} else {
		util.GetLogger().Warn(ctx, "UIA returned empty text")
	}

	return getSelectedByClipboard(ctx)
}

func getSelectedByUIA(ctx context.Context) (selectedText string, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	owned, err := initializeCOMForUIA()
	if err != nil {
		return "", err
	}
	if owned {
		defer ole.CoUninitialize()
	} else {
		util.GetLogger().Debug(ctx, "UIA using existing COM apartment (RPC_E_CHANGED_MODE)")
	}

	unknown, err := ole.CreateInstance(CLSID_CUIAutomation, IID_IUIAutomation)
	if err != nil {
		return "", fmt.Errorf("CreateInstance failed: %w", err)
	}
	defer unknown.Release()

	automation := (*IUIAutomation)(unsafe.Pointer(unknown))

	var focusedElement *IUIAutomationElement
	if err := automation.GetFocusedElement(&focusedElement); err != nil {
		return "", fmt.Errorf("GetFocusedElement failed: %w", err)
	}
	if focusedElement == nil {
		return "", fmt.Errorf("GetFocusedElement returned nil")
	}
	defer focusedElement.Release()

	// Read diagnostic properties only on failure, before releasing the COM element.
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w; focused=%s", err, describeUIAElement(focusedElement))
		}
	}()
	textPattern, depth, err := findTextPattern(automation, focusedElement)
	if err != nil {
		return "", err
	}
	defer textPattern.Release()

	var selectionRanges *IUIAutomationTextRangeArray
	if err := textPattern.GetSelection(&selectionRanges); err != nil {
		return "", fmt.Errorf("GetSelection failed: %w; depth=%d", err, depth)
	}
	if selectionRanges == nil {
		return "", fmt.Errorf("no selection ranges; depth=%d", depth)
	}
	defer selectionRanges.Release()

	var length int32
	if err := selectionRanges.GetLength(&length); err != nil {
		return "", fmt.Errorf("GetLength failed: %w; depth=%d", err, depth)
	}
	if length == 0 {
		return "", fmt.Errorf("empty selection; depth=%d ranges=0", depth)
	}

	var textRange *IUIAutomationTextRange
	if err := selectionRanges.GetElement(0, &textRange); err != nil {
		return "", fmt.Errorf("GetElement failed: %w; depth=%d ranges=%d", err, depth, length)
	}
	defer textRange.Release()

	var text string
	if err := textRange.GetText(-1, &text); err != nil {
		return "", fmt.Errorf("GetText failed: %w; depth=%d ranges=%d", err, depth, length)
	}
	if text == "" {
		return "", fmt.Errorf("empty text; depth=%d ranges=%d", depth, length)
	}

	return text, nil
}

// initializeCOMForUIA enters STA when this thread has no apartment yet.
// RPC_E_CHANGED_MODE means the thread is already MTA (for example after DirectWrite).
// CUIAutomation is free-threaded, so UIA can continue and must not CoUninitialize
// an apartment it did not create.
func initializeCOMForUIA() (owned bool, err error) {
	return comInitOwned(ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED))
}

func comInitOwned(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	oleErr, ok := err.(*ole.OleError)
	if !ok {
		return false, fmt.Errorf("CoInitializeEx failed: %w", err)
	}
	switch oleErr.Code() {
	case ole.S_OK, oleSFalse:
		return true, nil
	case rpcEChangedMode:
		return false, nil
	default:
		return false, fmt.Errorf("CoInitializeEx failed: %w", err)
	}
}

func describeUIAElement(element *IUIAutomationElement) (desc string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			desc = fmt.Sprintf("describe-panic:%v", recovered)
		}
	}()
	if element == nil {
		return "nil"
	}
	controlType := element.currentControlType()
	return fmt.Sprintf("name=%q class=%q type=%s(%d) localized=%q automationId=%q framework=%q",
		truncateUIALog(element.getCurrentBSTR(element.VTable().Get_CurrentName)),
		truncateUIALog(element.getCurrentBSTR(element.VTable().Get_CurrentClassName)),
		uiaControlTypeLabel(controlType),
		controlType,
		truncateUIALog(element.getCurrentBSTR(element.VTable().Get_CurrentLocalizedControlType)),
		truncateUIALog(element.getCurrentBSTR(element.VTable().Get_CurrentAutomationId)),
		truncateUIALog(element.getCurrentBSTR(element.VTable().Get_CurrentFrameworkId)),
	)
}

func truncateUIALog(value string) string {
	runes := []rune(value)
	if len(runes) <= uiaNameLogMaxRunes {
		return value
	}
	return string(runes[:uiaNameLogMaxRunes]) + "..."
}

func uiaControlTypeLabel(id int32) string {
	switch id {
	case uiaControlTypeEdit:
		return "Edit"
	case uiaControlTypeText:
		return "Text"
	case uiaControlTypeCustom:
		return "Custom"
	case uiaControlTypeDocument:
		return "Document"
	case uiaControlTypeWindow:
		return "Window"
	case uiaControlTypePane:
		return "Pane"
	case 0:
		return "unknown"
	default:
		return fmt.Sprintf("%d", id)
	}
}

// findTextPattern walks from the focused node to its text container because browser focus often lands on a child that does not expose TextPattern itself.
func findTextPattern(automation *IUIAutomation, focusedElement *IUIAutomationElement) (*IUIAutomationTextPattern, int, error) {
	var walker *IUIAutomationTreeWalker
	if err := automation.GetRawViewWalker(&walker); err != nil {
		return nil, 0, fmt.Errorf("GetRawViewWalker failed: %w", err)
	}
	if walker == nil {
		return nil, 0, fmt.Errorf("RawViewWalker unavailable")
	}
	defer walker.Release()

	current := focusedElement
	currentOwned := false
	depth := 0
	for current != nil {
		var patternUnknown *IUnknown
		patternErr := current.GetCurrentPattern(UIA_TextPatternId, &patternUnknown)
		if patternErr == nil && patternUnknown != nil {
			if currentOwned {
				current.Release()
			}
			return (*IUIAutomationTextPattern)(unsafe.Pointer(patternUnknown)), depth, nil
		}
		if patternUnknown != nil {
			patternUnknown.Release()
		}

		var parent *IUIAutomationElement
		if err := walker.GetParentElement(current, &parent); err != nil {
			if currentOwned {
				current.Release()
			}
			return nil, depth, fmt.Errorf("GetParentElement failed: %w", err)
		}
		if currentOwned {
			current.Release()
		}
		current = parent
		currentOwned = true
		depth++
	}

	return nil, depth, fmt.Errorf("TextPattern not supported by focused element or its ancestors")
}
