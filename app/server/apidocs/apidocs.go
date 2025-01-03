package apidocs

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
)

type Opts func(*config)

// configures the Doc middlewares
type config struct {
	// SpecURL the url to find the spec for
	SpecURL string
	// When this return value is false, 403 will be responsed.
	Authorizer func(*http.Request) bool

	IsHTTPS bool
}

func prepare(basePath string, cfg *config, apiJSON []byte) (string, string, []byte) {
	docPath := path.Join(basePath, "apidocs")

	// html
	tmpl := template.Must(template.New("apidoc").Parse(pageTemplate))
	buf := bytes.NewBuffer(nil)
	_ = tmpl.Execute(buf, cfg)
	uiHTML := buf.String()

	// json
	responseJSON := apiJSON
	if cfg.IsHTTPS {
		responseJSON = []byte(strings.Replace(
			string(apiJSON),
			`"schemes": [
    "http"
  ],`,
			`"schemes": [
    "https"
  ],`,
			1))
	}

	return docPath, uiHTML, responseJSON
}

func strIn(target string, source ...string) bool {
	for _, s := range source {
		if target == s {
			return true
		}
	}

	return false
}

// Doc creates a middleware to serve a documentation site for a swagger spec.
// This allows for altering the spec before starting the http listener.
func Doc(basePath string, apiJSON []byte, opts ...Opts) echo.MiddlewareFunc {
	cfg := &config{
		SpecURL: path.Join(basePath, "apispec.json"),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	docPath, uiHTML, responseJSON := prepare(basePath, cfg, apiJSON)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqPath := c.Request().URL.Path
			if strIn(reqPath, basePath, docPath, cfg.SpecURL) {
				if cfg.Authorizer != nil && !cfg.Authorizer(c.Request()) {
					return c.String(403, "Forbidden")
				}

				switch reqPath {
				case docPath:
					return c.HTML(http.StatusOK, uiHTML)
				case cfg.SpecURL:
					return c.JSONBlob(http.StatusOK, responseJSON)
				case basePath:
					return c.Redirect(http.StatusFound, docPath)
				}
			}

			if next == nil {
				return c.String(http.StatusNotFound, fmt.Sprintf("%q not found", reqPath))
			}

			return next(c)
		}
	}
}

const pageTemplate = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <title>API documentation</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1" />
  </head>

  <body>
    <script id="api-reference" data-url="{{ .SpecURL }}"></script>

    <script src="https://cdnjs.cloudflare.com/ajax/libs/scalar-api-reference/1.25.99/standalone.min.js" integrity="sha512-ai3lOYZ5efNXMYwnqhz0mnCaImbqfwLE1VCx9Y9nhB3OJX4/uegjIAoQtJHy3SILHp/gS1OlPCIeNFPZT5i2WQ==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>
  </body>
</html>`
