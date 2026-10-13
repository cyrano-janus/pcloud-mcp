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

Deployment und echte Privatvideo-/ChatGPT-Abnahme sind hier erst nach ihrem
Nachweis zu ergänzen.

## Primärquellen

- https://docs.pcloud.com/methods/streaming/getfilelink.html
- https://render.com/docs/native-runtimes
- https://modelcontextprotocol.io/specification/2026-07-28/server/tools
