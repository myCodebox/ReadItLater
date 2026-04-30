ReadItLater — Go Analyzer

Kurz:
Dieses kleine Tool lädt eine beliebige Webseite, extrahiert Titel, lesbaren Text (via go-readability), OpenGraph‑Daten sowie Medien (Bild, Video, Audio) und zeigt die Ergebnisse in einem simplen Web‑UI an. Es hat einen eingebauten persistenten Cache (SQLite), Retry‑Mechanismen und einfache UI‑Controls zum erneuten Laden (Refetch).

Wofür es nützlich ist:
- Schnell einzelne Artikel analysieren und den bereinigten Text / Medien extrahieren
- Ergebnisse zwischenspeichern (persistenter Cache)
- Basis für weitergehende Read‑it‑later / Scraping‑Workflows

Voraussetzungen
- Go (Version entsprechend `go.mod`, z. B. Go 1.20+)
- Optional: Chromium/Headless (nicht notwendig; kein Browser‑Fallback in der Standardkonfiguration)

Aufbau / relevante Dateien
- `main.go` – Server‑Start, Logger (zap) und Cache‑Initialisierung
- `server.go` – HTTP Handler / Template Rendering
- `fetch.go` – HTTP‑Fetch, Prefetch, Retry‑Policy, Integration mit Readability
- `extract.go` – Extraktionsfunktionen (OG, Bilder, JSON‑LD, etc.)
- `templates.go` – HTML‑Template und kleine UI‑Skripte
- `sqlcache.go` – SQLite‑basierter Cache (TTL + max entries + async writer)
- `*_test.go` – Unit‑ und Integrationstests

Bauen & Starten (lokal)
1) Code bauen / starten

   go run .

2) Debug/Dev Modus (mehr Logs)

   go run . -debug

   oder per Environment:

   READITLATER_DEBUG=1 go run .

3) Optional: Konfiguration über Umgebungsvariablen
- `READITLATER_ADDR` — Server Adresse (z. B. 127.0.0.1:8080). Default: 127.0.0.1:8080
- `READITLATER_DEBUG` — wenn =1 oder "true" aktiviert Zap Dev Logging
- `READITLATER_MAX_RETRIES` — maximale Retry‑Versuche für HTTP (Standard 3)
- `READITLATER_BASE_BACKOFF_MS` — Basis Backoff in ms (Standard 200)

Beispiel:

   READITLATER_MAX_RETRIES=5 READITLATER_BASE_BACKOFF_MS=500 go run .

Web‑UI
- Öffne im Browser: http://127.0.0.1:8080/
- Gib im Feld die gewünschte URL ein und klicke "Analysieren".
- Die Seite zeigt Titel, bereinigten Text, Body HTML, OpenGraph JSON und ein Ergebnis‑JSON.
- Wenn bereits eine Analyse im Cache vorliegt, wird das Ergebnis schnell zurückgegeben.
- "Neu laden" (Refetch): erzwingt ein erneutes Laden (löscht Cache‑Eintrag vor Fetch). Bei Fehlern (z. B. Cloudflare JS‑Challenge) erscheint ein Dialog mit der Fehlermeldung und einem Button "In neuem Tab öffnen".

Cache
- Persistenter SQLite Cache liegt unter `cache/readitlater.db`
- Default TTL: 10 Minuten
- Default max entries: 1000
- Schreibzugriffe erfolgen asynchron; es gibt einen Hintergrund‑Worker, der die DB schreibt.

Retry / Backoff
- Netzwerkfehler und temporäre Serverfehler (429, 500, 502, 503, 504) werden mit Exponential Backoff und Jitter erneut versucht.
- Anzahl Versuche und Basis‑Backoff sind konfigurierbar über ENV (siehe oben).

Fehlerbehandlung bei Bot‑/JS‑Challenges
- Wenn die Zielseite eine JavaScript‑Challenge (z. B. Cloudflare) ausgibt, kann der Headless‑Fallback nicht automatisch gelöst werden (kein Browser‑Fallback standardmäßig).
- Die App erkennt typische Challenge‑Markers und zeigt eine verständliche Fehlermeldung im UI an: "Seite wird durch eine JavaScript-/Bot‑Challenge ... Bitte öffne die URL im Browser und versuche es erneut.".
- Workaround: Klicke im Dialog auf "In neuem Tab öffnen", löse die Challenge manuell im Browser und klicke dann "Neu laden" in der App.

Logging
- Structured logging via `go.uber.org/zap`.
- Dev/Debug Mode (`-debug` oder `READITLATER_DEBUG`) aktiviert Dev‑Config von `zap` (menschlichere Logs).

Entwicklung / Tests
- Unit‑ und Integrationstests sind vorhanden. Ausführen mit:

   go test ./...

- Formatierung: `gofmt -w .`

Erweiterungen / Ideen
- Optionaler Headless‑Browser‑Fallback (chromedp) für JS‑gesicherte Seiten — nicht integriert standardmäßig.
- Stale‑while‑revalidate UX: sofortiges Zurückgeben von Cache und Hintergrund‑Refresh.
- Verbesserte OG/JSON‑LD‑Normalization und Domain‑spezifische Heuristiken.

Lizenz & Hinweise
- Das Projekt ist ein privates Hilfswerkzeug. Achte beim Scrapen auf die Nutzungsbedingungen der Zielwebseiten.

Kontakt
- Falls du weitere Features möchtest oder Bugs findest — sag Bescheid. Ich helfe beim Erweitern oder beim Erstellen von PRs.
