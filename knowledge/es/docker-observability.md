---
type: Guide
title: Docker y observabilidad
description: El stack por defecto de docker compose (API, daemon, Prometheus, Loki, Alloy), cómo funcionan el binding de puertos y la autenticación del Studio, y cómo las pantallas Logs y Monitoring del Studio consultan Loki y Prometheus del lado del servidor.
tags: [docker, compose, prometheus, loki, alloy, observability, monitoring, español]
timestamp: 2026-08-11T00:00:00Z
---

# Docker y observabilidad

## El stack de compose

`docker compose up -d` (o `just up`) inicia el **stack por defecto**:

| Servicio | Rol |
| --- | --- |
| `gex-faker-api` | El servidor de la API + Studio |
| `gex-daemon` | Descargas programadas + verificaciones de cobertura |
| `prometheus` | Recolecta métricas de la API (`:9090`) y del daemon (`:9091`) |
| `loki` | Almacena logs |
| `alloy` | Envía los logs de los contenedores a Loki |

Prometheus, Loki y Alloy corren en el stack **por defecto** para que las
pantallas Logs y Monitoring del Studio funcionen de inmediato. Grafana,
Uptime-Kuma y un gateway Caddy fueron removidos (issue #46) — el Studio es el
único panel de control.

Recetas comunes: `just up`, `just down`, `just logs`, `just api-logs`,
`just daemon-logs`, `just restart-api`, `just restart-daemon`.

## Binding de puertos y exposición

El puerto de la API está enlazado a loopback por defecto:

```
"${HOST_BIND:-127.0.0.1}:${HOST_PORT:-8080}:8080"
```

Así que `docker compose up` nunca expone el Studio a la LAN. Para acceso remoto
configura **`HOST_BIND=0.0.0.0`** y **`STUDIO_AUTH_TOKEN`** (y ponlo detrás de
TLS — Basic/Bearer sobre HTTP plano es texto plano). El contenedor siempre
escucha en `8080` internamente.

## Proxies de telemetría del lado del servidor

El navegador nunca habla directamente con Loki ni con Prometheus — el servidor
hace de proxy para ambos, así no hace falta exponer puertos adicionales y los
secretos quedan del lado del servidor:

- **Logs** — `GET /studio/api/logs` es un proxy SSE que consulta Loki
  (`LOKI_URL`, por defecto `http://loki:3100`) y transmite las líneas parseadas.
  Las líneas de log se redactan para las query strings de URLs firmadas en el
  origen (`internal/redact`) y otra vez en el proxy.
- **Monitoring** — `GET /studio/api/metrics/{query,range,alerts}` hacen de proxy
  de PromQL hacia Prometheus (`PROMETHEUS_URL`, por defecto
  `http://prometheus:9090`). La pantalla Monitoring renderiza los paneles de
  forma nativa.

Ambas pantallas **se degradan con elegancia** mostrando un mensaje cuando la URL
de su backend está sin configurar o inalcanzable (por ejemplo, al correr el
servidor vía `go run` sin el stack).

## Métricas de cobertura

El daemon exporta la cobertura por ticker a Prometheus —
`faker_daemon_snapshots{ticker}` (conteo del último snapshot) y
`faker_daemon_coverage_findings_total{kind}` — presentadas en la pantalla
Monitoring y como un sparkline en la Data Library. Ver [daemon](daemon.md) para
el lado de las alertas.
