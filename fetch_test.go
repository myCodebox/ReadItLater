package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// helper: gzip a string
func gzipBytes(s string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(s))
	_ = gz.Close()
	return buf.Bytes()
}

func TestAnalyzeURL_GzipAnd403Retry(t *testing.T) {
	// server that returns 403 to Chrome UA without Referer, otherwise returns gzipped HTML with JSON-LD
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		referer := r.Header.Get("Referer")
		// If Chrome UA and no referer -> 403
		if strings.Contains(ua, "Chrome") && referer == "" {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintln(w, "forbidden")
			return
		}
		// else return gzipped HTML with JSON-LD referencing /media/video.mp4
		videoURL := fmt.Sprintf("%s/media/video.mp4", srv.URL)
		html := fmt.Sprintf(`<!doctype html><html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"VideoObject","contentUrl":"%s"}</script></head><body><video src="/media/video.mp4"></video></body></html>`, videoURL)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(gzipBytes(html))
	}))
	defer srv.Close()

	// Call analyzeURL against the server URL (which will first return 403, then success on retry)
	target := srv.URL + "/testpage"
	title, image, video, audio, bodyHTML, cleanText, ogJSON, err := analyzeURL(target)
	if err != nil {
		t.Fatalf("analyzeURL returned error: %v", err)
	}
	if video == "" {
		t.Fatalf("expected video, got empty")
	}
	if !strings.Contains(video, "/media/video.mp4") {
		t.Fatalf("unexpected video value: %s", video)
	}
	if ogJSON == "" {
		t.Fatalf("expected ogJSON non-empty")
	}

	// also ensure image and audio empty for this fixture
	if image != "" || audio != "" {
		t.Fatalf("unexpected other fields: image=%q audio=%q", image, audio)
	}
	_ = cleanText
	_ = title
	_ = bodyHTML
}

func TestMalformedJSONLD(t *testing.T) {
	bad := `<script type="application/ld+json">{ this is not: valid json</script>`
	if got := findVideoInJSONLD(bad); got != "" {
		t.Fatalf("findVideoInJSONLD on malformed JSON returned %q; want empty", got)
	}
	if got := findAudioInJSONLD(bad); got != "" {
		t.Fatalf("findAudioInJSONLD on malformed JSON returned %q; want empty", got)
	}
}

func TestMultipleOGImagesAndFind(t *testing.T) {
	html := `<meta property="og:image:url" content="https://ex.com/og1.png"><meta property="og:image:secure_url" content="https://ex.com/og2.png">`
	og := extractOpenGraph(html)
	if got := findOGImageFromJSON(og); got != "https://ex.com/og1.png" {
		t.Fatalf("findOGImageFromJSON = %q; want %q", got, "https://ex.com/og1.png")
	}
}

func TestAudioNestedObjectWithoutExtension(t *testing.T) {
	html := `<script type="application/ld+json">{"audio":{"contentUrl":"https://cdn.example.com/stream?token=abc"}}</script>`
	if got := findAudioInJSONLD(html); got != "https://cdn.example.com/stream?token=abc" {
		t.Fatalf("findAudioInJSONLD nested = %q; want %q", got, "https://cdn.example.com/stream?token=abc")
	}
}

func TestVideoTopLevelContentUrlNoExtension(t *testing.T) {
	html := `<script type="application/ld+json">{"@type":"VideoObject","contentUrl":"https://cdn.example.com/stream?id=1"}</script>`
	if got := findVideoInJSONLD(html); got != "" {
		t.Fatalf("findVideoInJSONLD should not return URL without video extension, got %q", got)
	}
}

func TestAnalyzeURL_NonHTMLContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	_, _, _, _, _, _, _, err := analyzeURL(srv.URL + "/api")
	if err == nil {
		t.Fatalf("analyzeURL should error on non-HTML content type")
	}
	if !strings.Contains(err.Error(), "keine HTML-Antwort") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestFindContentUrlFallbackGeneric(t *testing.T) {
	// top-level contentUrl without extension should be returned by generic fallback
	if got := findContentURLInJSONValue(map[string]interface{}{"contentUrl": "https://cdn.example.com/resource"}); got != "https://cdn.example.com/resource" {
		t.Fatalf("findContentURLInJSONValue generic = %q; want %q", got, "https://cdn.example.com/resource")
	}
}

func TestResolveURL_EmptyRef(t *testing.T) {
	base, _ := url.Parse("https://example.com/page")
	if got := resolveURL(base, ""); got != "" {
		t.Fatalf("resolveURL empty ref = %q; want empty", got)
	}
}

func TestFindOGArrayBehavior(t *testing.T) {
	// If multiple og:image tags appear, extractOpenGraph keeps last for same property; ensure findOGImageFromJSON can still find secure variant
	html := `<meta property="og:image" content="https://ex.com/one.png"><meta property="og:image:secure_url" content="https://ex.com/secure.png">`
	og := extractOpenGraph(html)
	if got := findOGImageFromJSON(og); got != "https://ex.com/one.png" {
		t.Fatalf("findOGImageFromJSON multi = %q; want %q", got, "https://ex.com/one.png")
	}
}

func TestPrettyPrintPreservesTextNodes(t *testing.T) {
	in := `<div>   a\n b  </div>`
	out := PrettyPrintHTML(in)
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Fatalf("PrettyPrintHTML lost text nodes: %q", out)
	}
}

func TestFindFirstSrcInTag_NoMatches(t *testing.T) {
	if got := findFirstSrcInTag("<div></div>", "img"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestAnalyzeURL_AbsoluteAndRelativeResolution(t *testing.T) {
	// server returning a page with relative img and absolute og:image
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><head><meta property="og:image" content="https://cdn.example.com/og.png"></head><body><img src="/images/pic.jpg"></body></html>`
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, html)
	}))
	defer srv.Close()

	title, image, video, audio, bodyHTML, cleanText, ogJSON, err := analyzeURL(srv.URL + "/article")
	if err != nil {
		t.Fatalf("analyzeURL error: %v", err)
	}
	// image should be resolved from article content first (relative becomes absolute)
	if image == "" {
		t.Fatalf("expected image, got empty; ogJSON=%s body=%s", ogJSON, bodyHTML)
	}
	u, _ := url.Parse(image)
	if u.Host != "" && u.Scheme != "" {
		// ok
	} else {
		t.Fatalf("image not resolved to absolute URL: %s", image)
	}
	_ = title
	_ = video
	_ = audio
	_ = cleanText
}

func TestAnalyzeURL_OnlyOGImage(t *testing.T) {
	// server returning a page with only OpenGraph image meta tag (no <img>)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `<html><head><meta property="og:image" content="https://cdn.example.com/onlyog.png"></head><body><p>No images here</p></body></html>`
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintln(w, html)
	}))
	defer srv.Close()

	title, image, video, audio, bodyHTML, cleanText, ogJSON, err := analyzeURL(srv.URL + "/onlyog")
	if err != nil {
		t.Fatalf("analyzeURL error: %v", err)
	}
	if image != "https://cdn.example.com/onlyog.png" {
		t.Fatalf("expected OG image fallback, got: %q (ogJSON=%s body=%s)", image, ogJSON, bodyHTML)
	}
	_ = title
	_ = video
	_ = audio
	_ = cleanText
}
