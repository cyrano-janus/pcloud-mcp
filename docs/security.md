# Sicherheitsgrenzen des Release Candidates

## Identität und Policy

Eine Instanz bindet genau eine numerische pCloud-Konto-ID, einen erlaubten
Ordnerbaum und bei HTTP ein einziges OAuth-Subject. Vor jeder Operation wird die
pCloud-Identität geprüft. Fehlender Principal, fremde Eigentümerschaft und Objekte
außerhalb des konfigurierten Baums werden abgewiesen. Geteilte fremde Dateien
sind ausgeschlossen, auch wenn pCloud technisch Zugriff erlauben würde.

Die Ebenen sind getrennt: HTTP validiert die Client-Identität und Scopes;
`policy` entscheidet über Operationen und Schreibfreigabe; `service` prüft
Eigentümerschaft und Ordnerzugehörigkeit; `pcloud` spricht feste Provider-Endpunkte
an. Neue, unbekannte Operationen sind standardmäßig verboten. Ohne
`PCLOUD_ENABLE_WRITES=true` werden Schreibtools nicht registriert.

## Netzwerk und Secrets

pCloud-Requests gehen ausschließlich an `api.pcloud.com` oder `eapi.pcloud.com`.
Tokens stehen in POST-Bodies, nicht in URLs. Provider-Fehlertexte werden nicht
ungefiltert ausgegeben. HTTP-Redirects und Umgebungs-Proxys sind deaktiviert.
DNS-Ergebnisse werden gegen private, lokale und reservierte Adressen geprüft;
der Dialer verwendet anschließend die geprüfte IP ohne erneute Auflösung.
TLS prüft weiterhin den ursprünglichen Host. Keep-alive ist deaktiviert, weil
pCloud persistente Verbindungen an die zuerst authentifizierte Identität binden kann.

Secrets kommen aus Umgebungsvariablen oder ausschließlich gesetzten `_FILE`-
Varianten. Dateien müssen regulär, begrenzt und gegen fremden Zugriff geschützt
sein. Tokens, Inhalte und Dateinamen erscheinen nicht im eigenen Audit-Log.
Der Container enthält keine Secrets. Ein pCloud-Token kann beim Provider weiter
reichende Rechte besitzen als dieser Server erlaubt; die Ordner-Policy ersetzt
keine eingeschränkten Provider-Credentials.

## HTTP und OAuth

Remote-Zugriffe benötigen HTTPS und einen externen Authorization Server.
Validiert werden aktiver Zustand, Issuer, Audience, Subject, Zeitgrenzen und
Scopes über Introspection bei jedem Aufruf. Details stehen im
[Deployment-Vertrag](deployment.md). Fehlende oder widersprüchliche Konfiguration
verhindert den Start. pCloud-Zugangsdaten werden niemals an den Client durchgereicht.

Der Resource-Host muss exakt passen; ein vorhandener Origin muss die öffentliche
Origin treffen. Query-Parameter und mehrdeutige Authorization-Header werden
abgewiesen. HTTP- und Tool-Aufrufe haben Raten-, Parallelitäts-, Größen- und
Zeitgrenzen. Render-Proxy-Modus ist ausdrücklich aktiviert und setzt die
Plattform-TLS-Grenze voraus. Außerhalb dessen erfordert öffentliches HTTP eigene
TLS-Zertifikate; Plaintext ist nur auf explizitem Loopback zulässig.

## Dateioperationen

Text ist nicht vertrauenswürdiger Inhalt. Der Server markiert ihn entsprechend;
der Client muss ihn als Daten behandeln und darf Anweisungen darin nicht befolgen.
Lesen ist auf ausgewählte Textendungen und 128 KiB beschränkt. Suchläufe begrenzen
Tiefe, Ordner, Einträge, Ergebnisse und Metadaten-Aufrufe und melden `truncated`.
Ordnerantworten oberhalb der Provider-Antwortgrenze von 4 MiB schlagen fehl.
Offset-Seiten sind keine konsistente Momentaufnahme.

Uploads akzeptieren nur übergebene Base64-Bytes bis 4 MiB mit passendem SHA-256.
Dieser Hash prüft die Übergabe an den Server; er ist keine unabhängige
Rücklese-Verifikation des gespeicherten pCloud-Inhalts. Kopieren verwendet
`noover=1`, Upload `renameifexists=1` und `nopartial=1`. Mutationsantworten müssen
eigene Objekte am erwarteten Ziel mit gültiger Metadatenform enthalten.

Umbenennen und Verschieben bleiben deaktiviert, weil die dokumentierte
Rename-Operation Zielkonflikte überschreiben kann. Löschen und Überschreiben
sind ohne eigenen bestätigten Workflow grundsätzlich verboten. Es gibt keine
Behauptung, ein Modell könne seine eigene Sicherheitsbestätigung liefern.

Jeder zugelassene Schreibversuch protokolliert vor dem Provider-Aufruf einen
Start und danach ein Ergebnis mit gemeinsamer Request-ID. Scheitert das erste
Audit-Schreiben, erfolgt keine Mutation. Scheitert das letzte Schreiben oder
ist die Provider-Antwort unklar, wird kein sicherer Erfolg behauptet. Automatische
Wiederholung oder Rollback finden nicht statt. stderr allein ist weder dauerhaft
noch manipulationsgeschützt; produktive Aufbewahrung ist Betreiberaufgabe.

## Verbleibende Grenzen

Identitäts-, Metadaten- und Dateioperationen sind separate Provider-Aufrufe.
Gleichzeitige externe Verschiebungen oder Änderungen können daher zwischen
Prüfung und Zugriff liegen; pCloud bietet hier keine atomare Ordner-Policy-
Transaktion. Keine vollständige TOCTOU-Isolation, Mandantenfähigkeit oder
Produktionsreife wird behauptet. Für diese Zusagen sind weitere Architekturarbeit
und echte Provider-/Client-Tests nötig. Der OAuth-Helfer benötigt eine vertrauliche
pCloud-App; der dokumentierte Provider-Flow wird nicht als PKCE-/Refresh-Flow
umgedeutet.

## Primärquellen

- [pCloud allgemeine API-Parameter](https://docs.pcloud.com/methods/intro/parameters.html)
- [pCloud OAuth](https://docs.pcloud.com/methods/oauth_2.0/)
- [Kopieren](https://docs.pcloud.com/methods/file/copyfile.html)
- [Upload](https://docs.pcloud.com/methods/file/uploadfile.html)
- [Umbenennen](https://docs.pcloud.com/methods/file/renamefile.html)
- [Textdateien](https://docs.pcloud.com/methods/streaming/gettextfile.html)
- [MCP-Spezifikation 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28)
