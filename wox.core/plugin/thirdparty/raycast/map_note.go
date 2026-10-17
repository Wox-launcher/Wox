package raycast

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"wox/common"
)

const rayNoteDocumentVersion = 1

type rayNoteNode struct {
	Type    string        `json:"type"`
	Text    string        `json:"text"`
	Attrs   rayNoteAttrs  `json:"attrs"`
	Marks   []rayNoteMark `json:"marks"`
	Content []rayNoteNode `json:"content"`
}

type rayNoteAttrs struct {
	Level   float64 `json:"level"`
	Kind    string  `json:"kind"`
	Checked bool    `json:"checked"`
}

type rayNoteMark struct {
	Type  string `json:"type"`
	Attrs struct {
		Href string `json:"href"`
	} `json:"attrs"`
}

// mapNoteRows keeps notes whose document has visible text. The text field is only a preview.
func mapNoteRows(payload Payload, current woxSnapshot) []mappedNote {
	var rows []mappedNote
	for index, note := range payload.Notes.Notes {
		document, ok := rayNoteDocument(note)
		if !ok {
			continue
		}
		title, body := noteSnapshotText(document)
		row := mappedNote{
			id: "note:" + strconv.Itoa(index), sourceID: strings.TrimSpace(note.ID),
			title: title, body: body, detail: trimPreview(body), document: document, selectable: true,
		}
		applyNoteState(&row, current)
		rows = append(rows, row)
	}
	return rows
}

func applyNoteState(row *mappedNote, current woxSnapshot) {
	fingerprint := noteFingerprint(row.sourceID, row.title, row.body)
	for _, note := range current.Notes {
		if note.Deleted {
			continue
		}
		if note.Fingerprint != fingerprint && (row.sourceID == "" || note.SourceID != row.sourceID) {
			continue
		}
		row.selectable = false
		if note.Body == row.body {
			row.status = statusExists
		} else {
			row.status = statusConflict
		}
		return
	}
}

func noteFingerprint(sourceID, title, body string) string {
	return snippetFingerprint(sourceID, title, body, "", false)
}

// rayNoteDocument maps one Raycast note onto the Wox note block model.
func rayNoteDocument(note Note) (common.NoteDocument, bool) {
	blocks := rayNoteBlocksFromRaw(note.RawContent)
	if len(blocks) == 0 && strings.TrimSpace(note.Text) != "" {
		blocks = []common.NoteBlock{{
			Type: common.NoteBlockParagraph, Text: strings.ReplaceAll(note.Text, "\r\n", "\n"),
		}}
	}
	blocks = ensureRayNoteTitle(blocks, note.Title)
	document := common.NoteDocument{Version: rayNoteDocumentVersion, Blocks: blocks}
	if rayNoteTitle(document) == "" {
		return common.NoteDocument{}, false
	}
	return document, true
}

func rayNoteBlocksFromRaw(raw json.RawMessage) []common.NoteBlock {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var node rayNoteNode
	if json.Unmarshal(raw, &node) != nil {
		return nil
	}
	return rayNoteBlocks(node, 0)
}

func ensureRayNoteTitle(blocks []common.NoteBlock, title string) []common.NoteBlock {
	title = strings.TrimSpace(title)
	if title == "" || len(blocks) == 0 {
		return blocks
	}
	if rayNoteTitle(common.NoteDocument{Blocks: blocks}) == title {
		return blocks
	}
	return append([]common.NoteBlock{{Type: common.NoteBlockParagraph, Text: title}}, blocks...)
}

func rayNoteBlocks(node rayNoteNode, indent int) []common.NoteBlock {
	switch node.Type {
	case "", "doc":
		return rayNoteChildren(node, indent)
	case "paragraph":
		return rayVisibleBlocks(rayInlineBlock(node, common.NoteBlockParagraph))
	case "heading":
		return rayVisibleBlocks(rayInlineBlock(node, rayHeadingType(node.Attrs.Level)))
	case "blockquote":
		var blocks []common.NoteBlock
		for _, child := range node.Content {
			for _, block := range rayNoteBlocks(child, indent) {
				if block.Type == common.NoteBlockParagraph {
					block.Type = common.NoteBlockQuote
				}
				blocks = append(blocks, block)
			}
		}
		return blocks
	case "horizontalRule":
		return []common.NoteBlock{{Type: common.NoteBlockDivider}}
	case "list":
		return rayListBlocks(node, indent)
	case "codeBlock":
		return rayVisibleBlocks(rayInlineBlock(node, common.NoteBlockCode))
	default:
		return rayNoteChildren(node, indent)
	}
}

func rayNoteChildren(node rayNoteNode, indent int) []common.NoteBlock {
	var blocks []common.NoteBlock
	for _, child := range node.Content {
		blocks = append(blocks, rayNoteBlocks(child, indent)...)
	}
	return blocks
}

func rayListBlocks(node rayNoteNode, indent int) []common.NoteBlock {
	if indent < 0 {
		indent = 0
	}
	if indent > common.NoteMaximumIndent {
		indent = common.NoteMaximumIndent
	}
	listType := common.NoteBlockBullet
	switch node.Attrs.Kind {
	case "ordered":
		listType = common.NoteBlockOrdered
	case "task":
		listType = common.NoteBlockTask
	}
	if node.Attrs.Checked && listType == common.NoteBlockBullet {
		listType = common.NoteBlockTask
	}
	var blocks []common.NoteBlock
	claimed := false
	for _, child := range node.Content {
		if child.Type == "list" {
			blocks = append(blocks, rayListBlocks(child, indent+1)...)
			continue
		}
		for _, block := range rayNoteBlocks(child, indent) {
			if !claimed && block.Type == common.NoteBlockParagraph && strings.TrimSpace(block.Text) != "" {
				block.Type = listType
				block.Indent = indent
				block.Checked = listType == common.NoteBlockTask && node.Attrs.Checked
				claimed = true
			}
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func rayVisibleBlocks(block common.NoteBlock) []common.NoteBlock {
	if block.Type != common.NoteBlockDivider && strings.TrimSpace(block.Text) == "" {
		return nil
	}
	return []common.NoteBlock{block}
}

func rayHeadingType(level float64) common.NoteBlockType {
	switch int(level) {
	case 1:
		return common.NoteBlockHeading1
	case 2:
		return common.NoteBlockHeading2
	case 0:
		return common.NoteBlockHeading1
	default:
		return common.NoteBlockHeading3
	}
}

func rayInlineBlock(node rayNoteNode, kind common.NoteBlockType) common.NoteBlock {
	block := common.NoteBlock{Type: kind}
	var walk func(rayNoteNode)
	walk = func(node rayNoteNode) {
		switch node.Type {
		case "text":
			rayAppendText(&block, node.Text, node.Marks)
		case "hardBreak":
			rayAppendText(&block, "\n", nil)
		default:
			for _, child := range node.Content {
				walk(child)
			}
		}
	}
	for _, child := range node.Content {
		walk(child)
	}
	return block
}

func rayAppendText(block *common.NoteBlock, text string, marks []rayNoteMark) {
	if text == "" {
		return
	}
	start := utf8.RuneCountInString(block.Text)
	block.Text += text
	end := start + utf8.RuneCountInString(text)
	span := common.NoteSpan{Start: start, End: end}
	styled := false
	for _, mark := range marks {
		switch mark.Type {
		case "bold", "strong":
			span.Bold = true
			styled = true
		case "italic", "em":
			span.Italic = true
			styled = true
		case "underline":
			span.Underline = true
			styled = true
		case "strike", "strikethrough":
			span.Strike = true
			styled = true
		case "code":
			span.Code = true
			styled = true
		case "link":
			if href := rayNoteLink(mark.Attrs.Href); href != "" {
				span.Link = href
				styled = true
			}
		}
	}
	if styled {
		block.Spans = append(block.Spans, span)
	}
}

func rayNoteLink(href string) string {
	parsed, err := url.Parse(strings.TrimSpace(href))
	if err != nil || parsed.Scheme == "" {
		return ""
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto":
		return parsed.String()
	default:
		return ""
	}
}

// noteSnapshotText matches the plain text a stored note reports back on the next import.
func noteSnapshotText(document common.NoteDocument) (title, body string) {
	title = rayNoteTitle(document)
	body = strings.Trim(strings.TrimPrefix(rayNotePlain(document), title), "\n")
	return title, body
}

func rayNoteTitle(document common.NoteDocument) string {
	for _, block := range document.Blocks {
		line := strings.TrimSpace(strings.SplitN(block.Text, "\n", 2)[0])
		if line != "" {
			return line
		}
	}
	return ""
}

func rayNotePlain(document common.NoteDocument) string {
	lines := make([]string, 0, len(document.Blocks))
	ordered := 1
	for _, block := range document.Blocks {
		prefix := ""
		switch block.Type {
		case common.NoteBlockBullet:
			prefix = "• "
		case common.NoteBlockOrdered:
			prefix = strconv.Itoa(ordered) + ". "
			ordered++
		case common.NoteBlockTask:
			if block.Checked {
				prefix = "☑ "
			} else {
				prefix = "☐ "
			}
		case common.NoteBlockQuote:
			prefix = "> "
		case common.NoteBlockDivider:
			lines = append(lines, "────────")
			continue
		default:
			ordered = 1
		}
		if block.Type == common.NoteBlockBullet || block.Type == common.NoteBlockOrdered || block.Type == common.NoteBlockTask {
			prefix = strings.Repeat("    ", block.Indent) + prefix
		}
		if prefix == "" && strings.TrimSpace(block.Text) == "" {
			continue
		}
		lines = append(lines, prefix+block.Text)
	}
	return strings.Join(lines, "\n")
}
