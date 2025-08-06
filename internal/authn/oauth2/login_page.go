package oauth2

import (
	"log"
	"net/http"

	"github.com/bornholm/calli/internal/ui"
	"github.com/pkg/errors"
)

func (h *Handler) getLoginPage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		ui.HeadTemplateData
		Providers []Provider
	}{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: "Authentification",
		},
		Providers: h.providers,
	}

	if err := templates.ExecuteTemplate(w, "login", data); err != nil {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
	}
}
