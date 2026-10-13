# Geschützte Video-Vorschau

Stand: 2026-10-13. Optionales Leseprofil; keine automatische Tanzerkennung.

## Bedienung

MCP-Tool `get_video_frames` mit `file_id` und `seconds`, beispielsweise
`{"file_id":123,"seconds":[0,1,2,3]}`. IDs zuerst aus `list_folder` beziehen.
Der Server liefert JPEG-Bilder direkt als MCP ImageContent und Metadaten
`file_id`, `requested_seconds`, `untrusted_content`, `limitation`.
Die Positionen sind Seek-Anforderungen, keine framegenauen Zeitnachweise.
Die Zeitposition muss innerhalb des Videos liegen; fehlende Bilder führen
zu einem Fehler ohne Teilergebnis. Weitere Positionen können separat angefragt
werden. Ein bildfähiger Client kann daraus eine vorsichtige Einschätzung
ableiten; der Server berechnet weder Tanztyp noch Konfidenzwert.

Nach Aktivierung ggf. die ChatGPT-MCP-Verbindung aktualisieren, damit das neue
Tool in deren Katalog erscheint. Bildauswertung im echten ChatGPT-Client ist
separat zu testen; ein erfolgreicher SDK-Test ersetzt diese Abnahme nicht.

## Betrieb und Sicherheitsgrenzen

- Opt-in `PCLOUD_ENABLE_VIDEO=true`, Standard false; Schreibrechte bleiben aus.
- FFmpeg und Linux prlimit müssen im Runtime-PATH vorhanden sein. Render listet
  FFmpeg als Build- und Runtime-Werkzeug für native Dienste. Keine neue
  Render-Ressource und kein kostenpflichtiger Tarif benötigt.
- Ein Videoabruf pro Prozess gleichzeitig, maximal 128 MiB pro Video; Download
  innerhalb des bestehenden 30-Sekunden-Toolbudgets. Große/langsame Downloads
  können auf Free fehlschlagen, ohne die Größenlimits zu erhöhen.
- MP4/video/mp4, MOV/video/quicktime, WebM/video/webm; keine Playlists oder URLs
  als Eingaben. Dateieigentümer und freigegebener Baum gelten weiter.
- pCloud `getfilelink` nur serverintern per POST mit dem pCloud-Token im Body;
  nachfolgender CDN-GET ohne Kontotoken. Capability-Link wird nicht ausgegeben
  oder geloggt. Kein öffentlicher Freigabelink wird erstellt.
- HTTPS-CDN nur unter `.pcloud.com`, öffentliche IPs, gepinnte DNS-Verbindung,
  Redirects aus, begrenzte Antwort und exakte erwartete Dateigröße.
- Private zufällig benannte temporäre Datei (0600); Bereinigung auch bei
  Fehler und Abbruch. Nach Prozess-/Host-Crash ist dies keine sichere Löschung
  des Datenträgers; Render-Dateisystem ist temporär.
- FFmpeg bekommt keine geerbten Umgebungs-Secrets; nur lokale Dateiprotokolle
  und Pipe-Ausgabe, nur MOV/Matroska/WebM-Demuxer. Keine Shellausführung.
- Je Decoder: 384 MiB Adressraum, 15 CPU-Sekunden, 64 Dateideskriptoren,
  1 MiB Dateiausgabelimit, 32 MiB je FFmpeg-Allokation. Gesamtextraktion
  maximal 20 Sekunden. JPEG-Pipe-Ausgabe zusätzlich auf 128 KiB je Bild begrenzt.
- Ausgabe maximal vier Bilder, maximal 640×360, kein Ton. Bilder und eingebetteter
  Text sind nicht vertrauenswürdige Daten, keine Anweisungen.
- Decoder-Prozesslimits ersetzen keine vollständige OS-Sandbox. FFmpeg-Version
  und Sicherheitsupdates verwaltet im nativen Render-Runtime die Plattform;
  das Scratch-Dockerimage enthält keinen Decoder und unterstützt diese Option
  nicht ohne zusätzliches gehärtetes Runtime-Image.

## Nachweis

Tests vor Implementierung: fehlendes Video-API/Media-Paket verursachte den
beabsichtigten roten Testlauf. Danach echte FFmpeg-Extraktion eines synthetischen
Videos, MCP-SDK-End-to-End mit zwei ImageContent-Blöcken, negative Owner-/Scope-/
Typ-/Größen-/Positionsfälle, CDN-Host-/Downloadgrenzen und Tempdatei-Bereinigung.
Zusätzliche gezielte Mutationen: Typ-, Größen-, Host-, Frameanzahl- und
Bildausgabegrenze. Neuer Fuzzer `FuzzFramePositions`.

CI installiert FFmpeg/util-linux, führt reale Decoder-Tests, Race-Tests, Vet,
gezielte Mutationen und Fuzzing aus. Das Standardprofil bleibt fünf Tools;
mit expliziter Aktivierung werden sechs ausschließlich lesende Tools getestet.
Lokal bestanden: `make check` einschließlich Race-Tests, Vet und stdio-Smoke;
37 gezielte Mutationen erkannt; neuer Positions-Fuzzer 15 Sekunden,
1.133.498 Ausführungen; Gitleaks-Arbeitsverzeichnisscan ohne Befund.
Dies ist kein allgemeiner Mutationsscore oder Sicherheitsbeweis.

Code-Commit: `22b40644a0c31a804c275db3905513a002c02a1c`.
[GitHub CI 38386423571](https://github.com/cyrano-janus/pcloud-mcp/actions/runs/38386423571):
vollständig erfolgreich, einschließlich 8 Fuzz-Ziele, 37 Mutationen,
Vulnerability-/Secret-Checks und Container-Build.

Bestehender Render-Dienst `srv-db4jqecs728c73fj5qd0`, Workspace
`tea-db4bm53l550s73b87vl0`, weiterhin Free/Frankfurt/native Go.
`PCLOUD_ENABLE_VIDEO=true` wurde nach erfolgreicher CI per zusammenführendem
Env-Update gesetzt; andere Env-Werte wurden nicht ausgelesen oder ersetzt.
Aktivierungsdeployment `dep-db70ubbbc2fs73eu3vmg`: live, abgeschlossen
2026-10-13 11:00:33 UTC. URL: https://pcloud-mcp-ppew.onrender.com/mcp.

[Öffentlicher HTTP-Testbericht](video-http-verification.json):
HTTPS-Health und OAuth-Metadaten 200, fehlender/ungültiger Bearer 401,
fremder Origin 403, Query-Token 400. Bestehender authentifizierter Konto-
und Ordnerzugriff erfolgreich. Die echte Privatvideo-/ChatGPT-Bildabnahme
bleibt offen: Der Tool-Katalog der vorhandenen ChatGPT-Verbindung enthielt
in dieser Sitzung weiterhin nur die bisherigen fünf Tools; das neue Tool
muss im Client-Katalog aktualisiert werden. Decoder-Präsenz und Verarbeitung
des realen Privatvideos auf Render sind damit noch nicht als getestet behauptet.

## Primärquellen

- https://docs.pcloud.com/methods/streaming/getfilelink.html
- https://render.com/docs/native-runtimes
- https://modelcontextprotocol.io/specification/2026-07-28/server/tools
