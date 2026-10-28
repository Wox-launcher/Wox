package launcher

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
	"wox/ai"
	"wox/util"
)

type providerTestTransport func(*http.Request) (*http.Response, error)

func (f providerTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAIProviderTestUsesUnsavedConnection(t *testing.T) {
	client := util.GetHTTPClient(context.Background())
	original := client.Transport
	client.Transport = providerTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "http://example.test/v1/models" || r.Header.Get("Authorization") != "Bearer unsaved-key" {
			t.Errorf("unexpected request: %s, auth %q", r.URL, r.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"object":"list","data":[]}`)), Request: r}, nil
	})
	defer func() { client.Transport = original }()
	config := aiProviderFromValues(map[string]string{"Name": "magpie", "Host": "http://example.test/v1", "ApiKey": "unsaved-key"})
	provider, err := ai.NewProvider(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestAIProviderTestResultAndInvalidation(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, "i18n:ui_ai_provider_test_success"},
		{context.DeadlineExceeded, "i18n:ui_ai_provider_test_timeout"},
		{&openai.Error{StatusCode: 401}, "i18n:ui_ai_provider_test_auth"},
		{&net.OpError{Op: "dial", Err: errors.New("refused")}, "i18n:ui_ai_provider_test_unreachable"},
		{errors.New("secret"), "i18n:ui_ai_provider_test_failed"},
	} {
		if got := aiProviderTestResultKey(tc.err); got != tc.want {
			t.Errorf("result = %q, want %q", got, tc.want)
		}
	}
	values := map[string]string{"Name": "magpie", "Host": "http://localhost:3425", "ApiKey": "key"}
	test := &aiProviderTestState{provider: aiProviderFromValues(values), message: "success"}
	form := &formFieldsSnapshot{values: values}
	if snapshotAIProviderTest(test, form) == nil {
		t.Fatal("matching result was hidden")
	}
	values["Host"] = "http://localhost:9999"
	if snapshotAIProviderTest(test, form) != nil {
		t.Fatal("changed configuration retained test result")
	}
	state := &formTableEditorState{providerTest: test}
	clearFormTableRowValidationLocked(state)
	if state.providerTest != nil {
		t.Fatal("edit retained test result")
	}
}
