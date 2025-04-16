package file

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type LocateOptions struct {
	CaseInsensitive bool
	MaxResults      int
	Timeout         time.Duration
}

func LocateWithOptions(query string, opts LocateOptions) ([]string, error) {
	ctx := context.Background()
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := []string{}
	args = append(args, "-0")
	args = append(args, "-b")

	if opts.CaseInsensitive {
		args = append(args, "-i")
	}

	if opts.MaxResults > 0 {
		args = append(args, "-l", fmt.Sprintf("%d", opts.MaxResults))
	}
	args = append(args, query)

	cmd := exec.CommandContext(ctx, "locate", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("locate timed out (%s)", opts.Timeout)
		}
		return nil, fmt.Errorf("failed to run locate (%v): %s", err, out.String())
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\x00")
	var results []string
	for _, l := range lines {
		if l != "" {
			results = append(results, l)
		}
	}
	return results, nil
}

var searcher Searcher = &LinuxSearcher{}

type LinuxSearcher struct {
}

func (m *LinuxSearcher) Init(ctx context.Context) error {
	return nil
}

func (m *LinuxSearcher) Search(pattern SearchPattern) ([]SearchResult, error) {
	options :=
		LocateOptions{
			CaseInsensitive: false,
			MaxResults:      256,
			Timeout:         200000000,
		}
	results, err := LocateWithOptions(pattern.Name, options)
	if err != nil {
		return []SearchResult{}, nil
	}

	var searchResults []SearchResult
	for _, result := range results {
		fileName := filepath.Base(result)
		searchResults = append(searchResults, SearchResult{
			Name: fileName,
			Path: result,
		})
	}

	return searchResults, nil
}
