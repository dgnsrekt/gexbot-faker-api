# Base de conocimiento de GEX Faker (Español)

Bundle OKF v0.1 para la API de GEX Faker — un servidor de simulación que reproduce
datos históricos de opciones/GEX de GexBot sobre REST y WebSocket, con una UI web
(Studio) y una CLI para agentes (`gexfakercli`). Empieza aquí, o apunta un agente a
este directorio para responder preguntas de configuración, uso e integración desde
la propia fuente.

## Úsalo

* [Visión general](overview.md) — qué es el faker, qué reproduce y sus piezas de un vistazo
* [Inicio rápido](quick-start.md) — del clon a un stream en vivo en el Studio (el camino ideal)
* [Studio](studio.md) — las siete pantallas de la UI web y qué hace cada una
* [Descargar datos](download-data.md) — las dos formas en que llegan los datos (archivo EOD vs `/hist`) y la clave API
* [Materializar y cargar](materialize-load.md) — el ciclo archived → ready → loaded (la confusión n.º 1)
* [Docker y observabilidad](docker-observability.md) — el stack de compose, Prometheus/Loki y la pantalla Monitoring

## Desarrolla con él

* [Apunta un cliente al faker](point-a-client.md) — URL base, auth por header y reproducción por clave
* [gexfakercli](gexfakercli.md) — la CLI para agentes con salida JSON (`setup`, `describe`, pulls, cursor)
* [API REST](rest-api.md) — la superficie de endpoints, Swagger UI y la spec OpenAPI
* [Streaming WebSocket](websockets.md) — los cinco hubs, `/negotiate`, nombres de grupo y el formato de frame
* [Configuración](configuration.md) — cada variable de entorno del servidor y del descargador
* [El daemon](daemon.md) — descargas programadas, alertas de cobertura y notificaciones push por ntfy

## Cuando algo falla

* [Solución de problemas](troubleshooting.md) — sin datos, archived vs ready, errores 400 de auth, cursores agotados

---

*¿Prefieres inglés? → [knowledge base (English)](../index.md).*
