package main

import (
	"bytes"
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
	httpClient = &http.Client{
		Timeout: 15 * time.Second,
	}
	pageTmpl = template.Must(template.New("page").Parse(pageTemplate))
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
			<video src="{{.Video}}" controls style="max-width:400px;display:block;margin-bottom:1em;"></video>
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
	BodyHTML  template.HTML
	CleanText string
	OpenGraph template.HTML
	JSON      string
	Analyzed  bool
}

func main() {
	addr := getServerAddr()
	http.HandleFunc("/", handler)
	fmt.Printf("Server läuft auf http://%s\n", addr)
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
		title, image, video, bodyHTML, cleanText, ogJSONString, err := analyzeURL(urlStr)
		data.URL = urlStr
		data.Analyzed = true
		if err != nil {
			http.Error(w, "Fehler: "+err.Error(), http.StatusBadRequest)
			return
		}
		data.Title = title
		data.Image = image
		data.Video = video
		data.BodyHTML = bodyHTML
		data.CleanText = cleanText
		data.OpenGraph = template.HTML(ogJSONString)
		data.JSON = buildResultJSON(title, image, video, cleanText, bodyHTML, ogJSONString)
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		log.Printf("Template-Fehler: %v", err)
		http.Error(w, "Interner Fehler beim Rendern", http.StatusInternalServerError)
	}
}

func analyzeURL(urlStr string) (title, image, video string, bodyHTML template.HTML, cleanText string, ogJSONString string, err error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("Fehler beim Erstellen der Anfrage: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("Fehler beim Laden der Seite: %v", err)
	}
	defer resp.Body.Close()
	originalHTMLBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("Fehler beim Lesen des HTML: %v", err)
	}
	originalHTML := string(originalHTMLBytes)
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("Ungültige URL: %v", err)
	}
	article, err := readability.FromReader(strings.NewReader(originalHTML), parsedURL)
	if err != nil {
		return "", "", "", "", "", "", fmt.Errorf("Readability-Fehler: %v", err)
	}
	title = article.Title
	image = extractImage(article.Content, originalHTML, urlStr)
	video = extractVideo(article.Content, originalHTML, urlStr)
	ogJSONString = extractOpenGraph(originalHTML)
	formattedHTML := PrettyPrintHTML(article.Content)
	bodyHTML = template.HTML(formattedHTML)
	cleanText = cleanUpText(article.TextContent)
	return title, image, video, bodyHTML, cleanText, ogJSONString, nil
}

func extractImage(articleHTML, originalHTML, baseURL string) string {
	// 1. Aus Article-Content
	if img := findFirstImage(articleHTML); img != "" {
		return resolveURL(baseURL, img)
	}
	// 2. Aus Original-HTML
	if img := findFirstImage(originalHTML); img != "" {
		return resolveURL(baseURL, img)
	}
	// 3. Background-Image in Style-Attributen
	if img := findBackgroundImage(originalHTML); img != "" {
		return resolveURL(baseURL, img)
	}
	// 4. Background-Image in <style>-Tags
	if img := findBackgroundImageInStyleTag(originalHTML); img != "" {
		return resolveURL(baseURL, img)
	}
	return ""
}

func findFirstImage(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	imgSrc, imgExists := doc.Find("img").First().Attr("src")
	if imgExists && imgSrc != "" {
		return imgSrc
	}
	return ""
}

func findBackgroundImage(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var bgImg string
	re := regexp.MustCompile(`background-image\s*:\s*url\(['"]?([^'")]+)['"]?\)`)
	doc.Find("[style]").EachWithBreak(func(i int, s *goquery.Selection) bool {
		style, _ := s.Attr("style")
		if strings.Contains(style, "background-image") {
			matches := re.FindStringSubmatch(style)
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
		reBG := regexp.MustCompile(`background-image\s*:\s*url\(['"]?([^'")]+)['"]?\)`)
		matchesBG := reBG.FindStringSubmatch(css)
		if len(matchesBG) > 1 {
			image = matchesBG[1]
			found = true
			return false
		}
		reVar := regexp.MustCompile(`--featured-img\s*:\s*url\(['"]?([^'")]+)['"]?\)`)
		matchesVar := reVar.FindStringSubmatch(css)
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

func extractVideo(articleHTML, originalHTML, baseURL string) string {
	// 1. Aus Article-Content
	if vid := findFirstVideo(articleHTML); vid != "" {
		return resolveURL(baseURL, vid)
	}
	// 2. Aus Original-HTML
	if vid := findFirstVideo(originalHTML); vid != "" {
		return resolveURL(baseURL, vid)
	}
	// 3. Aus JSON-LD
	if vid := findVideoInJSONLD(originalHTML); vid != "" {
		return resolveURL(baseURL, vid)
	}
	return ""
}

func findFirstVideo(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var video string
	doc.Find("video").EachWithBreak(func(i int, s *goquery.Selection) bool {
		src, exists := s.Attr("src")
		if exists && src != "" {
			video = src
			return false
		}
		s.Find("source").EachWithBreak(func(j int, ss *goquery.Selection) bool {
			src, exists := ss.Attr("src")
			if exists && src != "" {
				video = src
				return false
			}
			return true
		})
		return video == ""
	})
	return video
}

func findVideoInJSONLD(htmlStr string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return ""
	}
	var video string
	doc.Find("script[type='application/ld+json']").EachWithBreak(func(i int, s *goquery.Selection) bool {
		jsonText := s.Text()
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonText), &data); err == nil {
			if val, ok := data["contentUrl"].(string); ok && val != "" {
				video = val
				return false
			}
			if videoObj, ok := data["video"].(map[string]interface{}); ok {
				if val, ok := videoObj["contentUrl"].(string); ok && val != "" {
					video = val
					return false
				}
			}
		}
		return true
	})
	return video
}

func resolveURL(base, ref string) string {
	baseURL, err1 := url.Parse(base)
	refURL, err2 := url.Parse(ref)
	if err1 == nil && err2 == nil && baseURL != nil {
		return baseURL.ResolveReference(refURL).String()
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

func cleanUpText(text string) string {
	cleanText := strings.TrimSpace(text)
	cleanText = strings.ReplaceAll(cleanText, "\t", " ")
	cleanText = strings.Join(strings.Fields(cleanText), " ")
	return cleanText
}

func buildResultJSON(title, image, video, cleanText string, bodyHTML template.HTML, ogJSONString string) string {
	var ogData map[string]interface{}
	_ = json.Unmarshal([]byte(ogJSONString), &ogData)
	jsonMap := map[string]interface{}{
		"headline":  title,
		"image":     image,
		"video":     video,
		"clear":     cleanText,
		"html":      string(bodyHTML),
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
