package media

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/processor"
)

const uploadField = "file"

type mediaHandler struct {
	processor processor.Processor[Media, struct{}, MediaUpdateDTO, MediaResponseDTO]
	service   *MediaService
	converter *MediaConverter
	config    *Config
}

func RegisterRoutes(router fiber.Router, db database.Database, config *Config, service *MediaService) {
	converter := NewMediaConverter(service.Storage())

	proc := processor.New(processor.ProcessorConfig[Media, struct{}, MediaUpdateDTO, MediaResponseDTO]{
		DB:                 db,
		CRUD:               crud.New[Media](db),
		Converter:          converter,
		PaginationLimit:    config.PaginationLimit,
		PaginationMaxLimit: config.MaxPaginationLimit,
		FieldMap: map[string]string{
			"id":         "id",
			"name":       "name",
			"mime_type":  "mime_type",
			"kind":       "kind",
			"extension":  "extension",
			"size":       "size",
			"user_id":    "user_id",
			"created_at": "created_at",
			"updated_at": "updated_at",
		},
		AllowedFields: []string{"id", "name", "mime_type", "kind", "extension", "size", "user_id", "created_at", "updated_at"},
	})

	h := &mediaHandler{processor: proc, service: service, converter: converter, config: config}

	router.Post("/media", h.Upload)
	router.Get("/media", h.GetAll)
	router.Get("/media/:id", h.GetByID)
	router.Get("/media/:id/download", h.Download)
	router.Put("/media/:id", h.Update)
	router.Delete("/media/:id", h.Delete)
}

func (h *mediaHandler) Upload(c fiber.Ctx) error {
	header, err := c.FormFile(uploadField)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "missing file upload field \""+uploadField+"\"")
	}

	m, err := h.service.Upload(c.Context(), header, c.FormValue("name"), currentUserID(c))
	if err != nil {
		switch {
		case errors.Is(err, ErrFileTooLarge):
			return fiber.NewError(fiber.StatusRequestEntityTooLarge, err.Error())
		case isNotAllowed(err):
			return fiber.NewError(fiber.StatusUnsupportedMediaType, err.Error())
		default:
			return fiber.NewError(fiber.StatusInternalServerError, "failed to store upload")
		}
	}

	return c.Status(fiber.StatusCreated).JSON(h.converter.ModelToResponseDTO(*m))
}

func (h *mediaHandler) GetAll(c fiber.Ctx) error {
	return h.processor.GetAll(c)
}

func (h *mediaHandler) GetByID(c fiber.Ctx) error {
	return h.processor.GetByID(c)
}

func (h *mediaHandler) Download(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid media id")
	}

	m, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "media not found")
	}

	reader, err := h.service.Open(c.Context(), m)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "file not found in storage")
	}
	// SendStream consumes reader after this handler returns and closes it when
	// it implements io.Closer, so it must not be closed here.

	c.Set(fiber.HeaderContentType, m.MimeType)
	c.Set(fiber.HeaderContentDisposition, "inline; filename=\""+m.Name+"\"")
	return c.SendStream(reader)
}

func (h *mediaHandler) Update(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid media id")
	}

	var dto MediaUpdateDTO
	if err := c.Bind().Body(&dto); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	m, err := h.service.Rename(c.Context(), id, dto.Name)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	return c.JSON(h.converter.ModelToResponseDTO(*m))
}

func (h *mediaHandler) Delete(c fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid media id")
	}

	m, err := h.service.GetByID(c.Context(), id)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "media not found")
	}

	if err := h.service.Delete(c.Context(), m); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete media")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func isNotAllowed(err error) bool {
	return err != nil && strings.Contains(err.Error(), "is not allowed")
}
