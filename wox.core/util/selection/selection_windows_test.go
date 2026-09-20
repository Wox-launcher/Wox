//go:build windows

package selection

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
)

func TestComInitOwnedTreatsChangedModeAsExistingApartment(t *testing.T) {
	owned, err := comInitOwned(nil)
	if err != nil || !owned {
		t.Fatalf("S_OK: owned=%t err=%v, want owned", owned, err)
	}

	owned, err = comInitOwned(ole.NewError(oleSFalse))
	if err != nil || !owned {
		t.Fatalf("S_FALSE: owned=%t err=%v, want owned", owned, err)
	}

	owned, err = comInitOwned(ole.NewError(rpcEChangedMode))
	if err != nil {
		t.Fatalf("RPC_E_CHANGED_MODE: err=%v, want proceed", err)
	}
	if owned {
		t.Fatal("RPC_E_CHANGED_MODE must not CoUninitialize an apartment this call did not create")
	}

	owned, err = comInitOwned(ole.NewError(ole.E_FAIL))
	if err == nil || owned {
		t.Fatalf("E_FAIL: owned=%t err=%v, want failure", owned, err)
	}

	owned, err = comInitOwned(errors.New("not ole"))
	if err == nil || owned {
		t.Fatalf("plain error: owned=%t err=%v, want failure", owned, err)
	}
}

func TestUIAElementVtblKeepsCachedChildrenBeforeCurrentName(t *testing.T) {
	var vt IUIAutomationElementVtbl
	if unsafe.Offsetof(vt.GetCachedParent) >= unsafe.Offsetof(vt.Get_CurrentProcessId) {
		t.Fatal("GetCachedParent must precede get_CurrentProcessId")
	}
	if unsafe.Offsetof(vt.GetCachedChildren) >= unsafe.Offsetof(vt.Get_CurrentProcessId) {
		t.Fatal("GetCachedChildren must precede get_CurrentProcessId")
	}
	if unsafe.Offsetof(vt.Get_CurrentName) <= unsafe.Offsetof(vt.GetCachedChildren) {
		t.Fatal("get_CurrentName must follow GetCachedChildren")
	}
}

func TestIsPlausibleBSTRRejectsControlTypeIDs(t *testing.T) {
	if isPlausibleBSTR(nil) {
		t.Fatal("nil must not look like a BSTR")
	}
	controlTypeAsPointer := (*uint16)(unsafe.Pointer(uintptr(uiaControlTypeEdit)))
	if isPlausibleBSTR(controlTypeAsPointer) {
		t.Fatalf("control type %d must not look like a BSTR", uiaControlTypeEdit)
	}
}

func TestDescribeUIAElementNil(t *testing.T) {
	if got := describeUIAElement(nil); got != "nil" {
		t.Fatalf("nil element = %q", got)
	}
}

func TestUIAControlTypeLabel(t *testing.T) {
	if got := uiaControlTypeLabel(uiaControlTypeDocument); got != "Document" {
		t.Fatalf("document label = %q", got)
	}
	if got := uiaControlTypeLabel(0); got != "unknown" {
		t.Fatalf("zero label = %q", got)
	}
	if got := uiaControlTypeLabel(50123); got != "50123" {
		t.Fatalf("unknown id label = %q", got)
	}
}

func TestTruncateUIALog(t *testing.T) {
	if got := truncateUIALog("short"); got != "short" {
		t.Fatalf("short = %q", got)
	}

	runes := make([]rune, uiaNameLogMaxRunes+5)
	for i := range runes {
		runes[i] = '选'
	}
	got := truncateUIALog(string(runes))
	want := string(runes[:uiaNameLogMaxRunes]) + "..."
	if got != want {
		t.Fatalf("truncated = %q, want %q", got, want)
	}
}
