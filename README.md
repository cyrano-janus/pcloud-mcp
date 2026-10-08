# pcloud-mcp

Sicherer MCP-Server in Go für pCloud: Dateien über ChatGPT, Claude und andere MCP-Clients verwalten – lokal oder remote.

**Status:** 0.2.0-draft (Spezifikation; noch keine Implementierung).

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
