package web

import (
	"embed"
	"html/template"
)

//go:embed templates/*.html
var templatesFS embed.FS

// pageFiles maps a page name to its content template file. Each page is parsed
// together with the shared layout.
var pageFiles = map[string]string{
	"login":        "login.html",
	"dashboard":    "dashboard.html",
	"issues":       "issues.html",
	"projects":     "projects.html",
	"agents":       "agents.html",
	"agent_detail": "agent_detail.html",
	"tokens":       "tokens.html",
	"users":        "users.html",
}

// funcMap holds template helpers shared by every page.
var funcMap = template.FuncMap{
	"shortID": shortID,
	"deref":   deref,
}

// shortID returns the first 8 characters of s for compact display.
func shortID(s string) string {
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// deref returns the pointed-to int64, or 0 when p is nil.
func deref(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// parseTemplates parses every page against the shared layout and returns them
// keyed by page name. It panics on a malformed template, which is a build-time
// bug in the embedded files.
func parseTemplates() map[string]*template.Template {
	out := make(map[string]*template.Template, len(pageFiles))
	for name, file := range pageFiles {
		out[name] = template.Must(template.New(file).Funcs(funcMap).
			ParseFS(templatesFS, "templates/layout.html", "templates/"+file))
	}
	return out
}
