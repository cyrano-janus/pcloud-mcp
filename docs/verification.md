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

## Schutzkorrekturen vom 12. Oktober 2026

Neue Regressionstests wurden vor der Änderung ausgeführt: öffentliche Formulare
füllten die OAuth-Sitzungstabelle und lieferten 503; nach unauthentifizierten
MCP-Anfragen lieferte der Health-Endpunkt 429. Beide Tests schlugen erwartungsgemäß
fehl. Nach der Korrektur reservieren öffentliche Formulare keine Sitzung;
erst gültiger Betreiber-Schlüssel und browsergebundener HMAC-Formularnachweis
erlauben eine Sitzung. Falsche Cookies, manipulierte und abgelaufene Nachweise
sowie Nachweise aus einem vorherigen Prozess werden abgewiesen. Health-Probes
bleiben vom MCP-Budget getrennt, behalten aber Host-/Origin-/Query-Prüfungen.

Lokal bestanden `make check`, die Python-Tests und alle **22** gezielten
Mutationen. Zwei neue Mutationen entfernen die Formular-MAC-Prüfung bzw. die
Health-Ausnahme und werden durch Testfehler erkannt. `govulncheck ./...` meldete
keine bekannten erreichbaren Schwachstellen. Nach der unabhängigen Prozess-
Schlüssel-Ergänzung bestanden die Onboarding-Race-Tests und vet erneut.

Der Betreiber meldete die pCloud-App-Freigabe. Reale App-Credentials,
Konto-Freigabe, externer MCP-OAuth-Anmeldedienst und Client-Abnahme bleiben offen.

## Formular-Origin und sichere Startdiagnose

Startdiagnosen erlauben nur feste Konfigurationsnamen; unbekannte Fehler und
unbekannte Feldnamen werden maskiert. Race-Tests sowie die Mutation, die rohe
Fehlertexte ausgibt, prüfen diese Grenze.

Der zusätzliche Formular-Test schlug zunächst fehl, weil das Einrichtungsformular
`no-referrer` verwendete. Es verwendet jetzt `same-origin`; Provider-Redirect und
OAuth-Ergebnis behalten `no-referrer`. Fehlende, fremde und `null`-Origins bleiben
gesperrt. Die betroffenen Race-Tests, vet und alle **24** gezielten Mutationen
bestanden lokal. Ein realer Browser-Login beim Provider bleibt eine gesonderte
Abnahme; die Handler-Tests behaupten keine erfolgreiche Kontofreigabe.

Provider-Fehlerdiagnosen unterscheiden Token-Austausch, `userinfo` und Konto-/
Regionsbindung. Nur feste Kategorien und numerische HTTP-/Provider-Fehlerstatus
werden geloggt. Tests prüfen unbekannte Fehlertexte und Stufen auf Redaktion;
HTTP-Fehler erhalten die Transport-Klassifizierung und den Hinweis auf unklare
Schreibergebnisse. Die betroffenen Race-Tests und alle **25** Mutationen bestanden
lokal; der neue Fehlerausgabe-Mutant wurde durch Testfehler erkannt.

Der Token-Decoder unterstützt sowohl `uid` aus der Methodendokumentation als auch
`userid` aus der [offiziellen SDK-Referenz](https://github.com/pCloud/pcloud-sdk-js/blob/master/docs/api-oauth.md).
Regressionstests schlugen vor der Änderung für `userid` fehl. Positive Integer-
IDs sind erforderlich; widersprüchliche Aliase, Null-, String- und negative Werte
werden abgewiesen. Race-Tests und alle **26** Mutationen bestanden lokal,
einschließlich der Mutation, die widersprüchliche Konto-IDs akzeptiert. Das
bestehende Provider-JSON-Fuzz-Ziel prüft zusätzlich den Token-Decoder.

## Native Aktivierung und JWT-Anmeldung vom 12. Oktober 2026

Die reale gehostete pCloud-Freigabe wurde durch die vom Betreiber gezeigte
Erfolgsseite bestätigt. Zugangspaket und Schlüssel sind laut Betreiber in Render
hinterlegt; die importierte Kombination und der erlaubte Stammordner bleiben
bis zur Aktivierung und zum echten Zugriffstest unbestätigt.

Neue Aktivierungstests wurden vor der Implementierung hinzugefügt (anfangs
fehlende Funktion). Der bestehende native Prozess kann nach vollständiger
OAuth-Konfiguration auf read-only MCP wechseln. Öffnung ohne Flag, fremde
Runtime, paralleles Setup, Schreibfreigabe, defektes Paket, fremder Besitzer und
fremder Verifier-Host werden abgewiesen. Ein eigener Preflight-Test verhindert
Aktivierung mit fremdem pCloud-Konto oder nicht eigenem Stammordner.

JWT-Tests verwenden echte RSA-Schlüssel, signierte Tokens und ein simuliertes
JWKS-Transportziel. Falsche Signatur, fremder Benutzer, Audience und Issuer,
fehlendes/futures `iat`, Ablauf, überlange Laufzeit, zukünftiges `nbf`, fehlender
Lesescope, mehrdeutige Verifier-Konfiguration und ungültige/überlange JWKS sind
abgewiesen. Diese Fixtures sind kein echter Descope-/ChatGPT-Interop-Nachweis.

Lokal bestanden `make check` (Build, sämtliche Race-Tests, vet, Module,
MCP-Protokoll-Smoke-Test), zusätzliche betroffene Race-Tests nach dem
Preflight-Test, `go vet ./...`, `govulncheck ./...` ohne bekannte Schwachstellen
und alle **32 gezielten Mutationen**. Ein zunächst überlebender Aktivierungs-
Mutant führte zu einem vollständigen Konfigurationsfall mit ungültigem Flag;
danach wurde er durch einen Testfehler erkannt. Ein syntaktisch ungültiger
Setup-Mutant wurde so korrigiert, dass echte Testfehler statt Buildfehler zählen.

Das neue JWT-Fuzz-Ziel bestand lokal 15 Sekunden mit **191.418 Ausführungen**.
Die CI ergänzt es als siebtes 30-Sekunden-Fuzz-Ziel und führt alle 32 Mutationen
sowie die vorhandenen Dependency-, Secret- und Container-Prüfungen aus.
Der CI-Erfolg dieses neuen Commits ist separat zu prüfen.

Für JWTs ist sofortiger Widerruf nicht nachgewiesen: lokale Prüfung kann ein
vorher ausgestelltes Token bis zum Ablauf (höchstens 15 Minuten ab `iat`)
akzeptieren. Reale Anmeldung, PKCE, Anbieter-Refresh, Widerruf, Schlüsselrotation
und Read-only-Zugriff auf `mcp-test` sind noch Freigabekriterien.


## Secret-Scanner-Korrektur vom 13. Oktober 2026

CI-Lauf 38307081045 bestand Build, Race-Tests, vet, sieben Fuzz-Ziele, alle
32 Mutationen und beide Vulnerability-Scans. Gitleaks beanstandete ausschließlich
einen deterministischen Dummy-Verschlüsselungsschlüssel in
`cmd/pcloud-remote/activation_test.go`, Commit `ee57fe2`, Zeile 40. Er wird nur
für das synthetische Test-Token `test-provider-token` verwendet, nicht für eine
Produktiv-Credential. Es liegt damit kein Nachweis eines realen Secret-Leaks vor.

Die Fixture baut ihren Testwert jetzt zur Laufzeit zusammen. `.gitleaksignore`
enthält ausschließlich den vollständigen Fingerprint dieses historischen
Testfundes (Commit, Pfad, Regel, Zeile). Keine Datei, kein Testverzeichnis und
keine Erkennungsregel wird pauschal ausgenommen. Der vollständige Git-History-
Scan mit den bestehenden Standardregeln wird weiterhin in CI ausgeführt.


## Reale ChatGPT-Abnahme vom 13. Oktober 2026

CI 38377874006 ist erfolgreich. Der Betreiber meldete die ChatGPT-Anmeldung als
fehlerfrei; danach waren die fünf Lese-Tools in dieser Sitzung verfügbar.
Tatsächlich ausgeführte OAuth-geschützte Aufrufe bestätigten das konfigurierte
pCloud-Konto, listeten eine paginierte Stichprobe von fünf eigenen WEBP-Dateien
im erlaubten Ordner und lasen die Metadaten einer daraus ausgewählten Datei.
Ein kontrollierter Zugriff auf Ordner-ID 0 außerhalb des erlaubten Stammordners
wurde mit `object outside the permitted owned folder tree` verweigert.
Keine Bilder wurden heruntergeladen, keine Dateien geschrieben.

Dies belegt reale ChatGPT-Authentifizierung und diese Lese-/Policy-Fälle,
keine vollständige Penetrationsprüfung oder erfolgreiche Refresh-/Widerrufs-
Abnahme. Suche, Textlesen, Schlüsselrotation, Refresh und Claude bleiben offen.
Aktivierungsdeployment und externe HTTP-Negativtests: [Render-Status](render-status.md).
