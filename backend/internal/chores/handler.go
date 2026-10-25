package chores

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"kinlight/internal/auth"
	"kinlight/internal/respond"
	"kinlight/internal/store"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(s *Service, l *slog.Logger) *Handler { return &Handler{s, l} }
func (h *Handler) Serve(operation string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor, ok := auth.UserFrom(r.Context())
		if !ok {
			respond.Error(w, 401, "Please sign in.", "")
			return
		}
		subject, _ := auth.SubjectFrom(r.Context())
		var result any
		var err error
		status := 200
		if operation == "list" {
			result, err = h.service.List(r.Context(), actor, subject)
		} else if operation == "summary" {
			result, err = h.service.Summary(r.Context(), actor)
		} else {
			var id int64
			if operation != "create" {
				id, err = strconv.ParseInt(r.PathValue("choreId"), 10, 64)
				if err != nil || id < 1 {
					h.failure(w, r, store.ErrNotFound)
					return
				}
			}
			var body struct {
				Title     string `json:"title"`
				Date      string `json:"date"`
				Completed *bool  `json:"completed"`
			}
			if operation != "remove" {
				dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
				dec.DisallowUnknownFields()
				if err := dec.Decode(&body); err != nil {
					respond.Error(w, 400, "Send a valid chore request.", "")
					return
				}
				if err := dec.Decode(new(any)); err != io.EOF {
					respond.Error(w, 400, "Send one chore request.", "")
					return
				}
			}
			if operation == "completion" && body.Completed == nil {
				respond.Error(w, 400, "Choose whether the chore is completed.", "")
				return
			}
			completed := body.Completed != nil && *body.Completed
			result, err = h.service.Mutate(r.Context(), actor, subject, id, operation, body.Title, body.Date, completed)
			if operation == "create" {
				status = 201
			}
		}
		if err != nil {
			h.failure(w, r, err)
			return
		}
		respond.JSON(w, status, result)
	}
}
func (h *Handler) failure(w http.ResponseWriter, r *http.Request, err error) {
	status := 500
	message := "Couldn't save or load chores. Please try again."
	switch {
	case errors.Is(err, store.ErrNotFound):
		status = 404
		message = "That chore doesn't exist."
	case errors.Is(err, ErrForbidden):
		status = 403
		message = err.Error()
	case errors.Is(err, ErrStale), errors.Is(err, store.ErrChoreLimit):
		status = 409
		message = err.Error()
	case errors.Is(err, ErrDate), errors.Is(err, store.ErrChoreTitle):
		status = 400
		message = err.Error()
	default:
		h.logger.ErrorContext(r.Context(), "chore request failed", "err", err)
	}
	respond.Error(w, status, message, "")
}
