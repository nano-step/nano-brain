package webui

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const missingUIHTML = `<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"><title>nano-brain — Web UI not built</title></head>
<body style="font-family:system-ui;padding:2em;max-width:600px;line-height:1.6;color:#333">
<h1>nano-brain Web UI not built</h1>
<p>This binary was built without the bundled web UI.</p>
<p>Either:</p>
<ul>
  <li>Install the prebuilt npm package: <code>npx @nano-step/nano-brain@latest</code></li>
  <li>Or build from source: <code>make web-build &amp;&amp; go build ./cmd/nano-brain</code></li>
</ul>
<p>The REST API at <code>/api/v1/*</code> works regardless.</p>
</body>
</html>
`

func serveMissingUI(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-cache")
	return c.HTML(http.StatusOK, missingUIHTML)
}
