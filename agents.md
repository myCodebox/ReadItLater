# Agents Task List

Ziel: Schrittweise Refactor- und Verbesserungsarbeiten am Projekt `ReadItLater` in klaren, überprüfbaren Tasks. Jede Aufgabe wird hier dokumentiert, ihren Status aufgeführt und (nach Fertigstellung) mit einem kurzen Changelog versehen.

Format:
- [Status] Task title
  - Beschreibung
  - Dateien / Funktionen betroffen
  - Notes / Links

Tasks:

1. [Done] Improve HTTP client transport & add graceful shutdown
   - Beschreibung: Setze `http.Client.Transport` mit sinnvollen Pool- und Timeout-Werten. Ersetze `http.ListenAndServe` durch `http.Server` mit `Shutdown`-Logik, damit SIGINT/SIGTERM sauber verarbeitet werden.
   - Dateien: `main.go`
   - Grund: Bessere Verbindungspool-Nutzung, robustes Stoppen des Servers.
   - Changelog:
     - `http.Client.Transport` mit `MaxIdleConns`, `IdleConnTimeout`, `TLSHandshakeTimeout` konfiguriert.
     - Server verwendet nun `http.Server` und `server.Shutdown` für Graceful-Stop.
     - Signal-Handler für SIGINT/SIGTERM hinzugefügt.
     - `agents.md` aktualisiert (Task-Status auf Done).

2. [Done] Split code into packages/files (fetch, extract, ui)
   - Beschreibung: Aufteilen von `main.go` in kleinere Dateien / internal packages: `fetch.go` (HTTP/Fetch-Logic), `extract.go` (HTML/OG/JSON-LD-Parsing), `server.go` (HTTP Server + handler), `templates.go` (pageTemplate + PageData).
   - Dateien: `main.go`, `server.go`, `fetch.go`, `extract.go`, `templates.go`
   - Grund: Lesbarkeit, Testbarkeit, bessere Isolation.
   - Changelog:
     - `main.go` verkleinert und startet nun Server; `http.Client` + Transport konfiguriert.
     - `server.go` enthält `handler` und `getServerAddr`.
     - `fetch.go` enthält `analyzeURL` (HTTP-Fetch, gzip-Handling, 403-Retry, Readability integration).
     - `extract.go` enthält Extraktionslogik (OG, JSON-LD, Bild-/Audio-/Video-Finder, PrettyPrintHTML, resolveURL).
     - `templates.go` enthält `pageTemplate`, `PageData` und `pageTmpl`.
     - Projekt baut erfolgreich nach Refactor (`go build ./...`).

3. [Done] Add unit tests for extract logic
   - Beschreibung: Tests für `findFirstSrcInTag`, `findBackgroundImage`, `findAudioInJSONLD`, `findVideoInJSONLD`, `resolveURL`.
   - Dateien: `extract_test.go`, `fetch_test.go`
   - Grund: Regression-Schutz
   - Changelog:
     - `extract_test.go` hinzugefügt/erweitert mit Tests für image/video/audio/extraction, background-image, pretty-print und resolveURL.
     - `fetch_test.go` hinzugefügt/erweitert mit Integrationstests für `analyzeURL` (gzip, 403-retry, Content-Type checks, OG fallback).
     - Alle Tests: `go test ./...` => OK

4. [Done] Implement caching for fetched pages (SQLite persistent cache)
   - Beschreibung: Implementierung eines persistenten Caches auf SQLite-Basis. Einträge haben TTL und optional eine maximale Anzahl. Ergebnisse der Analyse (Titel, Bild, Video, Audio, bereinigter Text, Body HTML, OpenGraph JSON) werden als JSON-BLOB gespeichert.
   - Dateien: `sqlcache.go`, `fetch.go`, `main.go`, `server.go`
   - Grund: Reduziere Netzwerkanfragen, beschleunige wiederholte Analysen, persistente Speicherung zwischen Neustarts.
   - Changelog:
     - `sqlcache.go` implementiert `SQLCache` (SQLite), `Get`, `Set`, `Delete`, `Close` und janitor zum Entfernen abgelaufener Einträge.
     - `main.go` initialisiert den Cache (`NewSQLCache("cache/readitlater.db", 10*time.Minute, 1000)`).
     - `fetch.go` prüft vor dem HTTP-Request auf Cache-Hits und speichert Ergebnisse nach erfolgreicher Analyse.
     - `server.go` unterstützt `force=1` als Query-Parameter, um Cache-Einträge zu löschen (Refetch).
     - `templates.go` enthält einen "Neu laden"-Link (`&force=1`) zum Erzwingen eines Refetch.
     - Wechsel zu cgo‑freiem SQLite-Treiber (`modernc.org/sqlite`) und `go mod` angepasst.
     - Tests ausgeführt: `go test ./...` => OK

5. [TODO] Improve OpenGraph extraction and normalization
   - Beschreibung: Unterstütze Arrays (`og:image`) und sichere Normalisierung (secure_url, url)
   - Dateien: `extract.go` / `main.go`

6. [Done] Add structured logging (zap)
   - Beschreibung: Ersetze `log.Printf` durch ein strukturiertes Logger-Interface (`zap`), und stelle zentrale Logger-Instanz zur Verfügung.
   - Dateien: `main.go`, `fetch.go`, `server.go`, `templates.go`
   - Changelog:
     - `go.uber.org/zap` integriert und als globaler `logger *zap.SugaredLogger` initialisiert in `main.go`.
     - Debug/Dev Mode via CLI-Flag `-debug` oder ENV `READITLATER_DEBUG=1` hinzugefügt.
     - Nicht-kritische Logs (Cache init/close, Cloudflare-Erkennung, Template-Fehler) auf strukturierte `logger`-Aufrufe umgestellt.
     - Tests und Build angepasst; `go test ./...` läuft erfolgreich.

7. [TODO] Add graceful retry/backoff strategy for fetches
   - Beschreibung: Intelligentere Retry-Strategie (exponentielle Backoff) bei transienten Fehlern.
   - Dateien: `fetch.go`

8. [TODO] Add headless-browser fallback (optional)
   - Beschreibung: Headless-Browser (Playwright/Puppeteer) für JS-abhängige Seiten als optionaler Modus.
   - Dateien: neuer subservice oder CLI-Flag

---

Changelog:
- (now) Task 1 started: HTTP transport + graceful shutdown in progress.
