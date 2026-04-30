package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-shiori/go-readability"
	"golang.org/x/net/html"
)

// Konfiguration
const (
	defaultAddr = "127.0.0.1:8080"
)

var (
	// http client mit Default-Timeout
	httpClient = &http.Client{}

	// vorkompilierte Regexen
	bgRe       = regexp.MustCompile(`background-image\s*:\s*url\(['"]?([^'\")]+)['"]?\)`)
	featuredRe = regexp.MustCompile(`--featured-img\s*:\s*url\(['"]?([^'\")]+)['"]?\)`)

	pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
		"hasPrefix": strings.HasPrefix,
	}).Parse(pageTemplate))
)

const pageTemplate = `
<!DOCTYPE html>
<html lang="de">
<head>
	<meta charset="UTF-8">
	<title>ReadItLater Go Analyzer</title>
	<style>
		body { font-family: sans-serif; margin: 2em; }
		img { max-width: 400px; display: block; margin-bottom: 1em; }
		.result { margin-top: 2em; padding: 1em; border: 1px solid #ccc; background: #fafafa; }
		textarea { width: 100%; height: 150px; }
		@media (prefers-color-scheme: dark) {
			body { background: #181a1b; color: #e8e6e3; }
			.result { background: #232629; border-color: #444; }
			input, textarea {
				background: #232629;
				color: #e8e6e3;
				border: 1px solid #444;
			}
			button {
				background: #444;
				color: #e8e6e3;
				border: 1px solid #666;
			}
		}
	</style>
</head>
<body>
	<h1>ReadItLater Go Analyzer</h1>
	<form method="get" action="/">
		<label for="url">URL zum Analysieren:</label>
		<input type="text" id="url" name="url" value="{{.URL}}" style="width:60%;" required>
		<button type="submit">Analysieren</button>
	</form>
	{{if .Analyzed}}
	<div class="result">
		<h2>{{.Title}}</h2>
		{{if .Image}}
			<img src="{{.Image}}" alt="Artikelbild">
		{{end}}
		{{if .Video}}
			{{if (hasPrefix .Video "blob:")}}
				<div style="max-width:400px;display:block;margin-bottom:1em;">
					<strong>Hinweis:</strong> <span style="color:orange;">blob:-URLs funktionieren nur im Browser-Kontext und können nicht direkt heruntergeladen werden.</span>
					<br>
					<code>{{.Video}}</code>
				</div>
			{{else}}
				<div id="video-container" style="max-width:400px;display:block;margin-bottom:1em;">
					<video id="video" controls style="width:100%;">
						<source src="{{.Video}}">
						Dein Browser unterstützt das Video-Tag nicht.
					</video>
				</div>
				<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
				<script>
				(function() {
					var videoSrc = "{{.Video}}";
					var video = document.getElementById('video');
					// Falls HLS (.m3u8) vorhanden, versuche Hls.js oder native Unterstützung
					if (videoSrc && videoSrc.endsWith('.m3u8')) {
						if (window.Hls && Hls.isSupported()) {
							var hls = new Hls();
							hls.loadSource(videoSrc);
							hls.attachMedia(video);
						} else if (video.canPlayType('application/vnd.apple.mpegurl')) {
							video.src = videoSrc;
						} else {
							document.getElementById('video-container').innerHTML = '<div style="color:red;">Dein Browser unterstützt dieses Videoformat nicht direkt. Bitte verwende Safari oder installiere eine HLS-Erweiterung.</div>';
						}
					} else {
						// Für normale MP4/WebM/etc. setzen wir die Quelle direkt
						try {
							video.src = videoSrc;
						} catch (e) {
							console.error('Fehler beim Setzen der Videoquelle', e);
						}
					}
				})();
				</script>
			{{end}}
		{{end}}
		{{if .Audio}}
			{{if (hasPrefix .Audio "blob:")}}
				<div style="max-width:400px;display:block;margin-bottom:1em;">
					<strong>Hinweis:</strong> <span style="color:orange;">blob:-URLs funktionieren nur im Browser-Kontext und können nicht direkt heruntergeladen werden.</span>
					<br>
					<code>{{.Audio}}</code>
				</div>
			{{else}}
				<audio src="{{.Audio}}" controls style="max-width:400px;display:block;margin-bottom:1em;"></audio>
			{{end}}
		{{end}}
		<h3>Bereinigter Text:</h3>
		<textarea readonly>{{.CleanText}}</textarea>
		<h3>Body HTML:</h3>
		<textarea readonly>{{.BodyHTML}}</textarea>
		<h3>Open Graph Daten:</h3>
		<textarea readonly>{{.OpenGraph}}</textarea>
		<h3>JSON:</h3>
		<textarea readonly>{{.JSON}}</textarea>
	</div>
	{{end}}
</body>
</html>
`

type PageData struct {
	URL       string
	Title     string
	Image     string
	Video     string
	Audio     string
	BodyHTML  string
	CleanText string
	OpenGraph string
	JSON      string
	Analyzed  bool
}

func main() {
	addr := getServerAddr()
	http.HandleFunc("/", handler)
	fmt.Printf("Server läuft auf http://%s\n", addr)
	// Default http client timeout
	httpClient.Timeout = 15 * time.Second
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getServerAddr() string {
	if val := os.Getenv("READITLATER_ADDR"); val != "" {
		return val
	}
	return defaultAddr
}

func handler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}
	urlStr := r.URL.Query().Get("url")
	if urlStr != "" {
		if decoded, err := url.QueryUnescape(urlStr); err == nil {
			urlStr = decoded
		}
		title, image, video, audio, bodyHTML, cleanText, ogJSONString, err := analyzeURL(urlStr)
		data.URL = urlStr
		data.Analyzed = true
		if err != nil {
			http.Error(w, "Fehler: "+err.Error(), http.StatusBadRequest)
			return
		}
		data.Title = title
		data.Image = image
		data.Video = video
		data.Audio = audio
		data.BodyHTML = bodyHTML
		data.CleanText = cleanText
		data.OpenGraph = ogJSONString
		data.JSON = buildResultJSON(title, image, video, audio, cleanText, bodyHTML, ogJSONString)
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		log.Printf("Template-Fehler: %v", err)
		http.Error(w, "Interner Fehler beim Rendern", http.StatusInternalServerError)
	}
}

func analyzeURL(urlStr string) (title, image, video, audio string, bodyHTML string, cleanText string, ogJSONString string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Fehler beim Erstellen der Anfrage: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Fehler beim Laden der Seite: %w", err)
	}
	// Falls 403, versuchen wir es nochmal mit einem Referer-Header und leicht abgewandeltem User-Agent
	if resp.StatusCode == http.StatusForbidden {
		// schließe ursprünglichen Body bevor wir neu anfragen
		resp.Body.Close()
		if parsed, perr := url.Parse(urlStr); perr == nil {
			altReq, _ := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
			// kopiere einige Header
			altReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:117.0) Gecko/20100101 Firefox/117.0")
			altReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			altReq.Header.Set("Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
			altReq.Header.Set("Referer", parsed.Scheme+"://"+parsed.Host+"/")
			altResp, aerr := httpClient.Do(altReq)
			if aerr == nil {
				resp = altResp
				// defer close for the response we'll use further down
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
					// success with alternative headers; continue normally
				} else {
					// read small snippet for debugging
					snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
					return "", "", "", "", "", "", "", fmt.Errorf("ungültiger Statuscode nach Retry: %d - %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
				}
			} else {
				return "", "", "", "", "", "", "", fmt.Errorf("Fehler beim Laden der Seite (Retry): %w", aerr)
			}
		} else {
			return "", "", "", "", "", "", "", fmt.Errorf("ungültiger Statuscode: %d", resp.StatusCode)
		}
	} else {
		// Wenn ursprüngliche Antwort kein 403 war, stelle sicher dass wir sie später schließen
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return "", "", "", "", "", "", "", fmt.Errorf("ungültiger Statuscode: %d", resp.StatusCode)
		}
	}

	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "text/html") {
		// Nicht zwingend ein Fehler, aber meistens kein HTML zum Parsen
		return "", "", "", "", "", "", "", fmt.Errorf("keine HTML-Antwort (Content-Type: %s)", ct)
	}

	// handle gzip-encoded responses
	var reader io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gz, gzErr := gzip.NewReader(resp.Body)
		if gzErr == nil {
			defer gz.Close()
			reader = gz
		}
	}

	// Begrenze gelesene Daten, um OOM bei großen Antworten zu vermeiden
	const maxBytes = 2 << 20 // 2 MiB
	limitedReader := io.LimitReader(reader, maxBytes)
	originalHTMLBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Fehler beim Lesen des HTML: %w", err)
	}
	originalHTML := string(originalHTMLBytes)

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Ungültige URL: %w", err)
	}

	article, err := readability.FromReader(strings.NewReader(originalHTML), parsedURL)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Readability-Fehler: %w", err)
	}
	title = article.Title
	image = extractImage(article.Content, originalHTML, parsedURL)
	ogJSONString = extractOpenGraph(originalHTML)
	// og:image als Fallback nutzen, falls kein Bild gefunden wurde
	if image == "" {
		if ogImg := findOGImageFromJSON(ogJSONString); ogImg != "" {
			image = ogImg
		}
	}
	video = extractResource(article.Content, originalHTML, parsedURL, "video", findVideoInJSONLD)
	audio = extractResource(article.Content, originalHTML, parsedURL, "audio", findAudioInJSONLD)
	formattedHTML := PrettyPrintHTML(article.Content)
	bodyHTML = formattedHTML
	cleanText = cleanUpText(article.TextContent)
	return title, image, video, audio, bodyHTML, cleanText, ogJSONString, nil
}

// Anpassung: baseURL als *url.URL für robusteren Host-Vergleich / ResolveReference
func extractImage(articleHTML, originalHTML string, base *url.URL) string {
	// 1. Amazon-spezifisch: Suche nach data-old-hires für große Produktbilder
	if base != nil && strings.Contains(base.Hostname(), "amazon") {
		if largeImg := findAmazonLargeImage(originalHTML); largeImg != "" {
			return resolveURL(base, largeImg)
		}
	}
	// 2. Etsy-spezifisch: OG-Image bevorzugen
	if base != nil && strings.Contains(base.Hostname(), "etsy") {
		ogJSONString := extractOpenGraph(originalHTML)
		if ogImg := findOGImageFromJSON(ogJSONString); ogImg != "" {
			return resolveURL(base, ogImg)
		}
	}
	// 3. Aus Article-Content
	if img := findFirstSrcInTag(articleHTML, "img"); img != "" {
		return resolveURL(base, img)
	}
	// 4. Aus Original-HTML
	if img := findFirstSrcInTag(originalHTML, "img"); img != "" {
		return resolveURL(base, img)
	}
	// 5. Background-Image in Style-Attributen
	if img := findBackgroundImage(originalHTML); img != "" {
		return resolveURL(base, img)
	}
	// 6. Background-Image in <style>-Tags
	if img := findBackgroundImageInStyleTag(originalHTML); img != "" {
		return resolveURL(base, img)
	}
	// Kein Bild gefunden, Rückgabe leer (og:image wird später als Fallback genutzt)
	return ""
}

// Generische Funktion für das Finden von src-Attributen in Tags (img, audio, video)
func findFirstSrcInTag(htmlStr, tag string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var src string
	doc.Find(tag).EachWithBreak(func(i int, s *goquery.Selection) bool {
		if val, exists := s.Attr("src"); exists && val != "" {
			src = val
			return false
		}
		s.Find("source").EachWithBreak(func(j int, ss *goquery.Selection) bool {
			if val, exists := ss.Attr("src"); exists && val != "" {
				src = val
				return false
			}
			return true
		})
		return src == ""
	})
	return src
}

// Amazon-spezifische Bildersuche: Suche nach data-old-hires für große Produktbilder
func findAmazonLargeImage(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	img := doc.Find("#landingImage")
	if img.Length() > 0 {
		if hires, exists := img.Attr("data-old-hires"); exists && hires != "" {
			return hires
		}
		// Fallback: src-Attribut
		if src, exists := img.Attr("src"); exists && src != "" {
			return src
		}
	}
	return ""
}

func findBackgroundImage(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var bgImg string
	doc.Find("[style]").EachWithBreak(func(i int, s *goquery.Selection) bool {
		style, _ := s.Attr("style")
		if strings.Contains(style, "background-image") {
			matches := bgRe.FindStringSubmatch(style)
			if len(matches) > 1 {
				bgImg = matches[1]
				return false
			}
		}
		return true
	})
	return bgImg
}

func findBackgroundImageInStyleTag(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var image string
	found := false
	doc.Find("style").EachWithBreak(func(i int, s *goquery.Selection) bool {
		css := s.Text()
		matchesBG := bgRe.FindStringSubmatch(css)
		if len(matchesBG) > 1 {
			image = matchesBG[1]
			found = true
			return false
		}
		matchesVar := featuredRe.FindStringSubmatch(css)
		if len(matchesVar) > 1 {
			image = matchesVar[1]
			found = true
			return false
		}
		return true
	})
	if found && image != "" {
		return image
	}
	return ""
}

// Anpassung: base als *url.URL
func extractResource(articleHTML, originalHTML string, base *url.URL, tag string, jsonldFunc func(string) string) string {
	// 1. Aus Article-Content
	if src := findFirstSrcInTag(articleHTML, tag); src != "" {
		return resolveURL(base, src)
	}
	// 2. Aus Original-HTML
	if src := findFirstSrcInTag(originalHTML, tag); src != "" {
		return resolveURL(base, src)
	}
	// 3. Aus JSON-LD
	if src := jsonldFunc(originalHTML); src != "" {
		return resolveURL(base, src)
	}
	return ""
}

// Helfer: einfache Dateiendungsprüfung zur Unterscheidung Audio/Video
func isAudioURL(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	if u == "" {
		return false
	}
	// gängige Audio-Endungen
	return strings.HasSuffix(u, ".mp3") || strings.HasSuffix(u, ".m4a") || strings.HasSuffix(u, ".aac") || strings.HasSuffix(u, ".wav") || strings.HasSuffix(u, ".ogg") || strings.HasSuffix(u, ".flac")
}

func isVideoURL(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	if u == "" {
		return false
	}
	// gängige Video-Endungen
	return strings.HasSuffix(u, ".mp4") || strings.HasSuffix(u, ".webm") || strings.HasSuffix(u, ".m3u8") || strings.HasSuffix(u, ".mov") || strings.HasSuffix(u, ".ogg") || strings.HasSuffix(u, ".avi") || strings.HasSuffix(u, ".mkv")
}

// rekursive Suche nach contentUrl in JSON-LD, aber gefiltert nach Art ("audio"|"video")
func findContentURLInJSONValueForKind(v interface{}, kind string) string {
	switch val := v.(type) {
	case map[string]interface{}:
		// Wenn das Objekt einen @type oder type hat, prüfen wir diesen
		if t, ok := val["@type"].(string); ok {
			tLower := strings.ToLower(t)
			if kind == "audio" && strings.Contains(tLower, "audio") {
				if s, ok := val["contentUrl"].(string); ok && s != "" && isAudioURL(s) {
					return s
				}
				if s, ok := val["url"].(string); ok && s != "" && isAudioURL(s) {
					return s
				}
			}
			if kind == "video" && strings.Contains(tLower, "video") {
				if s, ok := val["contentUrl"].(string); ok && s != "" && isVideoURL(s) {
					return s
				}
				if s, ok := val["url"].(string); ok && s != "" && isVideoURL(s) {
					return s
				}
			}
		}
		// direkte Unterobjekte audio/video
		if audioObj, ok := val["audio"].(map[string]interface{}); ok && kind == "audio" {
			if s, ok := audioObj["contentUrl"].(string); ok && s != "" {
				return s
			}
			if s, ok := audioObj["url"].(string); ok && s != "" {
				return s
			}
		}
		if videoObj, ok := val["video"].(map[string]interface{}); ok && kind == "video" {
			if s, ok := videoObj["contentUrl"].(string); ok && s != "" {
				return s
			}
			if s, ok := videoObj["url"].(string); ok && s != "" {
				return s
			}
		}
		// generischer contentUrl: nur zurückgeben, wenn Endung passt
		if s, ok := val["contentUrl"].(string); ok && s != "" {
			if kind == "audio" && isAudioURL(s) {
				return s
			}
			if kind == "video" && isVideoURL(s) {
				return s
			}
		}
		// rekursiv in alle Felder
		for _, v2 := range val {
			if s := findContentURLInJSONValueForKind(v2, kind); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, e := range val {
			if s := findContentURLInJSONValueForKind(e, kind); s != "" {
				return s
			}
		}
	}
	return ""
}

// generische, unspezifische Suche (bleibt als Fallback)
func findContentURLInJSONValue(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		if s, ok := val["contentUrl"].(string); ok && s != "" {
			return s
		}
		if audioObj, ok := val["audio"].(map[string]interface{}); ok {
			if s, ok := audioObj["contentUrl"].(string); ok && s != "" {
				return s
			}
		}
		if videoObj, ok := val["video"].(map[string]interface{}); ok {
			if s, ok := videoObj["contentUrl"].(string); ok && s != "" {
				return s
			}
		}
		for _, v2 := range val {
			if s := findContentURLInJSONValue(v2); s != "" {
				return s
			}
		}
	case []interface{}:
		for _, e := range val {
			if s := findContentURLInJSONValue(e); s != "" {
				return s
			}
		}
	}
	return ""
}

func findAudioInJSONLD(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var audio string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(i int, s *goquery.Selection) bool {
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return true
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(jsonText), &parsed); err == nil {
			// versuche gezielt audio zu finden
			if found := findContentURLInJSONValueForKind(parsed, "audio"); found != "" {
				audio = found
				return false
			}
			// fallback: generisch, aber stelle sicher, dass es kein Video ist
			if found := findContentURLInJSONValue(parsed); found != "" {
				if isAudioURL(found) {
					audio = found
					return false
				}
			}
		}
		return true
	})
	return audio
}

func findFirstVideo(htmlStr string) string {
	return findFirstSrcInTag(htmlStr, "video")
}

func findVideoInJSONLD(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var video string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(i int, s *goquery.Selection) bool {
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return true
		}
		var parsed interface{}
		if err := json.Unmarshal([]byte(jsonText), &parsed); err == nil {
			// versuche gezielt video zu finden
			if found := findContentURLInJSONValueForKind(parsed, "video"); found != "" {
				video = found
				return false
			}
			// fallback: generisch, aber stelle sicher, dass es ein Video ist
			if found := findContentURLInJSONValue(parsed); found != "" {
				if isVideoURL(found) {
					video = found
					return false
				}
			}
		}
		return true
	})
	return video
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	refURL, err := url.Parse(ref)
	if err == nil && (refURL.IsAbs() || base == nil) {
		return refURL.String()
	}
	if base != nil {
		return base.ResolveReference(refURL).String()
	}
	return ref
}

func extractOpenGraph(htmlStr string) string {
	ogData := make(map[string]string)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err == nil {
		doc.Find("meta[property^='og:']").Each(func(i int, s *goquery.Selection) {
			prop, _ := s.Attr("property")
			content, _ := s.Attr("content")
			if prop != "" && content != "" {
				ogData[prop] = content
			}
		})
	}
	ogJSONBytes, _ := json.MarshalIndent(ogData, "", "  ")
	return string(ogJSONBytes)
}

// Extrahiere og:image aus dem OpenGraph-JSON-String, inkl. Fallbacks für og:image:url und og:image:secure_url
func findOGImageFromJSON(ogJSONString string) string {
	var ogData map[string]interface{}
	if err := json.Unmarshal([]byte(ogJSONString), &ogData); err == nil {
		// Prüfe verschiedene Varianten
		if val, ok := ogData["og:image"]; ok {
			if img, ok := val.(string); ok && img != "" {
				return img
			}
		}
		if val, ok := ogData["og:image:url"]; ok {
			if img, ok := val.(string); ok && img != "" {
				return img
			}
		}
		if val, ok := ogData["og:image:secure_url"]; ok {
			if img, ok := val.(string); ok && img != "" {
				return img
			}
		}
	}
	return ""
}

func cleanUpText(text string) string {
	cleanText := strings.TrimSpace(text)
	cleanText = strings.ReplaceAll(cleanText, "\t", " ")
	cleanText = strings.Join(strings.Fields(cleanText), " ")
	return cleanText
}

func buildResultJSON(title, image, video, audio, cleanText string, bodyHTML string, ogJSONString string) string {
	var ogData map[string]interface{}
	_ = json.Unmarshal([]byte(ogJSONString), &ogData)
	jsonMap := map[string]interface{}{
		"headline":  title,
		"image":     image,
		"video":     video,
		"audio":     audio,
		"clear":     cleanText,
		"html":      bodyHTML,
		"opengraph": ogData,
	}
	jsonBytes, _ := json.MarshalIndent(jsonMap, "", "  ")
	jsonStr := string(jsonBytes)
	jsonStr = strings.ReplaceAll(jsonStr, "\t", " ")
	jsonStr = strings.Join(strings.Fields(jsonStr), " ")
	// Unicode-Escaping für <, >, & rückgängig machen
	jsonStr = strings.ReplaceAll(jsonStr, "\\u003c", "<")
	jsonStr = strings.ReplaceAll(jsonStr, "\\u003e", ">")
	jsonStr = strings.ReplaceAll(jsonStr, "\\u0026", "&")
	jsonStr = strings.ReplaceAll(jsonStr, "\\n", "")
	// Überflüssige Leerzeichen zwischen HTML-Tags entfernen (z.B. ">   <" zu "><")
	jsonStr = strings.ReplaceAll(jsonStr, "> <", "><")
	return jsonStr
}

func PrettyPrintHTML(input string) string {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return input
	}
	var buf bytes.Buffer
	prettyPrintNode(&buf, doc, 0)
	return buf.String()
}

func prettyPrintNode(buf *bytes.Buffer, n *html.Node, depth int) {
	if n.Type == html.ElementNode || n.Type == html.DocumentNode {
		if n.Type == html.ElementNode {
			buf.WriteString(strings.Repeat("  ", depth))
			buf.WriteString("<" + n.Data)
			for _, attr := range n.Attr {
				buf.WriteString(fmt.Sprintf(` %s="%s"`, attr.Key, attr.Val))
			}
			buf.WriteString(">\n")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			prettyPrintNode(buf, c, depth+1)
		}
		if n.Type == html.ElementNode {
			buf.WriteString(strings.Repeat("  ", depth))
			buf.WriteString(fmt.Sprintf("</%s>\n", n.Data))
		}
	} else if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			buf.WriteString(strings.Repeat("  ", depth))
			buf.WriteString(text + "\n")
		}
	}
}
