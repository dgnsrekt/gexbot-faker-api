# Observability

The optional stack includes Prometheus, Grafana, Loki, Grafana Alloy, Uptime Kuma, and a Caddy gateway. The API and daemon expose diagnostics only on the private Compose network; all host-facing monitoring ports pass through Caddy.

## Start

Set these values in `.env`:

```dotenv
OBSERVABILITY_PASSWORD=choose-a-password
NTFY_TOPIC=your-private-topic
# Optional for protected ntfy topics:
NTFY_TOKEN=
```

Then run:

```sh
just observability-up
```

Authentication is enabled by default with `OBSERVABILITY_USERNAME` (`admin` by default) and `OBSERVABILITY_PASSWORD`. For a quick trusted-LAN session without gateway authentication:

```sh
just observability-up-no-auth
```

Basic Auth is served over HTTP. It prevents casual access but does not encrypt credentials; do not expose these ports beyond a trusted LAN.

| Service | Default URL |
| --- | --- |
| Grafana | `http://localhost:3006` |
| Prometheus | `http://localhost:9095` |
| Loki | `http://localhost:3101` |
| Uptime Kuma | `http://localhost:3012` |
| Faker API diagnostics | `http://localhost:9096` |
| Faker daemon diagnostics | `http://localhost:9096/daemon/` |

Grafana dashboards, data sources, alerts, and ntfy delivery are provisioned automatically. Uptime Kuma deliberately keeps its supported first-run setup: create monitors for `http://gex-faker-api:9090/livez`, `http://gex-faker-api:9090/readyz`, and a representative request to `http://gex-faker-api:8080`.

When running Compose from a Git worktree, set `DATA_HOST_DIR` to the checkout that owns the dataset; for example, `DATA_HOST_DIR=../gexbot-faker-api/data`. The default remains `./data`.

Stop everything with `just observability-down`. Normal `docker compose up` does not start the monitoring profile.
