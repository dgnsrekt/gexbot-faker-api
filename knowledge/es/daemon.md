---
type: Guide
title: El daemon de descarga
description: El scheduler de larga duración que descarga cada día de mercado automáticamente, su estrategia de EOD-con-fallback-a-hist, las alertas de regresión de cobertura que emite y las notificaciones push por ntfy.
tags: [daemon, scheduler, eod, coverage, alerts, ntfy, español]
timestamp: 2026-08-11T00:00:00Z
---

# El daemon de descarga

El daemon (`cmd/daemon`) es un servicio de larga duración que **descarga cada día
de mercado automáticamente**, de modo que un faker desplegado siempre tenga datos
frescos sin corridas manuales. Viene en el stack de docker por defecto como
`gex-daemon`.

## Programación

Cada día de mercado intenta la descarga del **reporte EOD** empezando a las
`DAEMON_SCHEDULE_HOUR:MINUTE` (por defecto 17:05 ET), reintentando cada cinco
minutos. **Después de las 8:00 PM ET** hace fallback a descargas individuales de
`/hist`. Es consciente de los días de mercado (omite fines de semana y feriados) y,
con `DAEMON_RUN_ON_STARTUP`, verifica/descarga al arrancar. Consulta
[configuration](configuration.md) para las variables de la programación.

> El fallback de `/hist` fuera de horario escribe JSONL pero no el marker
> `.eod-materialized`, así que esas fechas siguen mostrando `archived` /
> Materialize (issue #38). Consulta [materialize & load](materialize-load.md).

## Alertas de cobertura

Después de cada descarga exitosa se revisan los datos en busca de **regresiones de
cobertura** (`internal/coverage`); los hallazgos se registran, y se dispara una
alerta ntfy de prioridad `high` cuando las notificaciones están habilitadas:

- **Snapshot drop** — el conteo de snapshots intradía de un ticker cae >10% por
  debajo de su mediana de 20 días (lo que parece un cambio silencioso de la fuente
  — p. ej. un feed adelgazando el muestreo de SPX/NDX).
- **Session shape** — el día abre más tarde de ~09:30 ET, cierra antes de
  ~16:00 ET, o tiene un hueco intradía de más de 120s (un feed truncado / una
  interrupción).

La métrica de snapshots también se exporta a Prometheus
(`faker_daemon_snapshots{ticker}`, `faker_daemon_coverage_findings_total{kind}`) y
aparece en el Data Library de Studio (sparkline + badge de desviación) y en la
pantalla Monitoring — consulta [docker & observability](docker-observability.md).

## Notificaciones push (ntfy)

Tanto el daemon como el downloader de CLI pueden hacer push a
[ntfy.sh](https://ntfy.sh) al completarse, al fallar, o ante un hallazgo de
cobertura. Habilítalo con `NTFY_ENABLED=true` y un `NTFY_TOPIC`; suscríbete en
`https://ntfy.sh/<topic>` o en la app de ntfy. La lista completa de variables está
en [configuration](configuration.md).
