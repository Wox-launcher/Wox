package markdown

import (
	"bytes"
	"testing"

	"github.com/yuin/goldmark"
)

func TestPunctuationAdjacentBold(t *testing.T) {
	markdown := goldmark.New(goldmark.WithParser(NewParser()))
	tests := []struct{ source, want string }{
		{"**通常不行。**孩子", "<p><strong>通常不行。</strong>孩子</p>\n"},
		{"- **没有资格：**内地\n- **可以申请：**孩子", "<ul>\n<li><strong>没有资格：</strong>内地</li>\n<li><strong>可以申请：</strong>孩子</li>\n</ul>\n"},
		{"**第一。**接着**第二：**继续", "<p><strong>第一。</strong>接着<strong>第二：</strong>继续</p>\n"},
		{"[**标题：**链接](https://wox.one)", "<p><a href=\"https://wox.one\"><strong>标题：</strong>链接</a></p>\n"},
		{"**正常**文字 and **bold** text", "<p><strong>正常</strong>文字 and <strong>bold</strong> text</p>\n"},
		{"没有开头。**孩子", "<p>没有开头。**孩子</p>\n"},
		{"**English.**next", "<p>**English.**next</p>\n"},
		{"`**通常不行。**孩子`", "<p><code>**通常不行。**孩子</code></p>\n"},
		{"```\n**通常不行。**孩子\n```", "<pre><code>**通常不行。**孩子\n</code></pre>\n"},
		{`\**通常不行。**孩子`, "<p>**通常不行。**孩子</p>\n"},
		{"**未完成。**", "<p><strong>未完成。</strong></p>\n"},
		{"**未完成。", "<p>**未完成。</p>\n"},
		{"*文字。*孩子", "<p>*文字。*孩子</p>\n"},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			var output bytes.Buffer
			if err := markdown.Convert([]byte(tt.source), &output); err != nil {
				t.Fatal(err)
			}
			if got := output.String(); got != tt.want {
				t.Fatalf("rendered = %q, want %q", got, tt.want)
			}
		})
	}
}
