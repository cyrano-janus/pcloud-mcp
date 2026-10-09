# Render-Status, geprüft am 9. Oktober 2026

## Bestätigte Zuordnung

- Workspace: `pcloud-mcp`
- Workspace-ID: `tea-db4bm53l550s73b87vl0`, ausdrücklich vom Besitzer bestätigt.
- Repository: `cyrano-janus/pcloud-mcp`, `main`.
- Ziel: Frankfurt, Free. Keine kostenpflichtigen Ressourcen autorisiert oder angelegt.

## Tatsächlich beobachtet

GitHub-Head bei Beginn dieser Prüfung: `28f473495825ca804e645adbb4bfc2b33d1cb6a7`.
Der CI-Lauf [37932871047](https://github.com/cyrano-janus/pcloud-mcp/actions/runs/37932871047)
hat einschließlich Container-Build und Startprüfung erfolgreich abgeschlossen.
Streamable HTTP, OAuth-Introspection, Besitzer-/Audience-/Scope-Bindung und
getrennte Policy sind implementiert; siehe `docs/verification.md`.

Die Render-Abfrage `list_services` mit expliziter Workspace-ID gab erfolgreich
auf Transportebene den Inhalt `null` zurück. Auch nach bestätigter Workspace-
Auswahl und mit `includePreviews=true` wurde ausschließlich `null` geliefert.
Dies wurde **nicht** als verlässlich leere Service-Liste gewertet.

Deshalb wurde kein neuer Service angelegt, kein bestehender Service verändert
und kein Deployment ausgelöst. Damit werden unbekannte vorhandene Dienste nicht
überschrieben oder dupliziert.

| Angabe | Verifizierter Stand |
| --- | --- |
| Tatsächliche Service-URL | Nicht ermittelt |
| Service-ID | Nicht ermittelt |
| Deployment-ID | Nicht ermittelt |
| Live-Health-/HTTPS-Prüfung | Nicht ausgeführt, Ziel unbekannt |
| Live-MCP-/OAuth-Sicherheitsprüfung | Nicht ausgeführt |
| Reale pCloud-/Client-Interoperabilität | Noch nicht nachgewiesen |

## Reproduzierbare nächste Prüfung

Nach Klärung des existierenden Dienstes kann lokal oder mit dem GitHub-Workflow
`verify-deployment` getestet werden:

```sh
python3 scripts/verify_remote.py https://TATSAECHLICHER-SERVICE.onrender.com --mode readiness
# Erst nach Aktivierung des vollständig konfigurierten OAuth-Dienstes:
python3 scripts/verify_remote.py https://TATSAECHLICHER-SERVICE.onrender.com --mode protected
```

Die URL oben ist ausdrücklich ein Platzhalter. Der Prüfer validiert HTTPS mit
Zertifikatsprüfung, folgt keinen Redirects und schreibt einen JSON-Nachweis.
Readiness verlangt einen erreichbaren Health-Endpunkt und 503 für MCP.
Protected prüft zusätzlich Resource-Metadata, 401 für fehlende/ungültige Tokens,
403 für fremde Origin und 400 für Query-Tokens. Er führt keine authentifizierten
Dateioperationen aus und bezeichnet solche Tests ausdrücklich als offen.

## Noch notwendige Freigaben und Konfiguration

- Auswertbare Service-Inventarisierung über Render; bei weiterhin defekter
  Integration ist ein vom Nutzer erlaubter Dashboard-Fallback erforderlich.
- pCloud-OAuth-App, manuelle Kontofreigabe, Zugangstoken, Region, Konto-ID und
  freigegebene Ordner-ID. Secrets ausschließlich in Render/Secret-Dateien.
- Externer OAuth-Authorization-Server gemäß `docs/deployment.md`, einschließlich
  Issuer, Introspection-Client, Subject und Audience der tatsächlichen Service-URL.
- Reale Login-, Tokenablauf-, Widerrufs- und MCP-Client-Tests vor Freigabe.

Die Blueprint-Einstellung `autoDeployTrigger: checksPass` ist versioniert.
Ihre Anwendung auf einen tatsächlichen Render-Dienst ist noch nicht verifiziert;
eine aktive CI/CD-Verknüpfung wird deshalb noch nicht behauptet.
