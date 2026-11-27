package websiteicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"wox/util"
)

const (
	maxIconHTMLBytes   = 512 * 1024
	maxFaviconAttempts = 6
)

type faviconCandidate struct {
	url   string
	score int
	order int
}

// fetchSiteFavicon discovers favicon links from the site's HTML head and standard origin paths.
func fetchSiteFavicon(ctx context.Context, hostURL string) (image.Image, error) {
	baseURL, err := url.Parse(hostURL)
	if err != nil {
		return nil, err
	}

	var candidates []faviconCandidate
	effectiveBase := baseURL

	htmlHeaders := map[string]string{
		"Accept":  "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Referer": hostURL + "/",
	}
	if resp, fetchErr := util.HttpOpenWithHeaders(ctx, hostURL, htmlHeaders); fetchErr == nil {
		if resp.Request != nil && resp.Request.URL != nil {
			effectiveBase = resp.Request.URL
		}
		htmlBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, maxIconHTMLBytes))
		_ = resp.Body.Close()
		if readErr == nil {
			candidates = extractFaviconCandidates(htmlBytes, effectiveBase)
		}
	}

	originBase := &url.URL{Scheme: effectiveBase.Scheme, Host: effectiveBase.Host}
	candidates = append(candidates,
		faviconCandidate{url: originBase.ResolveReference(&url.URL{Path: "/apple-touch-icon.png"}).String(), score: 70, order: len(candidates)},
		faviconCandidate{url: originBase.ResolveReference(&url.URL{Path: "/favicon.ico"}).String(), score: 20, order: len(candidates) + 1},
	)

	ranked := rankFaviconCandidates(candidates)
	var lastErr error
	attempts := 0
	for _, candidate := range ranked {
		if attempts >= maxFaviconAttempts {
			break
		}
		attempts++

		img, decodeErr := fetchAndDecodeCandidate(ctx, candidate.url)
		if decodeErr == nil && img.Bounds().Dx() > 0 && img.Bounds().Dy() > 0 {
			return img, nil
		}
		if decodeErr != nil {
			lastErr = decodeErr
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("no valid favicon found")
}

func fetchAndDecodeCandidate(ctx context.Context, candidateURL string) (image.Image, error) {
	if strings.HasPrefix(strings.ToLower(candidateURL), "data:") {
		data, err := decodeDataImageURI(candidateURL)
		if err != nil {
			return nil, err
		}
		return decodeImageBytes(data)
	}

	data, err := fetchImageBytes(ctx, candidateURL)
	if err != nil {
		return nil, err
	}
	return decodeImageBytes(data)
}

func extractFaviconCandidates(htmlBytes []byte, pageURL *url.URL) []faviconCandidate {
	var candidates []faviconCandidate
	resolveBase := pageURL
	tokenizer := html.NewTokenizer(bytes.NewReader(htmlBytes))

	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return candidates
		case html.EndTagToken:
			tagName, _ := tokenizer.TagName()
			if strings.EqualFold(string(tagName), "head") {
				return candidates
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			tagName, hasAttr := tokenizer.TagName()
			tag := strings.ToLower(string(tagName))
			if tag == "body" {
				return candidates
			}
			if !hasAttr {
				continue
			}

			attrs := readTokenAttrs(tokenizer)
			switch tag {
			case "base":
				if href := strings.TrimSpace(attrs["href"]); href != "" {
					if ref, err := url.Parse(href); err == nil {
						resolveBase = pageURL.ResolveReference(ref)
					}
				}
			case "link":
				rel := strings.ToLower(strings.TrimSpace(attrs["rel"]))
				href := strings.TrimSpace(attrs["href"])
				if href == "" || !hasIconRel(rel) {
					continue
				}
				if resolved := resolveFaviconURL(resolveBase, href); resolved != "" {
					score := scoreFaviconCandidate(rel, attrs["sizes"], attrs["type"], resolved)
					candidates = append(candidates, faviconCandidate{url: resolved, score: score, order: len(candidates)})
				}
			case "meta":
				if strings.EqualFold(strings.TrimSpace(attrs["name"]), "msapplication-TileImage") {
					if content := strings.TrimSpace(attrs["content"]); content != "" {
						if resolved := resolveFaviconURL(resolveBase, content); resolved != "" {
							candidates = append(candidates, faviconCandidate{url: resolved, score: 45, order: len(candidates)})
						}
					}
				}
			}
		}
	}
}

func readTokenAttrs(tokenizer *html.Tokenizer) map[string]string {
	attrs := map[string]string{}
	for {
		key, val, more := tokenizer.TagAttr()
		k := strings.ToLower(string(key))
		if k != "" {
			if _, exists := attrs[k]; !exists {
				attrs[k] = string(val)
			}
		}
		if !more {
			break
		}
	}
	return attrs
}

func hasIconRel(rel string) bool {
	for _, token := range strings.Fields(rel) {
		switch token {
		case "icon", "apple-touch-icon", "apple-touch-icon-precomposed", "mask-icon":
			return true
		}
	}
	return false
}

func resolveFaviconURL(baseURL *url.URL, rawHref string) string {
	if strings.HasPrefix(strings.ToLower(rawHref), "data:image/") {
		return rawHref
	}
	ref, err := url.Parse(rawHref)
	if err != nil {
		return ""
	}
	resolved := baseURL.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func scoreFaviconCandidate(rel, sizes, mimeType, resolvedURL string) int {
	score := 50
	for _, token := range strings.Fields(rel) {
		switch token {
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			if 85 > score {
				score = 85
			}
		case "icon":
			if 60 > score {
				score = 60
			}
		case "mask-icon":
			if score == 50 {
				score = 25
			}
		}
	}

	lowerType := strings.ToLower(strings.TrimSpace(mimeType))
	if strings.EqualFold(strings.TrimSpace(sizes), "any") || strings.Contains(lowerType, "svg") {
		score += 25
	} else if maxDim := parseMaxIconSize(sizes); maxDim > 0 {
		switch {
		case maxDim >= 64 && maxDim <= 256:
			score += 30
		case maxDim > 256:
			score += 15
		case maxDim >= 32:
			score += 15
		default:
			score -= 15
		}
	}

	ext := strings.ToLower(path.Ext(resolvedURL))
	if strings.Contains(lowerType, "png") || ext == ".png" {
		score += 5
	} else if strings.Contains(lowerType, "icon") || ext == ".ico" {
		score -= 5
	}
	return score
}

func parseMaxIconSize(sizes string) int {
	maxDim := 0
	for _, token := range strings.Fields(strings.ToLower(sizes)) {
		parts := strings.SplitN(token, "x", 2)
		if len(parts) != 2 {
			continue
		}
		w, errW := strconv.Atoi(parts[0])
		h, errH := strconv.Atoi(parts[1])
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			continue
		}
		dim := w
		if h > dim {
			dim = h
		}
		if dim > maxDim {
			maxDim = dim
		}
	}
	return maxDim
}

func rankFaviconCandidates(candidates []faviconCandidate) []faviconCandidate {
	dedup := make(map[string]faviconCandidate, len(candidates))
	for _, c := range candidates {
		if existing, ok := dedup[c.url]; ok {
			if c.score > existing.score {
				existing.score = c.score
				dedup[c.url] = existing
			}
			continue
		}
		dedup[c.url] = c
	}

	ranked := make([]faviconCandidate, 0, len(dedup))
	for _, c := range dedup {
		ranked = append(ranked, c)
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].order < ranked[j].order
	})
	return ranked
}

func decodeDataImageURI(rawURI string) ([]byte, error) {
	comma := strings.IndexByte(rawURI, ',')
	if comma < 0 {
		return nil, errors.New("invalid data image URI")
	}
	meta := strings.ToLower(rawURI[:comma])
	if !strings.HasPrefix(meta, "data:image/") {
		return nil, ErrNotAnImage
	}
	payload := rawURI[comma+1:]
	if strings.HasSuffix(meta, ";base64") {
		decoded, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			decoded, err = base64.RawStdEncoding.DecodeString(payload)
		}
		return decoded, err
	}
	unescaped, err := url.PathUnescape(payload)
	if err != nil {
		return nil, err
	}
	return []byte(unescaped), nil
}
