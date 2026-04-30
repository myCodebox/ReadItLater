package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

const (
	defaultMaxRetries  = 3
	defaultBaseBackoff = 200 * time.Millisecond
)

var (
	configuredMaxRetries  = defaultMaxRetries
	configuredBaseBackoff = defaultBaseBackoff
)

func init() {
	if v := os.Getenv("READITLATER_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			configuredMaxRetries = n
		}
	}
	if v := os.Getenv("READITLATER_BASE_BACKOFF_MS"); v != "" {
		if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
			configuredBaseBackoff = time.Duration(ms) * time.Millisecond
		}
	}
}

// doRequestWithRetry performs the HTTP request with retries on transient errors/status codes.
func doRequestWithRetry(ctx context.Context, req *http.Request, maxRetries int) (*http.Response, error) {
	attempt := 0
	backoff := configuredBaseBackoff
	for {
		resp, err := httpClient.Do(req)
		if err != nil {
			if attempt >= maxRetries {
				return nil, err
			}
			// wait with jitter
			jitter := time.Duration(rand.Int63n(int64(backoff)))
			select {
			case <-time.After(backoff + jitter):
				attempt++
				backoff *= 2
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		// got a response
		if resp.StatusCode == 429 || resp.StatusCode == 500 || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504 {
			// transient server error, read and close body then retry if possible
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if attempt >= maxRetries {
				return resp, nil // return last response
			}
			jitter := time.Duration(rand.Int63n(int64(backoff)))
			select {
			case <-time.After(backoff + jitter):
				attempt++
				backoff *= 2
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return resp, nil
	}
}

func analyzeURL(urlStr string) (title, image, video, audio string, bodyHTML string, cleanText string, ogJSONString string, err error) {
	// check cache first
	if sqlCache != nil {
		if b, ok, e := sqlCache.Get(urlStr); e == nil && ok {
			var ent AnalysisCacheEntry
			if um := json.Unmarshal(b, &ent); um == nil {
				return ent.Title, ent.Image, ent.Video, ent.Audio, ent.BodyHTML, ent.CleanText, ent.OpenGraph, nil
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// parse URL early to allow origin prefetch and cookie checks
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Ungültige URL: %w", err)
	}
	// Prefetch origin to let servers set cookies (best-effort)
	if httpClient != nil && httpClient.Jar != nil {
		cookies := httpClient.Jar.Cookies(parsedURL)
		if len(cookies) == 0 {
			origin := parsedURL.Scheme + "://" + parsedURL.Host + "/"
			prefReq, _ := http.NewRequestWithContext(ctx, "GET", origin, nil)
			prefReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			prefReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
			prefReq.Header.Set("Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")
			prefReq.Header.Set("Referer", origin)
			if presp, perr := doRequestWithRetry(ctx, prefReq, 1); perr == nil {
				if presp.Body != nil {
					io.Copy(io.Discard, presp.Body)
					presp.Body.Close()
				}
				// small delay to let cookies be stored
				time.Sleep(150 * time.Millisecond)
			}
		}
	}

	// build main request
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", "", "", "", "", "", "", fmt.Errorf("Fehler beim Erstellen der Anfrage: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en-US;q=0.8,en;q=0.7")

	resp, err := doRequestWithRetry(ctx, req, defaultMaxRetries)
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
			altResp, aerr := doRequestWithRetry(ctx, altReq, defaultMaxRetries)
			if aerr == nil {
				resp = altResp
				// defer close for the response we'll use further down
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
					// success with alternative headers; continue normally
				} else {
					// read small snippet for debugging
					snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
					snippetStr := strings.TrimSpace(string(snippet))
					// detect common Cloudflare / JS Challenge markers
					lower := strings.ToLower(snippetStr)
					if strings.Contains(lower, "just a moment") || strings.Contains(lower, "challenges.cloudflare.com") || strings.Contains(lower, "cf-chl") || strings.Contains(lower, "checking your browser") || strings.Contains(lower, "cf-browser-verification") {
						// log full snippet for server-side debugging (trimmed)
						if logger != nil {
							logger.Debugw("JS/Cloudflare challenge detected", "url", urlStr, "snippet", snippetStr)
						}
						return "", "", "", "", "", "", "", fmt.Errorf("Seite wird durch eine JavaScript-/Bot-Challenge (z. B. Cloudflare) geschützt und kann nicht per HTTP-Client geladen werden. Bitte öffne die URL im Browser und versuche es erneut.")
					}
					return "", "", "", "", "", "", "", fmt.Errorf("ungültiger Statuscode nach Retry: %d - %s", resp.StatusCode, snippetStr)
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

	// parsedURL already parsed earlier for prefetch

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

	// store in cache
	if sqlCache != nil {
		entry := AnalysisCacheEntry{
			Title:     title,
			Image:     image,
			Video:     video,
			Audio:     audio,
			BodyHTML:  bodyHTML,
			CleanText: cleanText,
			OpenGraph: ogJSONString,
			StoredAt:  time.Now().Unix(),
		}
		if b, merr := json.Marshal(entry); merr == nil {
			_ = sqlCache.SetAsync(urlStr, b)
		}
	}
	return title, image, video, audio, bodyHTML, cleanText, ogJSONString, nil
}
