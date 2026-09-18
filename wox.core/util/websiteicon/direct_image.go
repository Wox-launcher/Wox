package websiteicon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"
	"wox/common"
	"wox/util"
	woxsvg "wox/util/svg"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

const (
	imageSniffLen       = 512
	maxDirectImageBytes = 8 << 20
)

// ErrNotAnImage is returned when the URL resolved to a non-image payload such as HTML.
var ErrNotAnImage = errors.New("url is not a direct image")

// FetchDirectImage downloads a URL only when the response itself is an image.
func FetchDirectImage(ctx context.Context, imageURL string) (common.WoxImage, error) {
	resp, err := util.HttpOpenWithHeaders(ctx, imageURL, imageRequestHeaders(imageURL))
	if err != nil {
		return common.WoxImage{}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxDirectImageBytes+1)
	peek := make([]byte, imageSniffLen)
	n, readErr := io.ReadFull(limited, peek)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return common.WoxImage{}, readErr
	}
	peek = peek[:n]
	if !isImagePayload(resp.Header.Get("Content-Type"), peek) {
		return common.WoxImage{}, ErrNotAnImage
	}
	if resp.ContentLength > maxDirectImageBytes {
		return common.WoxImage{}, fmt.Errorf("image exceeds %d bytes", maxDirectImageBytes)
	}

	rest, err := io.ReadAll(limited)
	if err != nil {
		return common.WoxImage{}, err
	}
	data := append(peek, rest...)
	if len(data) > maxDirectImageBytes {
		return common.WoxImage{}, fmt.Errorf("image exceeds %d bytes", maxDirectImageBytes)
	}

	decoded, err := decodeImageBytes(data)
	if err != nil {
		return common.WoxImage{}, err
	}
	return common.NewWoxImage(decoded)
}

// IsDirectImageURL reports whether the path looks like a pasted image address.
func IsDirectImageURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".ico", ".bmp", ".avif", ".jfif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// imageRequestHeaders mimic a browser image request so signed CDNs such as GitHub accept the GET.
func imageRequestHeaders(rawURL string) map[string]string {
	headers := map[string]string{
		"Accept": "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return headers
	}
	host := strings.ToLower(parsed.Hostname())
	if strings.HasSuffix(host, "githubusercontent.com") || strings.HasSuffix(host, "github.com") {
		headers["Referer"] = "https://github.com/"
		return headers
	}
	headers["Referer"] = parsed.Scheme + "://" + parsed.Host + "/"
	return headers
}

// isImagePayload accepts image Content-Type, sniffed raster bytes, or SVG markup.
func isImagePayload(contentType string, peek []byte) bool {
	if looksLikeSVG(peek) {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(mediaType, "image/") {
		return true
	}
	return strings.HasPrefix(http.DetectContentType(peek), "image/")
}

// looksLikeSVG detects inline SVG before HTML pages are treated as images.
func looksLikeSVG(data []byte) bool {
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if bytes.HasPrefix(trimmed, []byte("<svg")) || bytes.HasPrefix(trimmed, []byte("<SVG")) {
		return true
	}
	if bytes.HasPrefix(trimmed, []byte("<?xml")) || bytes.HasPrefix(trimmed, []byte("<?XML")) {
		lower := bytes.ToLower(trimmed)
		return bytes.Contains(lower, []byte("<svg"))
	}
	return false
}

// decodeImageBytes rasterizes SVG locally and uses the shared decoder for other formats.
func decodeImageBytes(data []byte) (image.Image, error) {
	if looksLikeSVG(data) {
		return woxsvg.Render(string(data), 32, 32)
	}
	return imaging.Decode(bytes.NewReader(data))
}
