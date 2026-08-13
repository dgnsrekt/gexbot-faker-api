package api

import _ "embed"

//go:embed asyncapi.yaml
var AsyncAPISpec []byte

// AsyncAPIUIBundle and AsyncAPIUICSS are the @asyncapi/react-component standalone
// renderer (pinned v3.1.5), vendored so the WebSocket docs page (/asyncapi) works fully
// offline instead of loading from the unpkg CDN — same approach as the embedded Swagger UI.
// Update: re-download browser/standalone/index.js + styles/default.min.css for the pinned
// version from unpkg into api/asyncapi-ui/.

//go:embed asyncapi-ui/asyncapi-standalone.js
var AsyncAPIUIBundle []byte

//go:embed asyncapi-ui/asyncapi.css
var AsyncAPIUICSS []byte
