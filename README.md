# pcloud-mcp

MCP-Server in Go für den kontrollierten Zugriff auf das eigene pCloud-Konto.

**Status: 0.3.0-rc.1.** Implementiert sind lokale Nutzung über stdio und ein
OAuth-geschützter HTTP-Transport. Die Produktspezifikation bleibt
[0.2.0-draft](SPEC.md). Ein echter pCloud-Konto- und ChatGPT/Claude-End-to-End-Test
steht noch aus; dieser Release Candidate ist keine produktive Freigabe.

Go **1.26.9**, offizielles **MCP-Go-SDK v1.8.0**, MCP **2026-07-28**.
Der Server aktiviert ausschließlich diese MCP-Version. Ältere Clients sind damit
nicht automatisch kompatibel. Versionsstand zuletzt am 9. Oktober 2026 geprüft.

## Funktionen

| Tool | Verhalten |
| --- | --- |
| `whoami` | Prüft und liefert die konfigurierte Kontoidentität |
| `list_folder` | Eigene Einträge im erlaubten Ordnerbaum, begrenzte Seiten |
| `get_file_info` | Metadaten einer eigenen Datei |
| `search_files` | Suche nach Name, relativem Pfad, Endung, MIME-Typ, Größe und Datum |
| `read_text_file` | Unterstützte Textdateien bis 128 KiB; Inhalt als nicht vertrauenswürdig markiert |
| `create_folder` | Optional: neuen Ordner erstellen |
| `copy_file` | Optional: kopieren mit pCloud-seitigem Überschreibschutz |
| `upload_file`, `save_artifact` | Optional: übergebene Bytes bis 4 MiB mit SHA-256-Prüfung speichern |

Schreiben ist standardmäßig deaktiviert. Bei Upload-Namenskonflikten wählt pCloud
einen neuen Namen; maßgeblich ist der zurückgegebene Name. Umbenennen, Verschieben,
Überschreiben und Löschen sind nicht freigeschaltet. Eine vorgeschaltete
Existenzprüfung allein würde konkurrierendes Überschreiben nicht verhindern.

## Lokal starten

```sh
make build
python3 scripts/smoke_test.py ./bin/pcloud-mcp
```

Ohne Zugangsdaten läuft nur das Protokollfundament, ohne Datei-Tools. Für echte
pCloud-Nutzung eine eigene vertrauliche pCloud-OAuth-App mit Callback
`http://127.0.0.1:8976/callback` anlegen. Client-ID und Client-Secret in der lokalen
Umgebung setzen (`PCLOUD_CLIENT_ID`, `PCLOUD_CLIENT_SECRET` oder
`PCLOUD_CLIENT_SECRET_FILE`), dann:

```sh
./bin/pcloud-auth
```

Der Helfer zeigt die Freigabe-URL, prüft Callback-State und Region und speichert
das Zugangstoken standardmäßig in `secrets/pcloud-token` mit Dateimodus 0600.
Er gibt die Konto-ID und die benötigten Konfigurationsnamen aus, niemals das Token.
Die Region wird aus dem geprüften pCloud-Callback übernommen. Der Helfer benötigt eine lokale
Browser-Freigabe; er ist kein Remote-OAuth-Anmeldedienst.

Folgende Umgebung für `pcloud-mcp` und den lokalen MCP-Client einrichten:

```sh
export PCLOUD_ACCESS_TOKEN_FILE="$PWD/secrets/pcloud-token"
export PCLOUD_REGION=eu
export PCLOUD_USER_ID=123456       # durch die eigene bestätigte Konto-ID ersetzen
export PCLOUD_ROOT_FOLDER_ID=98765 # eigenen freigegebenen Ordner wählen
./bin/pcloud-mcp check
```

`check` verifiziert Konto und Ordner mit lesenden API-Aufrufen. Ordner-ID `0`
erlaubt ausdrücklich die gesamte Kontowurzel; einen eigenen Unterordner bevorzugen.
Ein lokaler MCP-Client startet den absoluten Pfad von `bin/pcloud-mcp` als
stdio-Unterprozess und erhält dieselbe Umgebung. stdout enthält nur MCP-Nachrichten,
stderr Prozessfehler und Audit-Ereignisse.

### Ein Artefakt speichern

```sh
export PCLOUD_ENABLE_WRITES=true
./bin/pcloud-upload -file ./bild.png -folder 98765 -name bild.png
```

Der lokale Helfer liest die Datei, berechnet SHA-256 und ruft `save_artifact` über
das echte MCP-Go-SDK auf. Es werden keine Server-Dateipfade oder Download-URLs an
pCloud weitergereicht. Für direkte MCP-Aufrufe: `folder_id`, `name`, `data_base64`
und `sha256` übergeben. Für große Uploads
`PCLOUD_MCP_MAX_FRAME_BYTES=8388608` setzen; der Helfer setzt diese Grenze selbst.
Nach einem unklaren Schreibfehler zuerst pCloud kontrollieren, nicht blind wiederholen.

## Remote auf Render

Zielplattform ist **Render**. [Deployment-Anleitung](docs/deployment.md) und
[render-secure.yaml](render-secure.yaml) enthalten den konkreten Aufbau: Docker, Frankfurt,
kostenloser Einstiegsplan, `/healthz`, Deploy nach erfolgreichen CI-Prüfungen.
Der Free-Plan kann Kaltstarts haben; er ist kein Verfügbarkeitsversprechen.

`render.yaml` erhält den separat vorbereiteten Bereitschaftsdienst
`cmd/pcloud-remote`: `/healthz` antwortet, `/mcp` bleibt mit 503 gesperrt.
Er wird erst nach vollständiger Einrichtung durch den eigentlichen Dienst ersetzt.

Remote-Zugriffe benötigen zusätzlich einen externen OAuth-2.1-Anmeldedienst mit
Token-Introspection. pCloud-Zugangstoken und MCP-Client-Zugangstoken sind getrennt.
Fehlende Konfiguration verhindert den Start. Pro Instanz ist genau ein Besitzer
mit einem Ordnerbaum gebunden; dies ist kein mandantenfähiger Dienst.

## Entwicklung und Nachweise

```sh
make check       # Build, Race-Tests, vet, Module, Protokoll-Smoke-Test
make fuzz        # vier begrenzte Fuzz-Ziele
make mutations   # gezielte sicherheitsrelevante Mutationen
make security    # installierte govulncheck- und gitleaks-Werkzeuge
```

Die CI installiert die gepinnten Sicherheitswerkzeuge und baut zusätzlich den
Docker-Container. Details: [Testnachweis](docs/verification.md),
[Sicherheitsgrenzen](docs/security.md).

Projektstruktur: `cmd/` enthält Server, OAuth- und Upload-Helfer;
`internal/config` Konfiguration, `model` Datentypen, `netguard` Netzwerkgrenzen,
`pcloud` den Provider-Adapter, `policy` die Zugriffsregeln, `service` die
Anwendungsfälle, `server` die MCP-Tools und `remote` HTTP/OAuth. Transport,
Policy und pCloud-Adapter bleiben getrennt.
