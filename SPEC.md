# pcloud-mcp – Produktspezifikation

**Version:** 0.2.0-draft  
**Stand:** 2026-10-08  
**Status:** Produktspezifikation im Entwurf; Teilimplementierung 0.3.0-rc.1 vorhanden, keine produktive Freigabe. Implementierter Umfang und offene Nachweise: [README](README.md).

## 1. Ziel

Ein sicherer, herstellerneutraler MCP-Server in Go, lokal oder bei einem Hostinganbieter betreibbar. Berechtigte Nutzer sollen über ChatGPT, Claude oder andere kompatible MCP-Clients ihr eigenes pCloud-Dateisystem in natürlicher Sprache durchsuchen und verwalten können.

## 2. Funktionsumfang

**Lesen:** Identität ermitteln, Ordner auflisten, Dateimetadaten abrufen, Dateien nach Name, Pfad, Typ, Größe und Datum suchen, unterstützte Textdateien lesen.

**Schreiben:** Ordner erstellen, Dateien hochladen, vom Client bereitgestellte Artefakte (insbesondere erzeugte Bilder) speichern, Dateien kopieren, verschieben und umbenennen.

**Besonders geschützt:** Überschreiben und Löschen erfordern eine eigenständige serverseitige Policy, explizite Bestätigung durch den Nutzer im unterstützten Client-Workflow und nachvollziehbare Audit-Einträge. Fehlt ein geeigneter Bestätigungsmechanismus, ist die Aktion zu verweigern.

**Nicht im Umfang:** Bildinhalt analysieren, Bilder generieren, E-Mails/Nachrichten versenden. Semantische Bildsuche ohne zusätzliche Analyse oder Indizierung wird nicht behauptet. Öffentliche Freigabelinks sind nicht Bestandteil der ersten Ausbaustufen.

## 3. Normative Grundlagen

- Model Context Protocol Specification **2026-07-28**, einschließlich Transport, Tools und Authorization: https://modelcontextprotocol.io/specification/2026-07-28
- Offizielles Go SDK: https://github.com/modelcontextprotocol/go-sdk
- Go SDK **v1.8.0** unterstützt MCP 2026-07-28 und ältere Versionen laut offizieller Release-Dokumentation. Abweichende Client-Unterstützung wird durch echte Kompatibilitätstests festgestellt.
- pCloud API-Dokumentation und OAuth-Anforderungen müssen vor Implementierung der jeweiligen Funktion gegen Primärquellen geprüft werden.
- Relevante OAuth-, PKCE- und HTTP-Sicherheitsstandards in ihrer für die gewählte MCP-Version normativen Fassung.

Die MCP-Spezifikation ist normativ; SDK-Verhalten und Client-Kompatibilität sind gesondert zu testen.

## 4. Architektur

```text
ChatGPT / Claude / anderer MCP-Client
    -> MCP Transport + OAuth
    -> Identitätsprüfung / Mandantenbindung
    -> Policy Enforcement Layer
    -> MCP Tool Handler
    -> pCloud Service (fachliche Operationen)
    -> pCloud API Client (HTTP)
    -> pCloud
```

Tool-Handler dürfen pCloud-HTTP-Aufrufe nicht unmittelbar ausführen. Authentisierung, Autorisierung, Policy, Fachlogik und API-Transport sind getrennt und unabhängig testbar. Jede Operation wird unter der Identität des tatsächlich autorisierten Nutzers ausgeführt.

## 5. Transport und Identität

- Lokal: stdio für geeignete Desktop-Clients; keine Remote-OAuth-Anforderung für ausschließlich lokale stdio-Prozesse, sofern Zugriff und Geheimnisse lokal gesichert sind.
- Remote: MCP-konformer Streamable-HTTP-Endpunkt über TLS mit OAuth-basierter Nutzerauthentisierung und korrekter Resource-/Audience-Bindung.
- PKCE S256, sichere Redirect-Validierung, CSRF-/State-Schutz und strikt begrenzte Token-Laufzeiten nach normativen Anforderungen.
- Kein pCloud-Token in URL, Logs, Tool-Ergebnissen oder Fehlermeldungen.
- Eigene Refresh-Tokens nur mit wirksamer Rotation, Replay-Erkennung und Widerruf bzw. einem anderweitig normkonformen Verfahren; stateless Neuausstellung ohne Invalidierung darf nicht als Rotation bezeichnet werden.
- Multi-User-Hosting erfordert isolierte Benutzeridentitäten, Credentials, Zugriffsrechte und Audit-Daten.
- pCloud EU-/US-Endpunkte und OAuth-Abläufe vor Umsetzung verifizieren; keine stillschweigende Regionsverwechslung.

## 6. MCP-Tools und Risikoklassen

| Tool | Funktion | Risiko |
| --- | --- | --- |
| `whoami` | Autorisierte pCloud-Identität | A: Lesen |
| `list_folder` | Ordnerinhalt | A |
| `get_file_info` | Metadaten | A |
| `search_files` | Begrenzte Suche nach Dateimetadaten | A |
| `read_text_file` | Begrenztes Lesen unterstützter Textdateien | A |
| `create_folder` | Ordner erstellen | B: Änderung |
| `upload_file` | Client-seitig bereitgestellte Datei hochladen | B |
| `save_artifact` | Erzeugtes Artefakt in Zielordner speichern | B |
| `copy_file` | Datei kopieren | B |
| `move_file` | Datei verschieben | B |
| `rename_file` | Datei umbenennen | B |
| `overwrite_file` | Bestehende Datei ersetzen | C: destruktiv |
| `delete_file` | Datei löschen | C |

Dateien sortieren ist zunächst eine durch den Client geplante Folge von Such-, Kopier- und Verschiebeoperationen, kein eigenständiges unkontrolliertes Massen-Tool. Für größere Änderungen ist eine überprüfbare Vorschau mit Einzelzielen vorzusehen.

## 7. Upload und erzeugte Bilder

Ein KI-Client erzeugt ein Bild außerhalb dieses Servers und übergibt es – soweit technisch unterstützt – als Datei/Artefakt zur Speicherung. Die tatsächliche Binärübergabe von ChatGPT und Claude an einen Remote-MCP-Server ist **offen und clientabhängig**; ein erfolgreicher Ende-zu-Ende-Test ist Freigabekriterium. Der Server behauptet keine universelle automatische Übergabe.

- Keine unkontrollierten Downloads frei wählbarer URLs (SSRF- und Datenabflussrisiko).
- Definierte sichere Upload-Schnittstelle; Streaming, Größenlimit, Hash/Integritätsprüfung und Zielprüfung.
- Überschreiben standardmäßig verboten; separate bestätigte Operation erforderlich.
- Dateityp und Metadaten prüfen; Dateiinhalte nicht als Handlungsanweisung interpretieren.
- Keine Bildanalyse im Server.

## 8. Suche

Suche über Dateinamen, Pfade, Erweiterung/MIME-Typ (soweit verlässlich vorhanden), Datum und Größe. Die tatsächlich von pCloud unterstützten Suchfunktionen sind vor der Wahl von API-Suche, begrenzter Traversierung oder optionalem Index zu prüfen. Keine unbegrenzte rekursive Suche ohne Ressourcenbudget.

## 9. Sicherheitsmodell

- **Default deny**, Least Privilege, serverseitige Policy; Modelltexte dürfen Rechte nicht erhöhen.
- Eingaben, Dateiinhalte und MCP-Tool-Ausgaben sind nicht vertrauenswürdig; Schutz vor Prompt Injection.
- Objektidentität und Eigentümerschaft serverseitig prüfen, bevorzugt stabile Datei-/Ordner-IDs.
- Pfadnormalisierung, keine Traversal- oder Cross-User-Zugriffe.
- Feste pCloud-API-Hostallowlist; Redirects und Download-Hosts explizit validieren.
- SSRF-, DNS-Rebinding-, CSRF-, Replay-, Token-Diebstahl- und Cross-Tenant-Abwehr.
- Größenlimits, Timeouts, Rate Limits, parallele Auftragsgrenzen, Abbruch und Backpressure.
- Destruktive Änderungen: Ziel-ID, aktueller Zustand und Vorbedingungen erneut prüfen; bei Konflikt abbrechen. Keine rein vom LLM behauptete Nutzerbestätigung.
- Audit für Schreiboperationen mit Benutzer, Operation, Objekt, Ergebnis, Zeitpunkt und Request-ID; keine Secrets oder Inhalte protokollieren.
- Schutz gegen unautorisierte Tool-Ausführung auf localhost, auch bei lokalem HTTP-Betrieb.
- Dependency-Pinning, regelmäßige Sicherheitsupdates, Secret-Scanning und Vulnerability-Checks.

## 10. Deployment und Konfiguration

Ein Go-Binary, reproduzierbare Builds, Container-Image als Option, Konfiguration über Umgebungsvariablen/Secret-Store (12-Factor). Remote-Betrieb ausschließlich über HTTPS mit korrekter Proxy-/Host-Konfiguration. Readiness/Health ohne Offenlegung sensibler Daten. Sichere Defaults und dokumentierte Ressourcenlimits.

## 11. Qualitätssicherung

- TDD für Domain-, Policy- und API-Logik.
- Unit-, Integrations-, End-to-End- und MCP-Konformitätstests.
- Mutationstests für kritische Policy-/Autorisierungsregeln.
- Go-Fuzzing für URL-, Pfad-, OAuth- und Input-Parser.
- Race Detector, `go vet`, `govulncheck`, Secret-Scanning und Dependency-Prüfung in CI.
- Sicherheitsfälle: fremde Datei-IDs, Prompt Injection, SSRF, Redirects, Token-Leaks, Replay, Größenlimit +1, abgebrochene Streams, Konflikte und fehlende Bestätigung.
- Kompatibilitätstests separat mit ChatGPT und Claude einschließlich Auth, Refresh, Fehlerfällen und Artefaktübergabe.
- Barrierefreie Dokumentation und verständliche Fehlertexte.

## 12. Umsetzungsetappen

1. **M0 – Fundament:** Go-Projekt, offizielles SDK, Konformität, CI, Threat Model, OAuth-Konzept.
2. **M1 – Read-only:** `whoami`, `list_folder`, `get_file_info`, `search_files`, `read_text_file` inklusive Limits und Security-Tests.
3. **M2 – Reversible Änderungen:** `create_folder`, `upload_file`, `save_artifact`, `copy_file`, `move_file`, `rename_file`; clientseitige Artefaktübergabe nachweisen.
4. **M3 – Destruktive Änderungen:** `overwrite_file`, `delete_file` erst nach bestätigtem Policy-/Approval-Workflow und Security-Abnahme.

## 13. Offene Entscheidungen / Nachweise

- pCloud OAuth-Verhalten, Regionen, Scopes und API-Semantik anhand aktueller Primärquellen.
- Suche: native API vs. begrenzte Traversierung/Index.
- Client-spezifische Binär-/Artefaktübergabe von ChatGPT und Claude.
- Verlässliche Nutzerbestätigung im jeweils unterstützten MCP-Client.
- Persistente Speicherung und Widerrufsmodell für OAuth-Refresh-/Grant-Zustand.
- Dateiversionierung und Vorbedingungen, soweit pCloud verfügbar.

**Akzeptanz:** Keine Funktion gilt als fertig, bevor Security-Tests und reale Client-Interoperabilität für ihren Einsatzfall bestanden sind.
