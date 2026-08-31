package launcher

import "testing"

func TestRuntimeStatusRefreshableOnlyForMissingPythonAndNode(t *testing.T) {
	cases := []struct {
		runtime    string
		statusCode string
		want       bool
	}{
		{runtime: "PYTHON", statusCode: "executable_missing", want: true},
		{runtime: "NODEJS", statusCode: "unsupported_version", want: true},
		{runtime: "PYTHON", statusCode: "start_failed", want: true},
		{runtime: "PYTHON", statusCode: "running", want: false},
		{runtime: "SCRIPT", statusCode: "executable_missing", want: false},
	}
	for _, testCase := range cases {
		got := runtimeStatusRefreshable(runtimeStatus{Runtime: testCase.runtime, StatusCode: testCase.statusCode})
		if got != testCase.want {
			t.Fatalf("runtimeStatusRefreshable(%s, %s) = %v, want %v", testCase.runtime, testCase.statusCode, got, testCase.want)
		}
	}
}
