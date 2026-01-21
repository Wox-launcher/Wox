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
)

type IUIAutomation struct {
	IUnknown
}

type IUIAutomationElement struct {
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

func (v *IUIAutomationElement) VTable() *IUIAutomationElementVtbl {
	return (*IUIAutomationElementVtbl)(unsafe.Pointer(v.RawVTable))
}

type IUIAutomationElementVtbl struct {
	ole.IUnknownVtbl
	SetFocus                        uintptr
	GetRuntimeId                    uintptr
	FindFirst                       uintptr
	FindAll                         uintptr
	FindFirstBuildCache             uintptr
	FindAllBuildCache               uintptr
	BuildUpdatedCache               uintptr
	GetCurrentPropertyValue         uintptr
	GetCurrentPropertyValueEx       uintptr
	GetCachedPropertyValue          uintptr
	GetCachedPropertyValueEx        uintptr
	GetCurrentPatternAs             uintptr
	GetCachedPatternAs              uintptr
	GetCurrentPattern               uintptr
	GetCachedPattern                uintptr
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

// GetSelected tries to get the selected text using UI Automation first,
// and falls back to clipboard method if it fails.
func GetSelected(ctx context.Context) (Selection, error) {
	// Try UI Automation first
	text, err := getSelectedByUIA()
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

	// Fallback to clipboard method
	return getSelectedByClipboard(ctx)
}

func getSelectedByUIA() (string, error) {
	// Important: Lock OS thread for COM
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		// Just in case it's already initialized with a different mode, try to proceed
		if oleErr, ok := err.(*ole.OleError); ok && oleErr.Code() != ole.S_OK && oleErr.Code() != 0x00000001 { // S_FALSE
			return "", fmt.Errorf("CoInitializeEx failed: %w", err)
		}
	}
	defer ole.CoUninitialize()

	// Create UIAutomation object
	unknown, err := ole.CreateInstance(CLSID_CUIAutomation, IID_IUIAutomation)
	if err != nil {
		return "", fmt.Errorf("CreateInstance failed: %w", err)
	}
	defer unknown.Release()

	automation := (*IUIAutomation)(unsafe.Pointer(unknown))

	// Get focused element
	var focusedElement *IUIAutomationElement
	if err := automation.GetFocusedElement(&focusedElement); err != nil {
		return "", fmt.Errorf("GetFocusedElement failed: %w", err)
	}
	defer focusedElement.Release()

	// Check if the element supports TextPattern
	var patternUnknown *IUnknown
	if err := focusedElement.GetCurrentPattern(UIA_TextPatternId, &patternUnknown); err != nil {
		return "", fmt.Errorf("GetCurrentPattern failed: %w", err)
	}
	if patternUnknown == nil {
		return "", fmt.Errorf("TextPattern not supported")
	}
	// We manually release patternUnknown as we cast it to IUIAutomationTextPattern which shares the same Release
	defer func() {
		(*ole.IUnknown)(unsafe.Pointer(patternUnknown)).Release()
	}()

	textPattern := (*IUIAutomationTextPattern)(unsafe.Pointer(patternUnknown))

	// Get Selection
	var selectionRanges *IUIAutomationTextRangeArray
	if err := textPattern.GetSelection(&selectionRanges); err != nil {
		return "", fmt.Errorf("GetSelection failed: %w", err)
	}
	if selectionRanges == nil {
		return "", fmt.Errorf("no selection ranges")
	}
	defer selectionRanges.Release()

	var length int32
	if err := selectionRanges.GetLength(&length); err != nil {
		return "", fmt.Errorf("GetLength failed: %w", err)
	}

	if length == 0 {
		return "", fmt.Errorf("empty selection")
	}

	// Get text from the first range
	var textRange *IUIAutomationTextRange
	if err := selectionRanges.GetElement(0, &textRange); err != nil {
		return "", fmt.Errorf("GetElement failed: %w", err)
	}
	defer textRange.Release()

	var text string
	// -1 for no limit
	if err := textRange.GetText(-1, &text); err != nil {
		return "", fmt.Errorf("GetText failed: %w", err)
	}

	return text, nil
}
