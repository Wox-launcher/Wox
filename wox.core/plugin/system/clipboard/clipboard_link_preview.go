package system

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/imagecache"

	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/webp"
	nethtml "golang.org/x/net/html"
)

const (
	clipboardOpenLinkActionID            = "clipboard.open_link"
	clipboardCopyLinkTitleActionID       = "clipboard.copy_link_title"
	clipboardCopyLinkDescriptionActionID = "clipboard.copy_link_description"
	clipboardLinkHTMLLimit               = 256 * 1024
	clipboardLinkImageLimit              = 5 * 1024 * 1024
	clipboardLinkMaxPixels               = 16_000_000
	clipboardLinkMaxRedirects            = 5
	clipboardLinkTitleRunes              = 300
	clipboardLinkDescriptionRunes        = 500
	clipboardLinkFetchTimeout            = 12 * time.Second
	// Sites omit Open Graph tags for non-browser clients, so the preview request
	// identifies itself the same way Wox's shared HTTP client does.
	clipboardLinkUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"
)

// clipboardLinkFetchSlots caps how many page previews are in flight at once.
// The channel is created on the first fetch, so a disabled or idle clipboard
// plugin does not allocate it.
var (
	clipboardLinkSlotsOnce  sync.Once
	clipboardLinkFetchSlots chan struct{}
)

// clipboardLinkSlots initializes the shared fetch limit on first use.
func clipboardLinkSlots() chan struct{} {
	clipboardLinkSlotsOnce.Do(func() {
		clipboardLinkFetchSlots = make(chan struct{}, 2)
	})
	return clipboardLinkFetchSlots
}

// clipboardLinkCacheDirectory returns the image cache folder that holds link previews.
var clipboardLinkCacheDirectory = func() string {
	return util.GetLocation().GetImageCacheDirectory()
}

// fetchClipboardLinkPreview loads one page. Tests replace it to avoid the network.
var fetchClipboardLinkPreview = fetchClipboardLinkPreviewLive

// clipboardLinkPreview is the cached preview for one normalized URL.
// imagePNG is only set while a fetch is being written out.
type clipboardLinkPreview struct {
	Title       string
	Description string
	ImagePath   string
	DirectImage bool
	imagePNG    []byte
}

type clipboardLinkPreviewCache struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	DirectImage bool   `json:"directImage,omitempty"`
	HasImage    bool   `json:"hasImage,omitempty"`
}

type clipboardLinkMeta struct {
	Title       string
	Description string
	ImageURL    string
}

// clipboardRecordLink returns the normalized URL for a text record that is a single link.
func clipboardRecordLink(record ClipboardRecord) string {
	if record.Type != string(clipboard.ClipboardTypeText) || !util.IsUrl(record.Content) {
		return ""
	}
	return util.NormalizeUrl(record.Content)
}

// clipboardVisibleLink is one link row from the latest clipboard query.
type clipboardVisibleLink struct {
	ID        string
	URL       string
	Content   string
	QueryID   string
	SessionID string
}

// beginClipboardLinkQuery invalidates pending row updates even for queries
// such as cb paste that deliberately do not expose link previews.
func (c *ClipboardPlugin) beginClipboardLinkQuery(query plugin.Query) {
	c.linkResultsMu.Lock()
	defer c.linkResultsMu.Unlock()
	c.linkQueryID = query.Id
	c.linkSessionID = query.SessionId
	c.linkResults = nil
}

// rememberClipboardLinks keeps the result ids from the latest clipboard query
// so a finished fetch can patch those rows without reading the database.
func (c *ClipboardPlugin) rememberClipboardLinks(query plugin.Query, records []ClipboardRecord) {
	links := make(map[string]clipboardVisibleLink, len(records))
	for _, record := range records {
		link := clipboardRecordLink(record)
		if link == "" || record.ID == "" {
			continue
		}
		links[record.ID] = clipboardVisibleLink{ID: record.ID, URL: link, Content: record.Content, QueryID: query.Id, SessionID: query.SessionId}
	}
	c.linkResultsMu.Lock()
	// An older backend query can finish after a newer one has already begun.
	if c.linkQueryID == query.Id && c.linkSessionID == query.SessionId {
		c.linkResults = links
	}
	c.linkResultsMu.Unlock()
}

// scheduleLinkPreviewFetch reads or downloads one link preview without blocking capture or query.
// A URL is attempted once per session. Successful metadata stays in the image cache.
func (c *ClipboardPlugin) scheduleLinkPreviewFetch(ctx context.Context, normalized string) {
	if ctx == nil || ctx.Err() != nil || normalized == "" || c.api == nil {
		return
	}
	if _, loaded := c.linkPreviewAttempted.LoadOrStore(normalized, struct{}{}); loaded {
		return
	}

	c.backgroundTasks.Add(1)
	util.Go(ctx, "clipboard link preview", func() {
		defer c.backgroundTasks.Done()
		if ctx.Err() != nil {
			return
		}
		if _, ok := loadClipboardLinkPreview(ctx, normalized); ok {
			if ctx.Err() == nil {
				c.publishClipboardLinkPreview(ctx, normalized)
			}
			return
		}

		select {
		case <-ctx.Done():
			return
		case clipboardLinkSlots() <- struct{}{}:
		}
		defer func() { <-clipboardLinkSlots() }()

		fetchCtx, cancel := context.WithTimeout(ctx, clipboardLinkFetchTimeout)
		defer cancel()
		preview, err := fetchClipboardLinkPreview(fetchCtx, normalized)
		if err != nil {
			c.api.Log(ctx, plugin.LogLevelInfo, clipboardLinkPreviewLog("skipped", normalized, err))
			return
		}
		if preview.Title == "" && preview.Description == "" && len(preview.imagePNG) == 0 && preview.ImagePath == "" {
			return
		}
		if err := saveClipboardLinkPreview(normalized, &preview); err != nil {
			c.api.Log(ctx, plugin.LogLevelWarning, clipboardLinkPreviewLog("save failed", normalized, err))
			return
		}
		preview.imagePNG = nil
		if ctx.Err() != nil {
			return
		}
		c.publishClipboardLinkPreview(ctx, normalized)
	})
}

// publishClipboardLinkPreview patches link rows that are still on screen.
// Rows from an older query are left alone, so a finished fetch does not refresh another search.
func (c *ClipboardPlugin) publishClipboardLinkPreview(ctx context.Context, normalized string) {
	preview, ok := loadClipboardLinkPreview(ctx, normalized)
	if !ok {
		return
	}
	for _, row := range c.visibleClipboardLinks(normalized) {
		c.updateVisibleLinkPreview(ctx, row, preview)
	}
}

// visibleClipboardLinks snapshots matching rows without holding a lock during API calls.
func (c *ClipboardPlugin) visibleClipboardLinks(normalized string) []clipboardVisibleLink {
	c.linkResultsMu.Lock()
	defer c.linkResultsMu.Unlock()
	rows := make([]clipboardVisibleLink, 0)
	for _, row := range c.linkResults {
		if row.URL == normalized {
			rows = append(rows, row)
		}
	}
	return rows
}

// updateVisibleLinkPreview updates only preview-owned fields within the row's query scope.
func (c *ClipboardPlugin) updateVisibleLinkPreview(ctx context.Context, row clipboardVisibleLink, preview clipboardLinkPreview) {
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, row.SessionID), row.QueryID)
	current := c.api.GetUpdatableResult(ctx, row.ID)
	if current == nil {
		return
	}
	shown := buildClipboardLinkResultPreview(row.Content, row.URL, preview, true)
	updatedPreview := plugin.WoxPreview{PreviewType: shown.Type, PreviewData: shown.Data, PreviewOverlayData: shown.OverlayData}
	if current.Preview != nil {
		updatedPreview = *current.Preview
		updatedPreview.PreviewType = shown.Type
		updatedPreview.PreviewData = shown.Data
		updatedPreview.PreviewOverlayData = shown.OverlayData
	}
	update := plugin.UpdatableResult{Id: row.ID, Preview: &updatedPreview, SubTitle: &shown.SubTitle}
	if current.Actions != nil {
		actions := c.insertClipboardLinkCopyActions(*current.Actions, shown, row.ID)
		if len(actions) != len(*current.Actions) {
			update.Actions = &actions
		}
	}
	c.api.UpdateResult(ctx, update)
}

// insertClipboardLinkCopyActions uses stable ids because cached action names are translated.
func (c *ClipboardPlugin) insertClipboardLinkCopyActions(actions []plugin.QueryResultAction, shown clipboardLinkResultPreview, recordID string) []plugin.QueryResultAction {
	var extra []plugin.QueryResultAction
	if shown.Title != "" && !clipboardActionsContain(actions, clipboardCopyLinkTitleActionID) {
		extra = append(extra, c.copyLinkTextAction(clipboardCopyLinkTitleActionID, "i18n:plugin_clipboard_copy_link_title", shown.Title))
	}
	if shown.Description != "" && !clipboardActionsContain(actions, clipboardCopyLinkDescriptionActionID) {
		extra = append(extra, c.copyLinkTextAction(clipboardCopyLinkDescriptionActionID, "i18n:plugin_clipboard_copy_link_description", shown.Description))
	}
	if len(extra) == 0 {
		return actions
	}
	// Existing action contexts belong to the core cache. Attach MRU data only
	// to new actions so the shared contexts are never mutated during an update.
	extra = attachClipboardMRUContext(extra, recordID)
	insertAt := len(actions)
	for index, action := range actions {
		if action.Id == clipboardOpenLinkActionID {
			insertAt = index + 1
			break
		}
	}
	updated := make([]plugin.QueryResultAction, 0, len(actions)+len(extra))
	updated = append(updated, actions[:insertAt]...)
	updated = append(updated, extra...)
	updated = append(updated, actions[insertAt:]...)
	return updated
}

// clipboardActionsContain identifies an action independently of its display name.
func clipboardActionsContain(actions []plugin.QueryResultAction, id string) bool {
	for _, action := range actions {
		if action.Id == id {
			return true
		}
	}
	return false
}

// clipboardLinkResultPreview is the preview surface for one link row.
type clipboardLinkResultPreview struct {
	Type        string
	Data        string
	OverlayData string
	SubTitle    string
	Title       string
	Description string
}

// buildClipboardLinkResultPreview uses a cached preview when one exists and otherwise
// keeps the clickable URL. Direct image links use the image preview surface.
func buildClipboardLinkResultPreview(rawContent string, normalized string, preview clipboardLinkPreview, ok bool) clipboardLinkResultPreview {
	result := clipboardLinkResultPreview{
		Type: plugin.WoxPreviewTypeMarkdown,
		Data: formatClipboardLinkMarkdown(rawContent, normalized),
	}
	if !ok {
		return result
	}
	result.Title = preview.Title
	result.Description = preview.Description
	if preview.Title != "" && preview.Title != strings.TrimSpace(rawContent) && preview.Title != normalized {
		result.SubTitle = preview.Title
	}
	if preview.DirectImage && preview.ImagePath != "" {
		imageValue := common.NewWoxImageAbsolutePath(preview.ImagePath).String()
		result.Type = plugin.WoxPreviewTypeImage
		result.Data = imageValue
		result.OverlayData = imageValue
		return result
	}
	result.Data = formatClipboardLinkPreviewMarkdown(rawContent, normalized, preview)
	return result
}

// copyLinkTextAction creates a metadata copy action shared by initial and live results.
func (c *ClipboardPlugin) copyLinkTextAction(id string, name string, text string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:   id,
		Name: name,
		Icon: icons.Get(icons.ActionCopy),
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			if strings.TrimSpace(text) == "" {
				return
			}
			if err := clipboard.WriteText(text); err != nil {
				c.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to copy clipboard link text: %s", err.Error()))
			}
		},
	}
}

// formatClipboardLinkPreviewMarkdown combines cached page content with the original clickable URL.
func formatClipboardLinkPreviewMarkdown(rawContent string, normalized string, preview clipboardLinkPreview) string {
	parts := make([]string, 0, 4)
	if preview.ImagePath != "" {
		parts = append(parts, clipboardLinkImageMarkdown(preview.ImagePath))
	}
	if title := strings.TrimSpace(preview.Title); title != "" {
		parts = append(parts, "**"+escapeClipboardMarkdownText(title)+"**")
	}
	if description := strings.TrimSpace(preview.Description); description != "" {
		parts = append(parts, escapeClipboardMarkdownText(description))
	}
	parts = append(parts, formatClipboardLinkMarkdown(rawContent, normalized))
	return strings.Join(parts, "\n\n")
}

// clipboardLinkImageMarkdown keeps spaces inside an angle-bracket destination so the
// preview image resolves as a local file instead of another network request.
func clipboardLinkImageMarkdown(imagePath string) string {
	slash := filepath.ToSlash(filepath.Clean(imagePath))
	slash = strings.NewReplacer("<", "%3C", ">", "%3E", "\n", "", "\r", "").Replace(slash)
	return "![](<" + slash + ">)"
}

// escapeClipboardMarkdownText keeps page metadata literal in the Markdown preview.
func escapeClipboardMarkdownText(text string) string {
	flat := strings.Join(strings.Fields(text), " ")
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, "`", "\\`", `[`, `\[`, `]`, `\]`).Replace(flat)
}

// loadClipboardLinkPreview reads only local cache files and tolerates independently expired images.
func loadClipboardLinkPreview(ctx context.Context, normalized string) (clipboardLinkPreview, bool) {
	metaPath := clipboardLinkMetaPath(normalized)
	info, err := os.Stat(metaPath)
	if err != nil {
		return clipboardLinkPreview{}, false
	}
	imagecache.Touch(ctx, metaPath, info)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return clipboardLinkPreview{}, false
	}
	var cached clipboardLinkPreviewCache
	if err := json.Unmarshal(data, &cached); err != nil {
		return clipboardLinkPreview{}, false
	}
	preview := clipboardLinkPreview{Title: cached.Title, Description: cached.Description, DirectImage: cached.DirectImage}
	if cached.HasImage {
		imagePath := clipboardLinkImagePath(normalized)
		imageInfo, statErr := os.Stat(imagePath)
		if statErr != nil {
			preview.DirectImage = false
		} else {
			imagecache.Touch(ctx, imagePath, imageInfo)
			preview.ImagePath = imagePath
		}
	}
	if preview.Title == "" && preview.Description == "" && preview.ImagePath == "" {
		return clipboardLinkPreview{}, false
	}
	return preview, true
}

// saveClipboardLinkPreview writes the image before publishing metadata that references it.
func saveClipboardLinkPreview(normalized string, preview *clipboardLinkPreview) error {
	if preview == nil {
		return fmt.Errorf("missing clipboard link preview")
	}
	dir := clipboardLinkCacheDirectory()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cached := clipboardLinkPreviewCache{Title: preview.Title, Description: preview.Description, DirectImage: preview.DirectImage}
	if len(preview.imagePNG) > 0 {
		imagePath := clipboardLinkImagePath(normalized)
		if err := writeClipboardLinkFile(imagePath, preview.imagePNG); err != nil {
			return err
		}
		preview.ImagePath = imagePath
		cached.HasImage = true
	}
	payload, err := json.Marshal(cached)
	if err != nil {
		return err
	}
	return writeClipboardLinkFile(clipboardLinkMetaPath(normalized), payload)
}

// writeClipboardLinkFile replaces a complete cache file so queries cannot read a partial write.
func writeClipboardLinkFile(dest string, payload []byte) error {
	temp := dest + ".tmp"
	if err := os.WriteFile(temp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, dest)
}

// clipboardLinkPreviewLog keeps query strings out of the log. HTTP clients include the full URL in errors.
func clipboardLinkPreviewLog(action string, normalized string, err error) string {
	message := ""
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) && urlErr.Err != nil {
			message = urlErr.Err.Error()
		} else {
			message = err.Error()
		}
		if normalized != "" {
			message = strings.ReplaceAll(message, normalized, clipboardLinkHostname(normalized))
		}
	}
	return fmt.Sprintf("clipboard link preview %s: host=%s err=%s", action, clipboardLinkHostname(normalized), message)
}

func clipboardLinkCacheKey(normalized string) string {
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func clipboardLinkMetaPath(normalized string) string {
	return path.Join(clipboardLinkCacheDirectory(), "clipboard_link_"+clipboardLinkCacheKey(normalized)+".json")
}

func clipboardLinkImagePath(normalized string) string {
	return path.Join(clipboardLinkCacheDirectory(), "clipboard_link_"+clipboardLinkCacheKey(normalized)+".png")
}

// fetchClipboardLinkPreviewLive requests the copied URL with bounded redirects and timeouts.
func fetchClipboardLinkPreviewLive(ctx context.Context, rawURL string) (clipboardLinkPreview, error) {
	normalized := util.NormalizeUrl(strings.TrimSpace(rawURL))
	if err := validateClipboardLinkURL(normalized); err != nil {
		return clipboardLinkPreview{}, err
	}
	client := newClipboardLinkHTTPClient(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return clipboardLinkPreview{}, err
	}
	req.Header.Set("User-Agent", clipboardLinkUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,image/webp,image/png,image/jpeg,image/*;q=0.8,*/*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return clipboardLinkPreview{}, err
	}
	finalURL := normalized
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if err := validateClipboardLinkURL(finalURL); err != nil {
		_ = resp.Body.Close()
		return clipboardLinkPreview{}, err
	}
	return clipboardLinkPreviewFromResponse(ctx, client, finalURL, resp)
}

// clipboardLinkPreviewFromResponse keeps HTML metadata even when its optional image fails.
func clipboardLinkPreviewFromResponse(ctx context.Context, client *http.Client, pageURL string, resp *http.Response) (clipboardLinkPreview, error) {
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return clipboardLinkPreview{}, fmt.Errorf("http status %d", resp.StatusCode)
	}
	mediaType := clipboardLinkMediaType(resp.Header.Get("Content-Type"))
	if clipboardLinkResponseIsDirectImage(pageURL, mediaType) {
		data, err := readClipboardLinkLimited(resp.Body, clipboardLinkImageLimit)
		if err != nil {
			return clipboardLinkPreview{}, err
		}
		pngBytes, err := transcodeClipboardLinkImage(data)
		if err != nil {
			return clipboardLinkPreview{}, err
		}
		return clipboardLinkPreview{DirectImage: true, imagePNG: pngBytes}, nil
	}

	data, err := readClipboardLinkLimited(resp.Body, clipboardLinkHTMLLimit)
	if err != nil {
		return clipboardLinkPreview{}, err
	}
	meta := parseClipboardLinkHTML(string(data))
	preview := clipboardLinkPreview{Title: meta.Title, Description: meta.Description}
	imageURL := resolveClipboardLinkReference(pageURL, meta.ImageURL)
	if imageURL == "" {
		return preview, nil
	}
	pngBytes, err := downloadClipboardLinkImage(ctx, client, imageURL)
	if err != nil {
		return preview, nil
	}
	preview.imagePNG = pngBytes
	return preview, nil
}

// clipboardLinkResponseIsDirectImage prefers the response type over a URL's image suffix.
func clipboardLinkResponseIsDirectImage(pageURL string, mediaType string) bool {
	if mediaType == "image/svg+xml" {
		return false
	}
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	if strings.HasPrefix(mediaType, "text/") || mediaType == "application/xhtml+xml" || mediaType == "application/xml" || mediaType == "application/json" {
		return false
	}
	return clipboardLinkImageExtension(pageURL) != ""
}

// downloadClipboardLinkImage validates the metadata URL and normalizes a bounded raster image.
func downloadClipboardLinkImage(ctx context.Context, client *http.Client, imageURL string) ([]byte, error) {
	if err := validateClipboardLinkURL(imageURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", clipboardLinkUserAgent)
	req.Header.Set("Accept", "image/webp,image/png,image/jpeg,image/*;q=0.8,*/*;q=0.5")
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http status %d", resp.StatusCode)
	}
	finalURL := imageURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if err := validateClipboardLinkURL(finalURL); err != nil {
		return nil, err
	}
	mediaType := clipboardLinkMediaType(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "text/") || mediaType == "image/svg+xml" || mediaType == "application/xhtml+xml" {
		return nil, fmt.Errorf("not an image")
	}
	data, err := readClipboardLinkLimited(resp.Body, clipboardLinkImageLimit)
	if err != nil {
		return nil, err
	}
	return transcodeClipboardLinkImage(data)
}

func readClipboardLinkLimited(reader io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, limit))
}

// transcodeClipboardLinkImage checks dimensions before allocating decoded pixels and emits PNG.
func transcodeClipboardLinkImage(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > clipboardLinkMaxPixels {
		return nil, fmt.Errorf("image dimensions %dx%d are outside the preview limit", config.Width, config.Height)
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, decoded); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

// parseClipboardLinkHTML extracts metadata with HTML tokenization so quoted
// delimiters, raw script text, and Unicode cannot corrupt tag boundaries.
func parseClipboardLinkHTML(body string) clipboardLinkMeta {
	var ogTitle, twitterTitle, htmlTitle string
	var ogDescription, twitterDescription, htmlDescription string
	var ogImage, twitterImage string

	tokenizer := nethtml.NewTokenizer(strings.NewReader(body))
	for tokenType := tokenizer.Next(); tokenType != nethtml.ErrorToken; tokenType = tokenizer.Next() {
		if tokenType != nethtml.StartTagToken && tokenType != nethtml.SelfClosingTagToken {
			continue
		}
		token := tokenizer.Token()
		if token.Data == "title" && htmlTitle == "" {
			if tokenizer.Next() == nethtml.TextToken {
				htmlTitle = cleanClipboardLinkText(string(tokenizer.Text()), clipboardLinkTitleRunes)
			}
			continue
		}
		if token.Data != "meta" {
			continue
		}
		var property, name, content string
		for _, attr := range token.Attr {
			switch attr.Key {
			case "property":
				property = attr.Val
			case "name":
				name = attr.Val
			case "content":
				content = attr.Val
			}
		}
		key := strings.ToLower(firstClipboardLinkValue(property, name))
		switch key {
		case "og:title":
			if ogTitle == "" {
				ogTitle = cleanClipboardLinkText(content, clipboardLinkTitleRunes)
			}
		case "og:description":
			if ogDescription == "" {
				ogDescription = cleanClipboardLinkText(content, clipboardLinkDescriptionRunes)
			}
		case "og:image", "og:image:secure_url":
			if ogImage == "" {
				ogImage = strings.TrimSpace(content)
			}
		case "twitter:title":
			if twitterTitle == "" {
				twitterTitle = cleanClipboardLinkText(content, clipboardLinkTitleRunes)
			}
		case "twitter:description":
			if twitterDescription == "" {
				twitterDescription = cleanClipboardLinkText(content, clipboardLinkDescriptionRunes)
			}
		case "twitter:image", "twitter:image:src":
			if twitterImage == "" {
				twitterImage = strings.TrimSpace(content)
			}
		case "description":
			if htmlDescription == "" {
				htmlDescription = cleanClipboardLinkText(content, clipboardLinkDescriptionRunes)
			}
		}
	}

	return clipboardLinkMeta{
		Title:       firstClipboardLinkValue(ogTitle, twitterTitle, htmlTitle),
		Description: firstClipboardLinkValue(ogDescription, twitterDescription, htmlDescription),
		ImageURL:    firstClipboardLinkValue(ogImage, twitterImage),
	}
}

// cleanClipboardLinkText normalizes already-decoded HTML text and caps its display length.
func cleanClipboardLinkText(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	value = strings.ToValidUTF8(value, "")
	if maxRunes > 0 && utf8.RuneCountInString(value) > maxRunes {
		runes := []rune(value)
		value = strings.TrimSpace(string(runes[:maxRunes]))
	}
	return value
}

// firstClipboardLinkValue selects the first non-empty metadata fallback.
func firstClipboardLinkValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// resolveClipboardLinkReference resolves relative metadata images against the final page URL.
func resolveClipboardLinkReference(baseRaw string, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(strings.ToLower(ref), "data:") {
		return ""
	}
	base, err := url.Parse(baseRaw)
	if err != nil {
		return ""
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	return base.ResolveReference(parsed).String()
}

// clipboardLinkImageExtension recognizes supported raster suffixes when the response type is absent.
func clipboardLinkImageExtension(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return strings.ToLower(path.Ext(parsed.Path))
	default:
		return ""
	}
}

// clipboardLinkMediaType normalizes Content-Type independently of charset or other parameters.
func clipboardLinkMediaType(header string) string {
	mediaType, _, err := mime.ParseMediaType(header)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(header))
	}
	return strings.ToLower(mediaType)
}

// validateClipboardLinkURL accepts the link the user copied, including intranet
// addresses. Only http and https are fetched, and a URL that embeds credentials is skipped.
func validateClipboardLinkURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported link scheme")
	}
	if parsed.User != nil {
		return fmt.Errorf("link credentials are not fetched")
	}
	if parsed.Hostname() == "" {
		return fmt.Errorf("link host is empty")
	}
	return nil
}

// newClipboardLinkHTTPClient retains Wox's proxy while limiting background request lifetimes.
func newClipboardLinkHTTPClient(ctx context.Context) *http.Client {
	var proxy func(*http.Request) (*url.URL, error)
	base := util.GetHTTPClient(ctx)
	if base != nil {
		if transport, ok := base.Transport.(*http.Transport); ok && transport != nil {
			proxy = transport.Proxy
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy:                 proxy,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		DisableKeepAlives:     true,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   clipboardLinkFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= clipboardLinkMaxRedirects {
				return fmt.Errorf("stopped after %d redirects", clipboardLinkMaxRedirects)
			}
			return validateClipboardLinkURL(req.URL.String())
		},
	}
}
