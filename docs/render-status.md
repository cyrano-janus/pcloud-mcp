# Render-Status, geprüft am 9. Oktober 2026

## Tatsächlicher Dienst

| Angabe | Verifiziert |
| --- | --- |
| Workspace | `pcloud-mcp` (`tea-db4bm53l550s73b87vl0`) |
| Service | `pcloud-mcp` |
| Service-ID | `srv-db4jqecs728c73fj5qd0` |
| URL | https://pcloud-mcp-ppew.onrender.com |
| Dashboard | https://dashboard.render.com/web/srv-db4jqecs728c73fj5qd0 |
| Region / Laufzeit-Tarif | Frankfurt / Free |
| Erstes Deployment | `dep-db4jqf4s728c73fj5sm0` |
| Erster deployter Commit | `57e81a3cdd0350ffc7b6e891e85bd85171ff435a` |
| Erster Deployment-Status | `live`, beendet 2026-10-09 19:15:52 UTC |
| Laufzeit | Native Go, `cmd/pcloud-remote` |

Die Service-Inventarisierung hatte zuvor nur `null` geliefert. Der Besitzer hat
anschließend ausdrücklich bestätigt, dass lediglich Account und Workspace
eingerichtet waren. Danach wurde der einzelne Free-Web-Service mit der bestätigten
Workspace-ID angelegt. Es wurden keine kostenpflichtigen Laufzeit-Ressourcen angelegt.

## Tatsächliche Live-Prüfung

Der [JSON-Bericht](render-readiness-report.json) enthält Zeitpunkt und Zuordnung.
Geprüft wurde mit TLS-Zertifikatsprüfung über den Plattform-Egress-Proxy und ohne
Redirect-Folgen:

| Prüfung | Ergebnis |
| --- | --- |
| HTTPS `GET /healthz` | 200 |
| `GET /mcp` | 503, Zugriff gesperrt |
| `POST /mcp` | 503, Zugriff gesperrt |
| `DELETE /mcp` | 503, Zugriff gesperrt |

Dieser Dienst bietet ausschließlich Bereitschaft. Es sind keine pCloud-Tokens
hinterlegt und keine Dateioperationen öffentlich verfügbar. Der vollständige
Streamable-HTTP-/OAuth-Code ist im Repository vorhanden, aber hier noch nicht
aktiviert. Reale OAuth-Anmeldung, Widerruf, pCloud-Zugriff und Client-Interoperabilität
sind weiterhin nicht nachgewiesen.

## Tatsächliche CI/CD-Einstellungen und Abweichungen

Render ist mit Repository `cyrano-janus/pcloud-mcp`, Branch `main`, verbunden.
Die direkte Service-Anlage setzt `autoDeploy=yes` und `autoDeployTrigger=commit`.
Damit erfolgen automatische Deployments bei Commits; das Warten auf erfolgreiche
GitHub-Prüfungen ist **noch nicht aktiv**. Die Blueprint-Vorlagen enthalten
`checksPass`; sie wurden durch die direkte Tool-Anlage nicht angewendet.

Die Render-API-Antwort zeigt außerdem `healthCheckPath` leer. `/healthz` existiert
und wurde extern geprüft, ist jedoch noch nicht als Plattform-Health-Check
konfiguriert. Die installierten Plugin-Tools bieten keine Änderung dieser beiden
Service-Einstellungen. Im Dashboard sind noch `After CI Checks Pass` und
Health-Check-Pfad `/healthz` einzustellen bzw. der Blueprint anzuwenden.

Render verwendete im ersten Build Go 1.27.1; die CI verwendet Go 1.26.9. Für den
vollständigen Dienst pinnt der vorbereitete Docker-Build seine Toolchain und sein
Basisimage. Der native Bereitschafts-Build ist kein Beleg für ein identisches
Container-Build-Ergebnis.

## Nächste Freigabeschritte

1. pCloud-OAuth-App einrichten und das Konto manuell freigeben. Zugangstoken,
   Region, bestätigte Konto-ID und erlaubte Ordner-ID ausschließlich in Render
   bzw. geschützten Secret-Dateien konfigurieren, niemals im Repository oder Chat.
2. Externen OAuth-Authorization-Server gemäß [Deployment-Vertrag](deployment.md)
   einrichten. Die Audience muss exakt
   `https://pcloud-mcp-ppew.onrender.com/mcp` sein.
3. Bestehenden Dienst kontrolliert auf die Docker-Konfiguration von
   `render-secure.yaml` umstellen; keinen zweiten Dienst anlegen. Zunächst lesend.
4. OAuth-Metadata und Token-/Origin-Abweisung mit `verify_remote.py --mode protected`
   prüfen, dann echten Client-Login, Tokenablauf, Widerruf und erlaubte
   Dateioperationen abnehmen. Erst anschließend den Datei-Endpunkt freigeben.

Der vorhandene [CI-Nachweis](https://github.com/cyrano-janus/pcloud-mcp/actions/runs/37934045114)
für den ersten deployten Commit ist erfolgreich. Dokumentations-Commits und ihre
automatischen Folge-Deployments erhalten eigene IDs; das oben genannte Deployment
bleibt ausdrücklich der erste geprüfte Stand.
