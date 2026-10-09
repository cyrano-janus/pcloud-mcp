# Bereitstellung auf Render

Ziel: eine einzelne Instanz `pcloud-mcp` in Frankfurt, zunächst nur lesend.
`render-secure.yaml` ist die reproduzierbare Definition des vollständigen Dienstes.
`render.yaml` erhält vorerst den vorhandenen Bereitschaftsdienst (kein Zugriff auf
pCloud, `/mcp` immer 503). Er benötigt keine Geheimnisse. Der Dienst ist erst nutzbar,
wenn pCloud- und OAuth-Zugangsdaten vollständig eingerichtet und geprüft sind.

## Voraussetzungen und konkrete Einrichtung

1. Render mit GitHub verbinden und den gewünschten Workspace bestätigen.
2. Eine pCloud-OAuth-App und das eigene pCloud-Konto mit `pcloud-auth` verbinden
   (siehe README). Konto-ID, Region und erlaubte Ordner-ID festhalten.
3. Einen OAuth-2.1-Authorization-Server konfigurieren. Er muss Authorization Code
   mit PKCE, passende MCP-Discovery und RFC-7662-Introspection unterstützen.
   Dieser Server stellt keine Tokens aus und enthält keine Login-Oberfläche.
4. Über [New Blueprint](https://dashboard.render.com/select-repo?type=blueprint)
   das Repository `cyrano-janus/pcloud-mcp`, `main` und Blueprint-Pfad
   `render-secure.yaml` auswählen. Bestehende Dienste vorher prüfen. Falls der
   Bereitschaftsdienst bereits existiert, diesen kontrolliert auf die Docker-
   Konfiguration umstellen bzw. den bestehenden Blueprint migrieren; keinen
   zweiten Dienst gleichen Namens anlegen. Die Bereitschaftsvorlage erst ersetzen,
   wenn die folgenden Werte vollständig vorliegen.
5. Die vom Blueprint abgefragten Werte im Render-Dashboard setzen. Geheimnisse
   weder ins Repository noch in Chat-Nachrichten schreiben.

| Variable | Wert |
| --- | --- |
| `PCLOUD_REGION` | `eu` oder `us`, passend zum pCloud-Konto |
| `PCLOUD_USER_ID` | Eigene bestätigte numerische Konto-ID |
| `PCLOUD_ROOT_FOLDER_ID` | Eigener freigegebener Ordner; `0` bedeutet Kontowurzel |
| `PCLOUD_ACCESS_TOKEN` | pCloud-OAuth-Zugangstoken |
| `PCLOUD_MCP_OAUTH_ISSUER` | Exakte HTTPS-Issuer-URL |
| `PCLOUD_MCP_INTROSPECTION_URL` | HTTPS-Endpunkt auf demselben Host wie der Issuer |
| `PCLOUD_MCP_OAUTH_CLIENT_ID` | Client-ID des Resource Servers für Introspection |
| `PCLOUD_MCP_OAUTH_CLIENT_SECRET` | Zugehöriges vertrauliches Secret |
| `PCLOUD_MCP_OAUTH_SUBJECT` | Exaktes erlaubtes `sub` des einzigen Besitzers |

Die Resource-URL ergibt sich aus `RENDER_EXTERNAL_URL` plus `/mcp`. Diese exakte
URL als Audience beim Authorization Server einrichten. Bei einer eigenen Domain
`PCLOUD_MCP_PUBLIC_URL=https://deine-domain/mcp` ausdrücklich setzen; Host-Prüfung
und Audience akzeptieren anschließend nur diese Domain.

## Introspection-Vertrag

Authentifizierung zum Introspection-Endpunkt: HTTP Basic, POST-Formular mit
`token` und `token_type_hint=access_token`. Jeder eingehende MCP-Aufruf wird neu
geprüft; es gibt keinen Autorisierungs-Cache.

Akzeptierte Antwort: `active: true`, `iss` gleich konfiguriertem Issuer, `sub`
gleich Besitzer, `aud` als String oder Array mit exakter Resource-URL,
`token_type: Bearer`, `exp` in der Zukunft, positives `iat` höchstens 30 Sekunden
in der Zukunft, Laufzeit `exp-iat` höchstens 900 Sekunden, optionales `nbf` nicht
in der Zukunft. `scope` muss `files:read` enthalten; Schreibzugriffe benötigen
zusätzlich `files:write` UND `PCLOUD_ENABLE_WRITES=true`.

Diese zusätzlichen Claims sind Teil dieses Serververtrags; nicht jeder beliebige
Introspection-Anbieter liefert sie standardmäßig. Vor Freigabe einen echten
Client-Login, Audience, Ablauf und Widerruf testen. Keine statischen pCloud-Tokens
als eingehende MCP-Bearer-Tokens verwenden.

## Plattform und Prüfung

Render terminiert HTTPS. `PCLOUD_MCP_TLS_PROXY=render` setzt zusätzlich
`RENDER=true` und einen gültigen `PORT` voraus und bindet dann `0.0.0.0:$PORT`.
Eigene TLS-Zertifikate sind in diesem Modus unzulässig. Diese Vertrauensgrenze ist
nur für die verwaltete Render-Umgebung gedacht; nicht auf einen ungeschützten
öffentlichen Server kopieren. Host/Origin, OAuth und Request-Limits bleiben aktiv.

`/healthz` zeigt nur Prozessbereitschaft, keine pCloud-Verbindung. Nach erfolgreichem
Build und Deploy prüfen:

```sh
curl --fail https://DEIN-SERVICE.onrender.com/healthz
curl --fail https://DEIN-SERVICE.onrender.com/.well-known/oauth-protected-resource/mcp
curl -i -X POST https://DEIN-SERVICE.onrender.com/mcp
```

Der letzte Aufruf muss ohne Token `401` samt Auth-Challenge liefern. Danach im
kompatiblen MCP-Client anmelden und `whoami`, `list_folder` und eine kontrollierte
Testdatei lesen. Erst dann optional Schreiben aktivieren und Namenskonflikte in
einem eigenen Testordner prüfen. Keine Destruktivoperation ist freigeschaltet.

Der Container läuft als UID 65532 ohne Shell. Geheimnisse gelangen nicht in das
Image; `.dockerignore` enthält ausschließlich Build-Eingaben. Audit-Logs liegen
auf stderr. Eine dauerhaft aufbewahrte, manipulationsgeschützte externe Ablage
ist vor einem produktiven Schreibbetrieb separat einzurichten.

## Alternative: eigener TLS-Host

`compose.yaml` und `deploy/example.env` sind eine separate Vorlage für einen
selbst betriebenen Host. Sie werden von Render nicht ausgewertet. Eigene
TLS-Dateien und Secrets unter `secrets/` bereitstellen, lesbar für UID 65532,
ohne Schreibrechte für Gruppe/andere oder Leserechte für andere. Niemals die
Render-Proxy-Option verwenden, wenn der Listener direkt öffentlich erreichbar ist.

Referenzen: [Render Web Services](https://render.com/docs/web-services),
[Blueprint-Schema](https://render.com/docs/blueprint-spec),
[MCP Authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization),
[RFC 7662](https://www.rfc-editor.org/rfc/rfc7662).
