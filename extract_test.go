package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestFindFirstSrcInTag(t *testing.T) {
	html := `<div><img src="/images/pic.jpg" alt="x"><video><source src="/videos/vid.mp4"></video></div>`
	if got := findFirstSrcInTag(html, "img"); got != "/images/pic.jpg" {
		t.Fatalf("findFirstSrcInTag(img) = %q; want %q", got, "/images/pic.jpg")
	}
	if got := findFirstSrcInTag(html, "video"); got != "/videos/vid.mp4" {
		t.Fatalf("findFirstSrcInTag(video) = %q; want %q", got, "/videos/vid.mp4")
	}
}

func TestFindBackgroundImage(t *testing.T) {
	html := `<div style="background-image: url('https://example.com/bg.png');"></div>`
	if got := findBackgroundImage(html); got != "https://example.com/bg.png" {
		t.Fatalf("findBackgroundImage = %q; want %q", got, "https://example.com/bg.png")
	}
}

func TestFindBackgroundImageVariants(t *testing.T) {
	cases := []struct {
		html string
		want string
	}{
		{`<div style="background-image: url('/img1.png')"></div>`, "/img1.png"},
		{`<div style='background-image: url("/img2.png")'></div>`, "/img2.png"},
		{`<div style="background-image:url(https://cdn/x.png)"></div>`, "https://cdn/x.png"},
	}
	for i, c := range cases {
		if got := findBackgroundImage(c.html); got != c.want {
			t.Fatalf("case %d: got %q want %q", i, got, c.want)
		}
	}
}

func TestFindBackgroundImageInStyleTag(t *testing.T) {
	html := `<style>.hero{background-image: url("/assets/hero.jpg");} :root{--featured-img: url('/assets/feat.jpg')}</style>`
	// Should find first background-image occurrence
	if got := findBackgroundImageInStyleTag(html); got != "/assets/hero.jpg" {
		t.Fatalf("findBackgroundImageInStyleTag = %q; want %q", got, "/assets/hero.jpg")
	}
	// Featured var case
	html2 := `<style>:root{--featured-img: url('/assets/feat.jpg')}</style>`
	if got := findBackgroundImageInStyleTag(html2); got != "/assets/feat.jpg" {
		t.Fatalf("findBackgroundImageInStyleTag(featured) = %q; want %q", got, "/assets/feat.jpg")
	}
}

func TestFindAmazonLargeImage(t *testing.T) {
	html := `<img id="landingImage" data-old-hires="https://amazon.img/large.jpg" src="/small.jpg">`
	if got := findAmazonLargeImage(html); got != "https://amazon.img/large.jpg" {
		t.Fatalf("findAmazonLargeImage = %q; want %q", got, "https://amazon.img/large.jpg")
	}
}

func TestExtractOpenGraphAndFindImage(t *testing.T) {
	html := `<meta property="og:title" content="Hello"><meta property="og:image" content="https://ex.com/og.png">`
	og := extractOpenGraph(html)
	var m map[string]string
	if err := json.Unmarshal([]byte(og), &m); err != nil {
		t.Fatalf("extractOpenGraph json unmarshal error: %v", err)
	}
	if m["og:image"] != "https://ex.com/og.png" {
		t.Fatalf("og:image mismatch: %q", m["og:image"])
	}
	if got := findOGImageFromJSON(og); got != "https://ex.com/og.png" {
		t.Fatalf("findOGImageFromJSON = %q; want %q", got, "https://ex.com/og.png")
	}
}

func TestFindAudioAndVideoInJSONLD(t *testing.T) {
	// audio only
	audioHTML := `<script type="application/ld+json">{"@context":"http://schema.org","@type":"AudioObject","contentUrl":"https://cdn.example.com/audio1.mp3"}</script>`
	if got := findAudioInJSONLD(audioHTML); got != "https://cdn.example.com/audio1.mp3" {
		t.Fatalf("findAudioInJSONLD = %q; want %q", got, "https://cdn.example.com/audio1.mp3")
	}
	// video only
	videoHTML := `<script type="application/ld+json">{"@context":"http://schema.org","@type":"VideoObject","contentUrl":"https://cdn.example.com/video1.mp4"}</script>`
	if got := findVideoInJSONLD(videoHTML); got != "https://cdn.example.com/video1.mp4" {
		t.Fatalf("findVideoInJSONLD = %q; want %q", got, "https://cdn.example.com/video1.mp4")
	}
	// If JSON-LD has a video contentUrl, audio must not return it
	ambig := `<script type="application/ld+json">{"@context":"http://schema.org","@type":"VideoObject","contentUrl":"https://cdn.example.com/video1.mp4"}</script>`
	if got := findAudioInJSONLD(ambig); got != "" {
		t.Fatalf("findAudioInJSONLD(ambig) = %q; want empty", got)
	}
	// array / graph case
	arrayHTML := `<script type="application/ld+json">{"@graph":[{"@type":"VideoObject","contentUrl":"https://cdn.example.com/graph_video.mp4"}]}</script>`
	if got := findVideoInJSONLD(arrayHTML); got != "https://cdn.example.com/graph_video.mp4" {
		t.Fatalf("findVideoInJSONLD(graph) = %q; want %q", got, "https://cdn.example.com/graph_video.mp4")
	}
}

func TestExtractResourceOrderAndResolve(t *testing.T) {
	article := `<video src="/article_vid.mp4"></video>`
	orig := `<video src="/orig_vid.mp4"></video>`
	base, _ := url.Parse("https://site.example/path/page.html")
	if got := extractResource(article, orig, base, "video", findVideoInJSONLD); got != "https://site.example/article_vid.mp4" {
		t.Fatalf("extractResource priority article failed: %q", got)
	}
	// If article missing, use original
	if got := extractResource("", orig, base, "video", findVideoInJSONLD); got != "https://site.example/orig_vid.mp4" {
		t.Fatalf("extractResource original fallback failed: %q", got)
	}
	// JSON-LD fallback
	jsonld := `<script type="application/ld+json">{"@type":"VideoObject","contentUrl":"/json_vid.mp4"}</script>`
	if got := extractResource("", jsonld, base, "video", findVideoInJSONLD); got != "https://site.example/json_vid.mp4" {
		t.Fatalf("extractResource jsonld fallback failed: %q", got)
	}
}

func TestPrettyPrintAndCleanText(t *testing.T) {
	html := `<div><p> Hello   world </p><p>Line\nTwo</p></div>`
	pp := PrettyPrintHTML(html)
	if !strings.Contains(pp, "<div") || !strings.Contains(pp, "Hello") {
		t.Fatalf("PrettyPrintHTML seems wrong: %q", pp)
	}
	ct := cleanUpText("  Hello\n\tworld   ")
	if ct != "Hello world" {
		t.Fatalf("cleanUpText = %q; want %q", ct, "Hello world")
	}
}
