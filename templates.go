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
		{{if .Analyzed}}
			<button id="refetch-btn" type="button" style="margin-left:1em;">Neu laden</button>
			<dialog id="refetch-dialog" aria-labelledby="refetch-title">
				<h3 id="refetch-title">Fehler beim Neuladen</h3>
				<p id="refetch-message"></p>
				<div style="text-align:right;margin-top:0.5em;">
					<button id="refetch-close">Schließen</button>
				</div>
			</dialog>
			<script>
			(function(){
				var btn = document.getElementById('refetch-btn');
				var dialog = document.getElementById('refetch-dialog');
				var msgEl = document.getElementById('refetch-message');
				var closeBtn = document.getElementById('refetch-close');
				if (!btn) return;
				closeBtn && closeBtn.addEventListener('click', function(){
					if (dialog && dialog.close) dialog.close();
				});
				btn.addEventListener('click', function(){
					btn.disabled = true;
					var url = '/?url={{.URL}}&force=1';
					fetch(url, { method: 'GET', credentials: 'same-origin' })
											.then(function(resp){
												if (resp.ok) {
													// Reload the page to show updated result (without force param)
													window.location.href = '/?url={{.URL}}';
													// stop further processing to avoid reading response after navigation
													return Promise.resolve();
												}
												// If not ok, read response text and show it
												return resp.text().then(function(txt){
													throw new Error(txt || ('HTTP ' + resp.status));
												});
											})
						.catch(function(err){
							console.error('Refetch failed', err);
							var msg = String(err.message || err);
							if (msg.indexOf('Fehler:') === 0) {
								msg = msg.replace(/^Fehler:\s*/i, '');
							}
							if (msgEl) msgEl.textContent = msg || 'Fehler beim Neuladen';
							if (dialog) {
								try {
									if (dialog.showModal) dialog.showModal(); else dialog.setAttribute('open', '');
								} catch (e) {
									// fallback
									dialog.setAttribute('open', '');
								}
							}
							btn.disabled = false;
						});
				});
			})();
			</script>
		{{end}}
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

	{{if .ErrorMessage}}
	<script>
	(function(){
		var dialogHTML = '<dialog id="error-dialog"><h3>Fehler</h3><p>' + {{printf "%q" .ErrorMessage}} + '</p><div style="text-align:right;margin-top:0.5em;"><button id="error-open">In neuem Tab öffnen</button> <button id="error-close">Schließen</button></div></dialog>';
		document.body.insertAdjacentHTML('beforeend', dialogHTML);
		var dlg = document.getElementById('error-dialog');
		var openBtn = document.getElementById('error-open');
		var closeBtn = document.getElementById('error-close');
		if (openBtn) {
			openBtn.addEventListener('click', function(){ window.open('{{.URL}}', '_blank'); });
		}
		if (closeBtn) {
			closeBtn.addEventListener('click', function(){ if (dlg && dlg.close) dlg.close(); });
		}
		if (dlg) {
			try { if (dlg.showModal) dlg.showModal(); else dlg.setAttribute('open', ''); } catch (e) { dlg.setAttribute('open', ''); }
		}
	})();
	</script>
	{{end}}
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
