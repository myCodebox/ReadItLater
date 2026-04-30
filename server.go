package main

import (
	"net/http"
	"net/url"
	"os"
)

// Konfiguration
const (
	defaultAddr = "127.0.0.1:8080"
)

func getServerAddr() string {
	if val := os.Getenv("READITLATER_ADDR"); val != "" {
		return val
	}
	return defaultAddr
}

func handler(w http.ResponseWriter, r *http.Request) {
	data := PageData{}
	urlStr := r.URL.Query().Get("url")
	force := r.URL.Query().Get("force") == "1"
	if urlStr != "" {
		if decoded, err := url.QueryUnescape(urlStr); err == nil {
			urlStr = decoded
		}
		// If force is set, delete cache entry before analyzing
		if force && sqlCache != nil {
			_ = sqlCache.Delete(urlStr)
		}
		title, image, video, audio, bodyHTML, cleanText, ogJSONString, err := analyzeURL(urlStr)
		data.URL = urlStr
		data.Analyzed = true
		if err != nil {
			// Don't abort rendering the page — show error dialog instead so page isn't interrupted
			data.ErrorMessage = err.Error()
		} else {
			data.Title = title
			data.Image = image
			data.Video = video
			data.Audio = audio
			data.BodyHTML = bodyHTML
			data.CleanText = cleanText
			data.OpenGraph = ogJSONString
			data.JSON = buildResultJSON(title, image, video, audio, cleanText, bodyHTML, ogJSONString)
		}
	}
	if err := pageTmpl.Execute(w, data); err != nil {
		if logger != nil {
			logger.Errorw("Template-Fehler", "err", err)
		}
		http.Error(w, "Interner Fehler beim Rendern", http.StatusInternalServerError)
	}
}
