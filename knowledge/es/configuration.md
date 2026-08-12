---
type: Reference
title: Configuración
description: Las variables de entorno que configuran el faker — el servidor (puerto, datos, caché, WebSocket, sync, auth, observabilidad), Docker/compose, el daemon, push por ntfy, limpieza por TTL y la configuración YAML del downloader.
tags: [configuration, environment, env-vars, settings, reference, español]
timestamp: 2026-08-11T00:00:00Z
---

# Configuración

El faker se configura mediante variables de entorno. Esta página cubre las que es
más probable que definas; la pantalla **Settings** de Studio muestra la
configuración *efectiva* del servidor en lenguaje sencillo, y `gexbot.example.env`
es el punto de partida anotado (`cp gexbot.example.env .env`).

## Server

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | 8080 | Puerto del servidor HTTP |
| `DATA_DIR` | ./data | Ruta del directorio de datos |
| `DATA_DATE` | latest | Fecha a cargar (`YYYY-MM-DD` o `latest`) |
| `DATA_MODE` | memory | `memory` (rápido) o `stream` (poca RAM) |
| `CACHE_MODE` | exhaust | `exhaust` (404 al final) o `rotation` (bucle) |
| `ENDPOINT_CACHE_MODE` | shared | `shared` (un cursor por ticker/pkg por key) o `independent` (por categoría) |
| `RANGE_END_POLICY` | clamp | Rango multi-día: al final del span, `clamp` (última fila) o `error` (HTTP 400) |
| `WS_ENABLED` | true | Habilita el streaming por WebSocket |
| `WS_STREAM_INTERVAL` | 1s | Intervalo de broadcast |
| `WS_GROUP_PREFIX` | blue | Prefijo para los nombres de grupo de WebSocket |
| `SYNC_BROADCAST_SYSTEM_ENABLED` | false | Habilita el endpoint SSE de sync-broadcast |
| `SYNC_BROADCAST_SYSTEM_ID` | hostname | Identificador del emisor |
| `SYNC_BROADCAST_SYSTEM_INTERVAL` | 1s | Intervalo de broadcast de posición |

## Docker / compose

| Variable | Default | Description |
| --- | --- | --- |
| `HOST_BIND` | 127.0.0.1 | Dirección de bind del host (`0.0.0.0` para LAN) |
| `HOST_PORT` | 8080 | Puerto del host |
| `DATA_HOST_DIR` | ./data | Directorio del host montado en `/app/data` |
| `PUID` / `PGID` | 1000 | Usuario/grupo con el que corren los contenedores |
| `PROMETHEUS_RETENTION` | 30d | Retención de métricas de Prometheus |

## Auth y observabilidad

| Variable | Default | Description |
| --- | --- | --- |
| `STUDIO_AUTH_TOKEN` | *(empty)* | Gate de HTTP Basic/Bearer para `/studio` **y** las rutas de control que mutan estado (`load`/`reset`); vacío = abierto |
| `LOKI_URL` | http://loki:3100 | Endpoint de Loki para la pantalla Logs |
| `PROMETHEUS_URL` | http://prometheus:9090 | Endpoint de Prometheus para Monitoring |
| `DAEMON_URL` | http://gex-daemon:9091 | Endpoint de diagnóstico del daemon que el Studio hace de proxy para el estado saneado del daemon (Settings). Vacío deshabilita el panel del daemon |
| `GEXBOT_DOWNLOADER_CONFIG` | *(empty)* | YAML del downloader que carga el worker de descargas del Studio — apúntalo al **mismo** archivo que el `DAEMON_CONFIG_PATH` del daemon para que las descargas manuales + programadas compartan una sola autoridad de cobertura. Vacío = descubrimiento en el working-dir (`./configs/default.yaml`) |

## Daemon (descargas programadas)

| Variable | Default | Description |
| --- | --- | --- |
| `DAEMON_SCHEDULE_HOUR` | 17 | Hora del primer intento de EOD (ET) |
| `DAEMON_SCHEDULE_MINUTE` | 5 | Minuto del primer intento de EOD |
| `DAEMON_TIMEZONE` | America/New_York | Zona horaria |
| `DAEMON_RUN_ON_STARTUP` | true | Verificar/descargar al arrancar |
| `DAEMON_RUN_TIMEOUT_MINUTES` | 45 | Duración máxima de una corrida de descarga |
| `DAEMON_CONFIG_PATH` | /app/configs/default.yaml | Config del downloader que usa el daemon |
| `DAEMON_STATE_FILE` | /app/data/.daemon-state | Dónde el daemon registra su progreso |

## Limpieza por TTL (opcional)

| Variable | Default | Description |
| --- | --- | --- |
| `GEXBOT_OUTPUT_AUTO_CLEANUP` | false | Devuelve al archive el JSONL materializado inactivo |
| `GEXBOT_OUTPUT_CLEANUP_AFTER_DAYS` | 7 | Días de inactividad antes de la evicción (debe ser ≥1 cuando la limpieza está activa; `gexbot.example.env` usa 7) |

## Notificaciones push (ntfy)

| Variable | Default | Description |
| --- | --- | --- |
| `NTFY_ENABLED` | false | Habilita las notificaciones push |
| `NTFY_SERVER` | https://ntfy.sh | URL del servidor de ntfy |
| `NTFY_TOPIC` | *(required)* | Nombre del topic |
| `NTFY_PRIORITY` | default | min / low / default / high / urgent |
| `NTFY_TAGS` | package | Tags emoji separados por comas |
| `NTFY_TOKEN` | *(optional)* | Token de acceso para topics privados |

## Downloader (`GEXBOT_API_KEY` + YAML)

La descarga necesita `GEXBOT_API_KEY`. Los tickers/packages/categorías y el tuning
viven en una config YAML seleccionada por `GEXBOT_DOWNLOADER_CONFIG` (o por el flag
`-c/--config` del downloader); copia `configs/default.yaml` a
`configs/custom.yaml`:

```yaml
api:
  api_key: "${GEXBOT_API_KEY}"
  timeout_sec: 300
  retry_count: 3
download:
  workers: 3
  rate_per_second: 2
  resume_enabled: true
output:
  directory: "data"
  auto_convert_to_jsonl: true
```

Nunca imprimas ni hagas commit de valores secretos (`GEXBOT_API_KEY`,
`STUDIO_AUTH_TOKEN`, `NTFY_TOKEN`). Consulta [daemon](daemon.md) para la
programación y [download data](download-data.md) para las rutas de descarga.
