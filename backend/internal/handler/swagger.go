package handler

import (
	"github.com/gofiber/fiber/v3"
	"github.com/swaggo/swag"
)

// SwaggerJSON serves the generated swagger spec as JSON.
//
// @Router /swagger/doc.json [get]
func SwaggerJSON(c fiber.Ctx) error {
	spec, err := swag.ReadDoc()
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "application/json")
	return c.SendString(spec)
}

// SwaggerUI serves the Swagger UI HTML (loads spec from /swagger/doc.json via CDN).
func SwaggerUI(c fiber.Ctx) error {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>SuperNetworkAI API</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/swagger/doc.json",
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    })
  </script>
</body>
</html>`
	c.Set(fiber.HeaderContentType, "text/html")
	return c.SendString(html)
}
