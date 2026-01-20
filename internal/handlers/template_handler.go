package handlers

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
)

type TemplateHandler struct {
	templates *template.Template
}

func NewTemplateHandler(templatesFS fs.FS) (*TemplateHandler, error) {
	tmpl, err := template.ParseFS(templatesFS, "*.html")
	if err != nil {
		return nil, err
	}

	return &TemplateHandler{
		templates: tmpl,
	}, nil
}

func (th *TemplateHandler) ServeTemplate(templateName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if err := th.templates.ExecuteTemplate(w, templateName+".html", nil); err != nil {
			log.Printf("Error executing template %s: %v", templateName, err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}
