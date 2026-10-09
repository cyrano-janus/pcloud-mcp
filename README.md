# pcloud-mcp

Sicherer MCP-Server in Go für pCloud: Dateien über ChatGPT, Claude und andere MCP-Clients verwalten – lokal oder remote.

**Implementierungsstatus:** 0.2.0-m0a – lokales stdio-Protokollfundament.
Die Produktspezifikation bleibt **0.2.0-draft**. pCloud-Funktionen und Remote-
Hosting sind noch nicht implementiert oder produktiv freigegeben.

## Jetzt nutzbar: lokaler MCP-Protokollserver

Voraussetzung: Go **1.26.9**, für die Python-Testhilfe zusätzlich Python 3.
Das offizielle MCP-Go-SDK ist auf **v1.8.0** gepinnt. Der Server unterstützt in
diesem ersten Schritt ausschließlich MCP **2026-07-28**.

```sh
go mod download
go build -trimpath -o bin/pcloud-mcp ./cmd/pcloud-mcp
```

Ein lokaler MCP-Client startet `bin/pcloud-mcp` als stdio-Unterprozess. Verwende
in seiner Konfiguration den absoluten Pfad des Binary. Ein Client muss die
MCP-Version 2026-07-28 unterstützen; die konkrete Konfigurationsoberfläche hängt
vom Client ab. Kompatibilität mit ChatGPT oder Claude ist noch nicht nachgewiesen.

Für einen direkten Protokoll-Smoke-Test ohne Zugangsdaten:

```sh
python3 scripts/smoke_test.py ./bin/pcloud-mcp
```

Die Antwort enthält die Serveridentität und unterstützte Version. stdout bleibt
dem MCP-Protokoll vorbehalten; Prozessfehler erscheinen auf stderr. Es gibt noch
keine Datei-Tools, keine pCloud-Verbindung und keinen HTTP-Endpunkt.

`PCLOUD_MCP_MAX_FRAME_BYTES` begrenzt eingehende stdio-Frames. Standard: 1048576
Bytes (1 MiB), zulässig: 1024 bis 1048576. Leere, nicht numerische oder außerhalb
des Bereichs liegende Werte verhindern den Start; Werte erscheinen nicht in
Konfigurationsfehlermeldungen. Das SDK setzt die Transportgrenze um.

## Automatisierte Prüfung

```sh
go test -race -count=1 -timeout=90s ./...
go vet ./...
go mod verify
go test ./internal/config -run='^$' -fuzz=FuzzConfig -fuzztime=30s -parallel=2
python3 scripts/mutation_test.py
```

Die Prozessintegrationstests starten ein mit Race Detector gebautes Binary und
prüfen Discovery ohne Legacy-Handshake, eine unbekannte moderne Version,
Ablehnung eines nicht registrierten destruktiven Tools, Frames am Limit und
Limit +1, EOF, Signalbehandlung und Konfigurationsfehler ohne Wertoffenlegung.
Die Mutationstesthilfe prüft drei konkrete Änderungen in temporären Kopien;
sie behauptet keinen allgemeinen Mutation-Score. CI ergänzt `govulncheck` und
Gitleaks mit gepinnten Werkzeugversionen sowie Dependency- und Formatprüfungen.

Siehe [Sicherheitsgrenzen und OAuth-Konzept](docs/security.md).

## Deployment

M0a wird lokal als vom Client gestarteter stdio-Prozess betrieben. Dafür ist
kein Hostingaccount erforderlich. Ein externer Anbieter ist in diesem Repository
noch nicht festgelegt. Remote-Deployment folgt erst nach Implementierung und
Abnahme von Streamable HTTP, TLS und OAuth. Dieses Binary nicht als öffentlichen
HTTP-Dienst oder über eine ungeschützte stdio-Bridge bereitstellen.

## Geplanter Umfang

- Dateien/Ordner auflisten und nach Metadaten suchen
- Metadaten und unterstützte Textdateien lesen
- Ordner erstellen; Dateien hochladen, kopieren, verschieben und umbenennen
- Von KI-Assistenten erzeugte Bilder/Dateien speichern, **sofern** der Client eine sichere Artefaktübergabe unterstützt
- Löschen und Überschreiben erst nach serverseitiger Policy und zuverlässiger Bestätigung

Nicht enthalten: Bildanalyse, Bildgenerierung durch den Server oder Versand über E-Mail/Messenger.

## Technische Grundlage

Go, [offizielles MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk), MCP-Spezifikation 2026-07-28, pCloud API, getrennte Policy Enforcement Layer, Security by Design, TDD, Mutationstests und Fuzzing.

**[Vollständige Produktspezifikation](SPEC.md)**

Roadmap: M0 Fundament → M1 Lesen/Suche → M2 Dateiverwaltung/Upload → M3 geschützte destruktive Operationen.
