# ReadItLater

ReadItLater ist eine Anwendung, mit der du interessante Artikel, Links oder Notizen speichern und später lesen kannst. Sie eignet sich ideal, um Inhalte aus dem Web zu sammeln und zu organisieren.

## Was kannst du mit ReadItLater machen?

- **Artikel und Links speichern:** Füge interessante Webseiten, Blogposts oder Videos mit nur einem Klick deiner persönlichen Leseliste hinzu.
- **Notizen anlegen:** Ergänze deine gespeicherten Links mit eigenen Notizen, um wichtige Gedanken oder To-dos festzuhalten.
- **Kategorien und Tags nutzen:** Organisiere deine Einträge mit Kategorien oder Tags, damit du sie später schnell wiederfindest.
- **Such- und Filterfunktionen:** Durchsuche deine Sammlung nach Stichworten, Tags oder Kategorien, um gezielt Inhalte zu finden.
- **Lesestatus verwalten:** Markiere Einträge als „gelesen“ oder „ungelesen“, um den Überblick zu behalten.
- **Favoriten festlegen:** Hebe besonders wichtige oder interessante Inhalte als Favoriten hervor.
- **Plattformübergreifend zugreifen:** Greife von verschiedenen Geräten auf deine gespeicherten Inhalte zu (je nach Implementierung).
- **Offline-Zugriff (optional):** Lies gespeicherte Artikel auch ohne Internetverbindung, sofern die App dies unterstützt.

ReadItLater hilft dir dabei, Informationsflut zu bändigen und spannende Inhalte nicht aus den Augen zu verlieren.

## Server & Zugriff

ReadItLater bringt einen integrierten Webserver mit. Nach dem Start erreichst du die Anwendung über deinen Browser:

- **Adresse:** [http://localhost:8080](http://localhost:8080)

Der Server lauscht standardmäßig auf Port 8080. Du kannst die Weboberfläche nutzen, um Links zu speichern, zu durchsuchen und zu verwalten.

## Features

- Speichern von Links und Artikeln zum späteren Lesen
- Kategorisierung und Tagging von Einträgen
- Einfache Such- und Filterfunktionen
- Minimalistische und benutzerfreundliche Oberfläche

## Installation

1. Repository klonen:
   ```
   git clone https://github.com/dein-benutzername/ReadItLater.git
   ```
2. In das Projektverzeichnis wechseln:
   ```
   cd ReadItLater
   ```
3. Abhängigkeiten installieren (z.B. für Go-Projekte):
   ```
   go mod download
   ```

## Nutzung

Starte die Anwendung mit:
```
go run main.go
```
oder baue ein ausführbares Programm:
```
go build -o readitlater
./readitlater
```

Nach dem Start kannst du im Browser [http://localhost:8080](http://localhost:8080) auf die App zugreifen.

## Mitwirken

Beiträge sind willkommen! Erstelle gerne einen Pull Request oder öffne ein Issue, wenn du Fehler findest oder neue Features vorschlagen möchtest.

## Lizenz

Dieses Projekt steht unter der MIT-Lizenz.
