package main

import (
	"html/template"
	"strings"
)

const pageTemplate = `
<!DOCTYPE html>
<html lang="de">
<head>
	<meta charset="UTF-8">
	<title>ReadItLater Go Analyzer</title>
	<link rel="stylesheet" href="/static/css/site.css">
	<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body{{if .ErrorMessage}} data-error-message="{{.ErrorMessage}}"{{end}}>
	<div class="container">
		<header class="site-header">
			<h1>ReadItLater Go Analyzer</h1>
			<p class="muted">Analysiere eine Webseite und zeige Bilder, Medien und strukturierte Daten an.</p>
		</header>

		<main>
			<form class="search-form" method="get" action="/" aria-label="URL suchen">
				<div class="search-row">
					<label class="search-label" for="url">URL</label>
					<input class="search-input" type="text" id="url" name="url" value="{{.URL}}" placeholder="https://example.com" required aria-label="URL zum Analysieren">
					<div class="search-actions">
						<button class="btn btn-primary" type="submit">Analysieren</button>
						{{if .Analyzed}}
							<button id="refetch-btn" class="btn btn-secondary" type="button" data-url="{{.URL}}">Neu laden</button>
							<dialog id="refetch-dialog" aria-labelledby="refetch-title">
								<h3 id="refetch-title">Fehler beim Neuladen</h3>
								<p id="refetch-message"></p>
								<div style="text-align:right;margin-top:0.5em;">
									<button id="refetch-close">Schließen</button>
								</div>
							</dialog>
							<!-- Refetch controls are handled by external JS -->
							<script src="/static/js/app.js" defer></script>
						{{end}}
					</div>
				</div>
			</form>

			{{if .Analyzed}}
	<section class="result">
		<header class="result-header">
			<h2 class="result-title">{{.Title}}</h2>
		</header>

		<div class="media-grid">
			{{if .Image}}
				<div class="media-item media-image">
					<div class="media-label">Bild</div>
					<img src="{{.Image}}" alt="Artikelbild">
				</div>
			{{end}}
			{{if .Video}}
				{{if (hasPrefix .Video "blob:")}}
					<div class="media-item">
						<div class="media-label">Video</div>
						<strong>Hinweis:</strong> <span class="muted">blob:-URLs funktionieren nur im Browser-Kontext und können nicht direkt heruntergeladen werden.</span>
						<br>
						<code>{{.Video}}</code>
					</div>
				{{else}}
					<div class="media-item media-video">
						<div class="media-label">Video</div>
						<video id="video" controls>
							<source src="{{.Video}}">
							Dein Browser unterstützt das Video-Tag nicht.
						</video>
					</div>
					<script src="https://cdn.jsdelivr.net/npm/hls.js@latest"></script>
					<script>
					(function() {
						var videoSrc = "{{.Video}}";
						var video = document.getElementById('video');
						if (videoSrc && videoSrc.endsWith('.m3u8')) {
							if (window.Hls && Hls.isSupported()) {
								var hls = new Hls();
								hls.loadSource(videoSrc);
								hls.attachMedia(video);
							} else if (video.canPlayType('application/vnd.apple.mpegurl')) {
								video.src = videoSrc;
							} else {
								document.querySelector('.media-video').innerHTML = '<div class="muted">Dein Browser unterstützt dieses Videoformat nicht direkt. Bitte verwende Safari oder installiere eine HLS-Erweiterung.</div>';
							}
						} else {
							try { video.src = videoSrc; } catch (e) { console.error('Fehler beim Setzen der Videoquelle', e); }
						}
					})();
					</script>
				{{end}}
			{{end}}
			{{if .Audio}}
				{{if (hasPrefix .Audio "blob:")}}
					<div class="media-item">
						<div class="media-label">Audio</div>
						<strong>Hinweis:</strong> <span class="muted">blob:-URLs funktionieren nur im Browser-Kontext und können nicht direkt heruntergeladen werden.</span>
						<br>
						<code>{{.Audio}}</code>
					</div>
				{{else}}
					<div class="media-item media-audio">
						<div class="media-label">Audio</div>
						<audio src="{{.Audio}}" controls></audio>
					</div>
				{{end}}
			{{end}}
		</div>

		<div class="result-sections">
			<div class="result-section">
				<h3>Bereinigter Text:</h3>
				<textarea readonly>{{.CleanText}}</textarea>
			</div>
			<div class="result-section">
				<h3>Body HTML:</h3>
				<textarea readonly>{{.BodyHTML}}</textarea>
			</div>
			<div class="result-section">
				<h3>Open Graph Daten:</h3>
				<textarea readonly>{{.OpenGraph}}</textarea>
			</div>
			<div class="result-section">
				<h3>JSON:</h3>
				<textarea readonly>{{.JSON}}</textarea>
			</div>
		</div>
	</section>
	{{end}}


	<!-- Static error dialog rendered and controlled by app.js -->
	<dialog id="error-dialog" aria-labelledby="error-title" style="display:none;">
		<h3 id="error-title">Fehler</h3>
		<p id="error-message"></p>
		<div style="text-align:right;margin-top:0.5em;">
			<button id="error-open">In neuem Tab öffnen</button>
			<button id="error-close">Schließen</button>
		</div>
	</dialog>
</body>
</html>
`

type PageData struct {
	URL          string
	Title        string
	Image        string
	Video        string
	Audio        string
	BodyHTML     string
	CleanText    string
	OpenGraph    string
	JSON         string
	Analyzed     bool
	ErrorMessage string
}

var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"hasPrefix": strings.HasPrefix,
}).Parse(pageTemplate))
