package api

import _ "embed"

//go:embed swagger-ui/swagger-ui-bundle.js
var SwaggerUIBundle []byte

//go:embed swagger-ui/swagger-ui.css
var SwaggerUICSS []byte
