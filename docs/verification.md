# Nachweis für 0.3.0-rc.1

Lokal geprüft am 9. Oktober 2026 mit Go 1.26.9, Linux amd64 und MCP-Go-SDK v1.8.0.

| Prüfung | Beobachtetes Ergebnis |
| --- | --- |
| Build von Server, OAuth- und Upload-Helfer | Erfolgreich |
| `go test -race -count=1 -timeout=90s ./...` | Erfolgreich |
| Separat integrierter Render-Bereitschaftsdienst, Race-Tests | Erfolgreich; `/mcp` bleibt 503 |
| `go vet ./...`, `go mod verify`, `go mod tidy` | Erfolgreich |
| Subprozess-Smoke-Test | Discovery meldet `pcloud-mcp`, `0.3.0-rc.1`, MCP `2026-07-28` |
| Echte SDK-Client-/Server-Integration mit kontrolliertem Provider-Double | Alle neun Tools, Ergebnisse und schreibgeschützte Registrierung geprüft |
| HTTP-/OAuth-Integration | Gültiger Besitzer gebunden, Scope durchgereicht; fehlender Lese-Scope abgewiesen |
| Fuzzing, jeweils 30 Sekunden, zwei Worker | Config, Dateinamen, Provider-JSON und öffentliche IP-Adressen bestanden |
| Gezielte Mutationstests | 14 von 14 durch Testfehler erkannt, nicht durch Kompilierungsfehler |
| `govulncheck ./...` und Modulscan | Keine bekannten Schwachstellen gemeldet |
| Gitleaks, Arbeitsverzeichnis und Git-Historie | Keine Secrets gefunden |

Fuzz-Laufzahlen dieses Durchlaufs: Config 965864, Dateinamen 839906,
Provider-JSON 69478, öffentliche IP-Adressen 322779. Zahlen sind maschinenabhängig
und keine Akzeptanzschwelle.

Die Mutationen entfernen oder verändern: Config-Validierung, Maximalgrenze,
Transportgrenze, Besitzerbindung, Schreibschalter, Verbot destruktiver Aktionen,
Ordnergrenze, Dateieigentümerschaft, Upload-Hash, OAuth-Audience, Issuer, Subject,
Copy-Überschreibschutz und Upload-Konfliktschutz. Dies ist ein gezielter Nachweis,
kein umfassender Mutation-Score oder Sicherheitsbeweis.

Weitere Negativtests prüfen begrenzte Suche, Datums-/Größenfilter, Text- und
Upload-Limits, nicht freigegebene Einträge, ausfallendes Audit und unplausible
Schreibantworten. Provider-Adapter-Tests kontrollieren POST-Parameter,
Multipart-Reihenfolge und Fehlerredaktion. Sie benutzen kontrollierte HTTP-
Transporte; dadurch werden keine echten pCloud-Dateien angelegt.

## Akzeptanzkriterien dieses Schritts

- Reproduzierbar bauen; stdio bleibt protokollrein; Prozess beendet sich kontrolliert.
- Ohne Credentials keine Datei-Tools, ohne vollständiges OAuth kein HTTP-Dateizugriff.
- Nur die konfigurierte Identität und ihr freigegebener Ordnerbaum sind zugelassen.
- Lesen ist standardmäßig aktiv, Schreiben ausdrücklich optional und auditiert.
- Kein exponierter Lösch-, Rename-, Move- oder Überschreibpfad.
- Upload-Bytes werden begrenzt und geprüft; Namenskonflikte überschreiben keine Datei.
- Die CI wiederholt Prüfungen und baut zusätzlich den Docker-Container.

## Noch nicht nachgewiesen

Echte pCloud-OAuth-Freigabe und Provider-Aufrufe, ein Login über einen konkreten
externen Authorization Server, Widerruf im realen Betrieb, End-to-End-Nutzung
mit ChatGPT/Claude und das vollständige geschützte Render-Deployment benötigen
die zugehörigen Konten und Konfiguration. Lokaler Docker-Daemon ist nicht vorhanden;
Container-Build und Start erfolgen in GitHub Actions. Den Status des konkreten
Commits unter [Actions](https://github.com/cyrano-janus/pcloud-mcp/actions) prüfen.

Diese Dokumentation behauptet weder vollständige MCP-Konformität noch
Produktionsreife. Sie ersetzt nicht die offenen Live-Prüfungen.

## Ergänzung: gehostete pCloud-Einrichtung

Die Tests für den Token-Umschlag und den gehosteten Ablauf wurden zunächst ohne
Implementierung ausgeführt und schlugen wegen fehlender Typen/Funktionen fehl.
Nach Umsetzung bestanden Build, Race-Tests, vet und Module-Prüfung. Kontrollierte
Provider-Doubles prüfen den Browser-/State-/Owner-Ablauf ohne echte Kontofreigabe.

Sechs zusätzliche Mutationen wurden durch Testfehler erkannt: Entfall von
Einrichtungsschlüssel, CSRF, Origin-Prüfung, Ablauf, einmaliger Callback-Verwendung
und verschlüsselter Konto-ID-Bindung. Insgesamt sind es jetzt 20 gezielte Mutationen.
Zusätzliche Fuzz-Ziele prüfen Zugangspaket und OAuth-Callback jeweils 30 Sekunden.
Die CI wiederholt diese Prüfungen, einschließlich Vulnerability- und Secret-Scans.

Auf Render wird `PCLOUD_SETUP_ENABLED=false` ausdrücklich gesetzt. Solange App-
Credentials und unabhängige Schlüssel fehlen, sind die gehostete Einrichtung und
MCP-Dateioperationen gesperrt. Ein echter pCloud-Login ist deshalb noch nicht als
bestanden dokumentiert.
