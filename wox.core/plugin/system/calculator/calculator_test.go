package calculator

import (
	"context"
	"sync"
	"testing"
	"time"
	"wox/plugin"
)

type calculatorTestAPI struct {
	plugin.API

	mu       sync.Mutex
	settings map[string]string
}

func (a *calculatorTestAPI) GetSetting(ctx context.Context, key string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings[key]
}

func (a *calculatorTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	a.mu.Lock()
	a.settings[key] = value
	a.mu.Unlock()
}

func (a *calculatorTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {}

// TestCalculateExactRationalArithmetic verifies that intermediate division is not rounded.
func TestCalculateExactRationalArithmetic(t *testing.T) {
	tests := []struct {
		expression string
		expected   string
	}{
		{expression: "2/3*6", expected: "4"},
		{expression: "1/3*3", expected: "1"},
		{expression: "1/3", expected: "0.3333333333333333"},
	}

	for _, test := range tests {
		result, err := Calculate(test.expression, ",", ".")
		if err != nil {
			t.Fatalf("Calculate(%q) returned an error: %v", test.expression, err)
		}
		if result.String() != test.expected {
			t.Errorf("Calculate(%q) = %s, expected %s", test.expression, result, test.expected)
		}
	}
}

func TestCalculatorDisplayResultOptsIntoAutomaticQueryHistory(t *testing.T) {
	api := &calculatorTestAPI{settings: map[string]string{}}
	calculator := &CalculatorPlugin{api: api, debounceInterval: time.Hour}

	response := calculator.Query(context.Background(), plugin.Query{
		Type:     plugin.QueryTypeInput,
		RawQuery: "1+1",
		Search:   "1+1",
	})
	t.Cleanup(func() {
		if calculator.debounceTimer != nil {
			calculator.debounceTimer.Stop()
		}
	})

	if len(response.Results) != 1 {
		t.Fatalf("calculator query returned %d results", len(response.Results))
	}
	if !response.AutoRecordQueryHistory {
		t.Fatal("successful calculator query did not opt into automatic history")
	}
}

func TestCalculatorInvalidPromptDoesNotOptIntoAutomaticQueryHistory(t *testing.T) {
	api := &calculatorTestAPI{settings: map[string]string{}}
	calculator := &CalculatorPlugin{api: api, debounceInterval: time.Hour}

	response := calculator.Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		RawQuery:       "calculator invalid+",
		TriggerKeyword: "calculator",
		Search:         "invalid+",
	})

	if response.AutoRecordQueryHistory {
		t.Fatal("invalid calculator prompt opted into automatic history")
	}
}

func TestCalculatorStillRecordsPrivateHistoryAfterDebounce(t *testing.T) {
	api := &calculatorTestAPI{settings: map[string]string{}}
	calculator := &CalculatorPlugin{api: api, debounceInterval: 10 * time.Millisecond}

	response := calculator.Query(context.Background(), plugin.Query{
		Type:     plugin.QueryTypeInput,
		RawQuery: "2+2",
		Search:   "2+2",
	})
	if !response.AutoRecordQueryHistory {
		t.Fatal("successful calculator query did not opt into global automatic history")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		api.mu.Lock()
		saved := api.settings["calculatorHistories"]
		api.mu.Unlock()
		if saved != "" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("calculator private history was not saved")
}

func TestCalculatorPrivateHistorySkipsConsecutiveDuplicate(t *testing.T) {
	api := &calculatorTestAPI{settings: map[string]string{}}
	calculator := &CalculatorPlugin{
		api:              api,
		debounceInterval: 10 * time.Millisecond,
		histories:        []CalculatorHistory{{Expression: "2+2", Result: "4"}},
	}

	calculator.Query(context.Background(), plugin.Query{
		Type:     plugin.QueryTypeInput,
		RawQuery: "2+2",
		Search:   "2+2",
	})
	time.Sleep(30 * time.Millisecond)

	if len(calculator.histories) != 1 {
		t.Fatalf("calculator histories = %#v", calculator.histories)
	}
	api.mu.Lock()
	saved := api.settings["calculatorHistories"]
	api.mu.Unlock()
	if saved != "" {
		t.Fatalf("duplicate calculator history was persisted: %s", saved)
	}
}
