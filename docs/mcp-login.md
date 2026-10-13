# MCP-Anmeldung für den bestehenden Render-Dienst

Stand: 12. Oktober 2026. Die reale pCloud-Kontofreigabe wurde durch die
Erfolgsseite bestätigt; der Betreiber meldete das Zugangspaket als in Render
gespeichert. Die importierten Credentials wurden anschließend am 13. Oktober mit echten
ChatGPT-Aufrufen für Konto-, Ordner- und Metadatenzugriff erfolgreich geprüft. Die pCloud-Einrichtung ist abgeschaltet. Der zugelassene Ordner ist
`mcp-test`, ID `27076377653`. Das ersetzt keine MCP-Client-Anmeldung.

## Umsetzung und Freigabe

Der bestehende native Dienst `cmd/pcloud-remote` kann jetzt ohne Änderung von
Build-/Startkommando oder zusätzlichen Render-Dienst auf geschütztes MCP
umgeschaltet werden. Ohne `PCLOUD_MCP_ENABLED=true` bleibt `/mcp` mit 503 gesperrt.
Ein ungültiger Wert, parallele Provider-Einrichtung, Schreibfreigabe, fehlende
Credentials oder ungültige OAuth-Konfiguration verhindern den Start.

Bei Aktivierung werden HTTP und Render-TLS-Proxy zwingend verwendet. Vor dem
Öffnen des HTTP-Listeners prüft ein begrenzter, rein lesender pCloud-Aufruf die
Konto-ID und die Eigentümerschaft des Stammordners. Fehlt die Freigabe, startet
der neue Prozess nicht. Der bisherige sichere Render-Prozess kann weiterlaufen.
Es werden dabei weder Bilder geladen noch Dateien verändert.

## Geplanter Anmeldedienst: Descope Free

Die offizielle [Preisseite](https://www.descope.com/pricing) nennt einen
`Free Forever`-Tarif ohne automatische Überschreitungsgebühren, mit einer
OIDC-Federation und begrenzten aktiven Consents/Tokens. Vor Erstellung im
Dashboard den tatsächlichen Free-Tarif und die für MCP benötigten Funktionen
prüfen; keine Pro-Buchung und kein kostenpflichtiges Add-on aktivieren.
Das Descope-Free-Projekt und die MCP-Ressource wurden inzwischen vom Betreiber
angelegt; Discovery und Besitzerbindung wurden konfiguriert.

Descope dokumentiert [MCP-Ressourcen und OAuth](https://docs.descope.com/identity-federation/inbound-apps/authorization-server)
mit Resource Indicators, Discovery, JWKS und Authorization Code Flow.
Der Server unterstützt jetzt signierte RS256-JWTs über einen fest konfigurierten
JWKS-Endpunkt. Kryptographie und Schlüsselprüfung übernimmt `coreos/go-oidc`
v3.21.0 mit `go-jose` v4.1.4; kein eigener Token-Aussteller.
Andere Algorithmen, unbekannte Signaturschlüssel, fremde Issuer/Subjects,
falsche Audience und abgelaufene Tokens werden abgewiesen.

### Noch nötige Betreiberaktionen

1. Ein eigenes Konto unter <https://app.descope.com> anlegen. Im Dashboard ein
   Free-Projekt wählen. Die Verwaltungsanmeldung ist von der späteren
   Endnutzer-Identität getrennt. Eine verfügbare EU-Region bevorzugen; ihre
   Verfügbarkeit im konkreten Free-Projekt ist noch nicht verifiziert.
2. Eine MCP-Ressource `pcloud-mcp` mit genau dieser Resource/Audience anlegen:
   `https://pcloud-mcp-ppew.onrender.com/mcp`. Nur `files:read` freigeben.
3. Den Login-/Consent-Flow für den einzigen Besitzer konfigurieren; z. B. eine
   vom Anbieter unterstützte E-Mail-Anmeldung. Keinen pCloud-Schlüssel verwenden.
   PKCE S256 erzwingen, Authorization Code verwenden, Access-Token-Laufzeit
   höchstens 900 Sekunden. Refresh/Rotation/Widerruf beim Anbieter prüfen.
4. CIMD oder einen vorab registrierten ChatGPT-Client konfigurieren. Bei einem
   vorab registrierten Client ausschließlich die **exakte** Redirect-URI aus
   der ChatGPT-Verwaltungsseite erlauben, keine Wildcards und keine geschätzte
   Callback-ID. Die OAuth-Client-ID muss vom MCP-Ressourcen-Identifier verschieden
   sein, damit ID-Tokens nicht als Ressourcen-Tokens behandelt werden.
5. Issuer und `jwks_uri` aus der echten Discovery übernehmen. Das exakte `sub`
   des Besitzer-Endnutzers aus dessen verifizierter Anmeldung festhalten.
   E-Mail-Adresse, Verwaltungsbenutzer oder pCloud-User-ID sind kein Ersatz für
   dieses OAuth-Subject. Ohne feste Subject-Bindung ist die Aktivierung verboten.

Die Erstkonfiguration ist inzwischen durchgeführt; die Anleitung oben bleibt
für eine erneute Einrichtung gültig. Keine Secrets in Chat oder Git ablegen.

### Konfiguration in Render

Die vorhandenen pCloud-Variablen und den Verschlüsselungsschlüssel beibehalten.
Zusätzlich beim bestehenden Dienst, Workspace `tea-db4bm53l550s73b87vl0`:

| Variable | Wert |
| --- | --- |
| `PCLOUD_SETUP_ENABLED` | `false` |
| `PCLOUD_ENABLE_WRITES` | `false` |
| `PCLOUD_MCP_OAUTH_ISSUER` | Exakter Issuer aus dem echten Projekt |
| `PCLOUD_MCP_JWKS_URL` | Exakte HTTPS-`jwks_uri`, gleicher Host wie Issuer |
| `PCLOUD_MCP_OAUTH_SUBJECT` | Exaktes Endnutzer-Subject des Besitzers |
| `PCLOUD_MCP_ENABLED` | Erst nach vollständiger Konfiguration `true` |

Im JWT-Modus keinen Introspection-Endpunkt setzen. Der bisherige
RFC-7662-Modus bleibt als Alternative erhalten und verlangt weiterhin eigene
vertrauliche Introspection-Credentials. Beide Modi gleichzeitig sind unzulässig.
JWKS sind öffentliche Schlüssel; ein weiteres Server-Client-Secret ist für den
JWT-Modus nicht nötig. OAuth-Client-Secrets für einen gegebenenfalls vertraulichen
ChatGPT-Client gehören in dessen sichere Client-Konfiguration.

### Abnahme nach Aktivierung

- Render-Deployment `live`; HTTPS `/healthz` 200.
- OAuth Protected Resource Metadata nennt exakt `/mcp`, den echten Issuer und
  `files:read`; keine Schreibrechte.
- Ohne Token `/mcp`: 401 plus `WWW-Authenticate`, ohne Zugriff auf pCloud.
- Ein echter ChatGPT-Login mit S256 und korrektem Resource Indicator funktioniert.
- `whoami` entspricht dem gespeicherten pCloud-Konto; `list_folder` funktioniert
  innerhalb `mcp-test`. Eine fremde Ordner-/Datei-ID wird verweigert.
- Falsche Signatur, fremder Benutzer, falsche Audience/Issuer, fehlende Scopes,
  abgelaufener Token, manipulierte Origin und Token in URL sind gesperrt.
- Token-Ablauf, erneute Anmeldung und Anbieter-Widerruf tatsächlich testen;
  keine universelle ChatGPT-/Claude-Kompatibilität aus Unit-Tests ableiten.

## Grenzen

JWTs werden lokal nach Signatur, Issuer, Audience, Benutzer, Laufzeit und Scopes
geprüft. JWKS werden begrenzt auf 64 KiB geladen; nur der konfigurierte HTTPS-Host
wird über den DNS-/SSRF-geschützten Client angesprochen. Schlüssel-Caching und
Rotation übernimmt die Bibliothek. Tokens werden nicht in Fehlertexten ausgegeben.

Ein bereits ausgestellter JWT kann nach Anbieter-Widerruf bis zu seinem Ablauf
(maximal 15 Minuten ab `iat`) lokal weiter gültig sein. Ein sofortiger Widerruf
wird deshalb **nicht** behauptet. Für sofortige Prüfung bleibt Introspection der
passende alternative Modus, soweit der Anbieter den dokumentierten Vertrag
unterstützt. Provider-Schlüsselrotation und echte Client-Refresh-Abläufe sind
noch live zu testen. Ein Neustart verliert keine in Render gespeicherten
Credentials; öffentliche Schlüssel werden erneut geladen.

Referenzen: [OpenAI-Anmeldung](https://developers.openai.com/plugins/build/auth),
[Descope Ressourcenprüfung](https://docs.descope.com/sessions/validation),
[OIDC-Verifier](https://github.com/coreos/go-oidc/releases/tag/v3.21.0).
