# ReadItLater — Go Analyzer

ReadItLater ist ein kleines Go‑Web‑Tool zum schnellen Analysieren von Web‑Seiten. Es extrahiert Titel, bereinigten Text, Bilder, Video/Audio‑Quellen, OpenGraph‑ und JSON‑LD‑Daten und präsentiert diese in einer einfachen Web‑UI.

Diese README fasst Bedienung, Architektur und Entwicklungs‑Hinweise zusammen.

---

## Kurzüberblick — was das Projekt jetzt macht

- Analysiert eine angegebene URL und extrahiert:
  - Titel, bereinigten Text (readability-like), Body HTML
  - Erste Bild‑/Video‑/Audio‑Quellen (inkl. HLS .m3u8 Erkennung)
  - OpenGraph‑Tags und JSON‑LD strukturierte Daten
- Persistenter SQLite‑Cache zur Beschleunigung wiederholter Abfragen
- Robuste HTTP‑Fetch‑Logik (Timeouts, CookieJar, Transport‑Tuning)
- "Neu laden" (force refetch) Option vom UI
- Strukturierte Logs (zap) mit Debug‑Flag / ENV
- Responsive, neutral gehaltenes UI mit Light/Dark Mode
- Unit‑ und Integrationstests für Kernfunktionen

---

## Quickstart (lokal)

Voraussetzungen: Go (1.20+ empfohlen), git

1. Repository klonen:

```bash
git clone https://github.com/myCodebox/ReadItLater.git
cd ReadItLater
```

2. Abhängigkeiten holen:

```bash
go mod download
```

3. App starten (Entwicklungsmodus):

```bash
# mit Debug‑Logging
go run . -debug
# oder (ohne Flag) mit normaler Log‑Konfiguration
go run .
```

Die Weboberfläche ist dann erreichbar unter:

- http://localhost:8080

Du kannst eine URL in die Suchleiste eingeben und "Analysieren" klicken. Falls eine Seite bereits im Cache liegt, wird das Ergebnis sofort angezeigt; mit "Neu laden" erzwingst du ein Refetch.

---

## CLI / Env Konfiguration

Das Programm liest einige Einstellungen aus CLI‑Flags und Umgebungsvariablen:

- `-debug` (Flag) oder `READITLATER_DEBUG=1` (ENV): schaltet Entwicklungs‑/Debug‑Logging ein (zap NewDevelopment)
- Cache DB‑Pfad ist derzeit hartkodiert in `main.go` als `cache/readitlater.db` — du kannst das beim Start anpassen oder in `main.go` ändern

Weitere Konfigurations‑Parameter (Retries, Backoff) befinden sich in `fetch.go` / Konstanten und sind als TODO markiert; ich kann sie bei Bedarf als CLI‑Flags hinzufügen.

---

## Architektur / wichtigste Dateien

- `main.go` — Server‑Bootstrap, Logger, Cache‑Init, HTTP Client Konfiguration
- `server.go` — HTTP Handler, Query parsing, force‑Parameter
- `fetch.go` — HTTP fetch + analyzeURL (Prefetch, retries, response validation, caching hook)
- `extract.go` — HTML/OG/JSON‑LD extraction, heuristics für media (image/video/audio)
- `templates.go` — HTML Template (Hero UI, Ergebnisdarstellung)
- `sqlcache.go` — SQLite basierter persistenter Cache (Get/Set/Delete, TTL, async writer)
- `static/` — CSS/JS für UI (`static/css/site.css`, `static/js/app.js`)
- `*_test.go` — Unit und Integrationstests

---

## Tests

Alle Tests lassen sich mit `go test ./...` ausführen. Es wurden Tests für die Extraktionsfunktionen und Teile der Fetch‑Logik ergänzt.

```bash
go test ./...
```

---

## Hinweise zur Entwicklung

- UI: Die Anwendung hat ein zentrales "Hero"‑Suchfeld. Ergebnisse werden als Karte(n) mit Medien links und Details rechts dargestellt (stacked auf Mobilgeräten).
- Fehler-/Challenge‑Erkennung: Wenn Fetches JS‑protected Inhalte (z. B. Cloudflare JS challenge) liefern, versucht der Server dies zu erkennen und gibt eine verständliche Meldung; ein Headless‑Browser‑Fallback ist bewusst optional und noch nicht implementiert.
- Caching: Der SQLite‑Cache beschleunigt wiederholte Analysen; Cache‑Einträge haben TTL (konfigurierbar) und es gibt eine async‑Schreibroutine zum Persistieren.
- Logging: Strukturierte Logs mit zap; verwende `-debug` für Entwickler‑freundliches Console Logging.

---

## Lizenz

MIT
