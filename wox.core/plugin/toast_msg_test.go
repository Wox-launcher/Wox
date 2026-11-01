package plugin

import (
	"context"
	"testing"
)

func TestShowToastMsgRejectsMissingContentAndCallback(t *testing.T) {
	api := &APIImpl{}
	for _, option := range []ShowToastMsgOption{
		{},
		{Message: "hello", Action: &ToastMsgAction{Name: "Open"}},
		{Message: "hello", Action: &ToastMsgAction{Action: func(context.Context, ToastMsgActionContext) {}}},
	} {
		result := api.ShowToastMsg(context.Background(), option)
		if result.Success || result.ErrMsg == "" {
			t.Fatalf("invalid option accepted: %+v", option)
		}
	}
}
