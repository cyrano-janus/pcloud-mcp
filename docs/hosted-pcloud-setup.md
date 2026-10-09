# pCloud direkt über den gehosteten Dienst verbinden

Kein lokaler Helfer und keine lokale IP sind nötig. Die Einrichtung läuft im
Browser über HTTPS. Sie verbindet ausschließlich pCloud; die Anmeldung eines
MCP-Clients über den externen Authorization Server bleibt ein eigener Schritt.
Dateioperationen sind im aktuell betriebenen Bereitschaftsdienst weiterhin gesperrt.

## Einmalige Vorbereitung

In [pCloud My Apps](https://docs.pcloud.com/my_apps/) die eigene App registrieren.
Für diesen Render-Dienst muss die Redirect-URI exakt lauten:

```text
https://pcloud-mcp-ppew.onrender.com/oauth/pcloud/callback
```

Im bestehenden Render-Service unter Environment konfigurieren:

| Variable | Bedeutung |
| --- | --- |
| `PCLOUD_CLIENT_ID` | App-ID von pCloud |
| `PCLOUD_CLIENT_SECRET` | App-Secret von pCloud, ausschließlich geschützt hinterlegen |
| `PCLOUD_SETUP_SECRET` | Eigener zufällig erzeugter Einrichtungsschlüssel, 32–128 druckbare ASCII-Zeichen ohne Leerzeichen |
| `PCLOUD_TOKEN_ENCRYPTION_KEY` | Separater zufällig erzeugter Verschlüsselungsschlüssel, gleiche Längengrenzen |
| `PCLOUD_SETUP_ENABLED` | Erst bei vollständiger Konfiguration ausdrücklich `true` |

Die beiden neuen Schlüssel mit einem sicheren Passwortmanager zufällig erzeugen;
keine selbst gewählten Passwörter und niemals denselben Wert verwenden. Dies
geht auch am Telefon. Schlüssel, Client-Secret und Tokens nicht im Chat teilen.
Wenn `PCLOUD_USER_ID` bereits gesetzt ist, muss die Freigabe genau zu diesem
Konto passen. Ungültige oder unvollständige aktivierte Konfiguration verhindert
den Prozessstart. Deshalb alle Werte zusammen speichern und deployen.

## Freigabe im Browser

1. Im selben Browser [pCloud verbinden](https://pcloud-mcp-ppew.onrender.com/setup/pcloud)
   öffnen. Solange die Einrichtung deaktiviert ist, liefert die Seite 503.
2. Den Einrichtungsschlüssel eingeben. Der Browser wird ausschließlich zur
   offiziellen pCloud-Freigabe weitergeleitet. Dort mit dem gewünschten Konto
   anmelden und Zugriff bewusst erlauben.
3. pCloud führt zurück auf den HTTPS-Callback. Der Server prüft Browser-Sitzung,
   State, Region und Kontoidentität und tauscht den einmaligen Code serverseitig
   gegen ein Token. Das Token erscheint weder als Klartext auf der Seite noch
   in den eigenen Anwendungslogs.
4. Die Ergebnis-Seite zeigt ein **verschlüsseltes Zugangspaket**. Dieses in Render
   als `PCLOUD_CREDENTIAL_ENVELOPE` speichern. Den angezeigten Wert für
   `PCLOUD_USER_ID` übernehmen, `PCLOUD_REGION` passend setzen und
   `PCLOUD_ROOT_FOLDER_ID` ausdrücklich für einen eigenen erlaubten Ordner wählen.
   `0` würde die gesamte Kontowurzel freigeben.
5. `PCLOUD_TOKEN_ENCRYPTION_KEY` behalten. `PCLOUD_SETUP_ENABLED=false` setzen und
   den Einrichtungsschlüssel anschließend entfernen. Alte `PCLOUD_ACCESS_TOKEN`
   bzw. `PCLOUD_ACCESS_TOKEN_FILE` entfernen; verschlüsseltes Paket und Klartext-
   Token sind bewusst nicht gleichzeitig zulässig.

Der Token wird im Prozess entschlüsselt und bleibt an Region und konfigurierte
Konto-ID gebunden. Die Anwendung schreibt keine Credentials automatisch in das
Render-Konto. Das einmalige Übernehmen des verschlüsselten Pakets vermeidet einen
zusätzlichen Render-Verwaltungsschlüssel mit weitreichenden Rechten im Dienst.
Ein Verlust des Verschlüsselungsschlüssels erfordert eine neue pCloud-Freigabe.

## Sicherheitsgrenzen

Einrichtung ist standardmäßig aus, erfordert einen eigenen geheimen Schlüssel,
exakten Host/Origin und einen CSRF-Wert. Secure-/HttpOnly-/SameSite-Lax-Cookies
binden die Freigabe an den Browser. Sitzungen enden nach fünf Minuten; State wird
vor dem Token-Austausch atomar verbraucht. Andere Provider-Hosts, doppelte Callback-
Parameter und abweichende konfigurierte Konto-IDs werden abgewiesen.

Der verschlüsselte Token nutzt AES-GCM mit zufälligem Nonce und Versions-/Zweck-
Bindung. Manipulationen und falsche Schlüssel werden erkannt. Token und Client-
Secrets stehen nur in serverseitigen POST-Anfragen; die Ergebnis-Seite enthält
nur Ciphertext sowie Region und Konto-ID. Keine Drittressourcen oder Scripts
werden eingebunden; Cache, Referrer und Einbettung sind unterbunden.

Render Free hat keine dauerhaft verlässliche lokale Dateispeicherung. Sitzungen
liegen deshalb nur kurz im Arbeitsspeicher: Neustart, Kaltstart oder ein Wechsel
auf mehrere Instanzen kann die Einrichtung abbrechen, ohne die Schutzprüfungen
zu lockern. Dann erneut beginnen. Die Implementierung ist für eine Instanz.

Absolute Risikofreiheit wird nicht behauptet: Ein verlorener Einrichtungsschlüssel
kann fremde Einrichtungsversuche ermöglichen. Zugriff auf Token **und**
Verschlüsselungsschlüssel ermöglicht Kontozugriff. Der dokumentierte pCloud-Code-
Flow bietet keinen hier behaupteten PKCE-/Refresh-Mechanismus. Der Callback-Code
steht wie beim Provider vorgesehen kurz in der URL und kann in Plattform-
Request-Logs erscheinen; er ist einmalig und wird sofort verbraucht. Die App
protokolliert ihn nicht. Deshalb Einrichtung anschließend wieder deaktivieren.

## Tests und verbleibende Abnahme

Tests prüfen falsche Einrichtungsschlüssel, CSRF und Origin, fehlende Browser-
Cookies, abgelaufene Sitzungen, State-/Replay-Abweisung, Owner-Bindung, manipulierte
Ciphertexte und falsche Schlüssel. OAuth-Callback und Zugangspaket besitzen
zusätzliche Fuzz-Ziele und gezielte Schutz-Mutationen.

Reale pCloud-Freigabe und der konkrete Browser-Redirect bleiben bis zur eigenen
App-Konfiguration offen. Der gehostete Callback ersetzt nicht die vollständige
MCP-OAuth-Einrichtung oder reale Client-Tests. `/mcp` bleibt im Bereitschaftsdienst
auch nach erfolgreicher pCloud-Freigabe 503.
