// Package smokereport carries optional measurements from feature cases to the
// native smoke summary. Reports never decide a case's pass/fail status.
package smokereport

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const Prefix = "SMOKE_REPORT "

type Metric struct {
	Name        string   `json:"name"`
	Value       *float64 `json:"value"`
	Unit        string   `json:"unit,omitempty"`
	Unavailable string   `json:"unavailable,omitempty"`
}

type Report struct {
	Title   string   `json:"title"`
	Scope   string   `json:"scope"`
	Metrics []Metric `json:"metrics"`
}

func (r *Report) Add(name string, value float64, unit string) {
	r.Metrics = append(r.Metrics, Metric{Name: name, Value: &value, Unit: unit})
}

func (r *Report) Missing(name string, err error) {
	r.Metrics = append(r.Metrics, Metric{Name: name, Unavailable: err.Error()})
}

// Line emits one bounded structured record that survives go test's log prefix.
func (r Report) Line() string {
	data, err := json.Marshal(r)
	if err != nil {
		return Prefix + `{"title":"Invalid measurement report","metrics":[]}`
	}
	return Prefix + string(data)
}

func Parse(line string) (Report, bool) {
	index := strings.Index(line, Prefix)
	if index < 0 {
		return Report{}, false
	}
	var report Report
	if err := json.Unmarshal([]byte(strings.TrimSpace(line[index+len(Prefix):])), &report); err != nil || report.Title == "" {
		return Report{}, false
	}
	return report, true
}

// Write displays unavailable metrics explicitly, never as zero-valued samples.
func (r Report) Write(destination io.Writer) {
	fmt.Fprintf(destination, "\n%s (report only)\n", r.Title)
	if r.Scope != "" {
		fmt.Fprintln(destination, r.Scope)
	}
	for _, metric := range r.Metrics {
		if metric.Value == nil {
			fmt.Fprintf(destination, "%s: unavailable (%s)\n", metric.Name, metric.Unavailable)
		} else {
			fmt.Fprintf(destination, "%s: %.3f %s\n", metric.Name, *metric.Value, metric.Unit)
		}
	}
}
