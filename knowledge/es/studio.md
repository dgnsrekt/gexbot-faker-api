---
type: Guide
title: El GEX Faker Studio
description: Un recorrido por la interfaz web embebida en /studio y sus siete pantallas — Server, Download, Data library, Live streams, Logs, Monitoring y Settings.
tags: [studio, web-ui, screens, guide, español]
timestamp: 2026-08-11T00:00:00Z
---

# El GEX Faker Studio

**GEX Faker Studio** es una interfaz web embebida en el servidor en **`/studio`** —
un panel de control estilo LM-Studio para manejar el faker sin curl ni variables
de entorno. Lo sirve el mismo binario (sin un proceso aparte) y lee/escribe a
través de los endpoints del plano de control del servidor.

Ábrelo en **http://localhost:8080/studio**. La barra lateral muestra el estado del
servidor en vivo (running/stopped, la fecha que se está reproduciendo, disco
usado) y la URL base accesible.

## Las siete pantallas

### Server
La pantalla por defecto: aquello con lo que hablan tus clientes. Estado del
servidor, la fecha cargada, los modos de datos/caché y la URL base para entregar a
un cliente.

### Download data
Obtén días históricos de mercado desde GexBot — un calendario para elegir solo
**fechas**. La cobertura (tickers, packages, categories) es **autoritativa según el
YAML y de solo lectura**: proviene del mismo YAML del downloader que usa el daemon
(`GEXBOT_DOWNLOADER_CONFIG`), mostrado con una etiqueta "Configured by …", así que
las descargas manuales y programadas nunca divergen. Requiere `GEXBOT_API_KEY`
configurada en el servidor y **degrada de forma elegante** ("set GEXBOT_API_KEY")
cuando no está configurada. Las descargas aterrizan como EOD archives; ver
[download data](download-data.md).

### Data library
Los EOD archives en esta máquina. Cada fecha muestra un estado — **`archived`**,
**`ready`** o **`loaded`** — con un botón **Materialize** o **Load**. Aquí es donde
conviertes un archive descargado en una fecha servida; ver
[materialize & load](materialize-load.md). También muestra un sparkline de
cobertura y una insignia de desviación por fila a partir de las verificaciones de
cobertura del daemon. Un control **Load a span** (fechas from/to) carga un rango
contiguo de varios días como un único dataset cross-day en un clic — cada día del
span queda marcado como **loaded**, y la reproducción/seek cruzan los límites de día
(ver [rango multi-día](materialize-load.md)).

### Live streams
Los cinco hubs de WebSocket con conteos de clientes en vivo y grupos activos, más
un **constructor de nombres de grupo**: elige un ticker y un feed, copia el nombre
de grupo exacto al que se suscribe tu cliente. Ver
[WebSocket streaming](websockets.md).

### Logs
Un feed en vivo de los logs del server, downloader y daemon. Respaldado por un
proxy SSE que consulta **Loki** del lado del servidor (`LOKI_URL`); el navegador
nunca habla con Loki. Muestra un mensaje de degradación cuando `LOKI_URL` no está
configurada (p. ej. `go run`).

### Monitoring
Métricas de **Prometheus** (`PROMETHEUS_URL`), renderizadas como paneles nativos
de series temporales — sin Grafana. Tasa de solicitudes/latencia, clientes de
WebSocket, conteos de snapshots por ticker y estado de las reglas de alerta de
Prometheus. Degrada cuando Prometheus no es accesible. Ver
[docker & observability](docker-observability.md).

### Settings
Cada variable de entorno en lenguaje sencillo — la configuración efectiva del
servidor, explicada. Solo lectura. Una sección **Daemon** aparte (Schedule,
Downloads, Packages, Cleanup, Notifications) muestra la configuración efectiva
saneada del daemon + su estado en runtime, obtenida vía proxy desde el daemon a
través de la red de compose (`DAEMON_URL`); muestra **"Daemon unavailable"** cuando
el daemon está caído, y nunca expone secretos (ni API key ni token de ntfy).

## Acceso remoto y autenticación

Por defecto Docker enlaza la API a `127.0.0.1`, así que `docker compose up` nunca
expone el Studio a la LAN. Para acceso remoto configura `HOST_BIND=0.0.0.0` **y**
`STUDIO_AUTH_TOKEN` (y ponlo detrás de TLS). El token protege todas las rutas
`/studio` con HTTP Basic (cualquier usuario, contraseña = el token); vacío =
abierto (desarrollo local). Ver [configuration](configuration.md).
