package fetch

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"yoopass-api/internal/dto"
	"yoopass-api/internal/http-server/handlers/response"
	resp "yoopass-api/internal/http-server/handlers/response"
	cipher "yoopass-api/internal/tools/cipher"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/render"
)

type Response struct {
	response.Response
	Message string `json:"message,omitempty"`
}

type SecretFetcher interface {
	// this matches call in storage
	Fetch(key string) ([]byte, error)
	Delete(key string) error
}

func New(log *slog.Logger, secretFetcher SecretFetcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.url.fetch.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		if secretFetcher == nil {
			log.Error("critical: secretFetcher is nil")
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal server error"))
			return
		}

		alias := chi.URLParam(r, "alias")
		if alias == "" {
			log.Info("Alias parameter is missing")
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("Alias parameter is missing"))
			return
		}

		key := chi.URLParam(r, "key")
		if key == "" {
			log.Info("Key parameter is missing")
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, resp.Error("Key parameter is missing"))
			return
		}

		cipherObject, err := secretFetcher.Fetch(alias)
		if err != nil {
			log.Error("Some error occured", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error(err.Error()))
			return
		}

		if cipherObject == nil {
			log.Info("Secret not found in storage", slog.String("alias", alias))
			render.Status(r, http.StatusNotFound)
			render.JSON(w, r, resp.Error("Secret not found"))
			return
		}

		object, err := cipher.Decode(cipherObject, key)
		if err != nil {
			log.Error("Failed to decode secret", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Failed to decode secret"))
			return
		}

		var dest dto.Secret

		err = json.Unmarshal(object, &dest)
		if err != nil {
			log.Error("Secret unmarshalling failed", slog.Any("error", err))
			render.Status(r, http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("Secret unmarshalling failed"))
			return
		}

		if dest.OneTime {
			err = secretFetcher.Delete(alias)
			if err != nil {
				log.Error("Failed to delete secret", slog.Any("error", err))
				render.Status(r, http.StatusInternalServerError)
				render.JSON(w, r, resp.Error("Failed to delete secret"))
				return
			}
		}

		render.JSON(w, r, Response{
			Response: resp.OK(),
			Message:  dest.Message,
		})
	}
}
