package media

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/processor"
	"github.com/nicolasbonnici/gorest/rbac"
)

const uploadField = "file"

type mediaHandler struct {
	processor processor.Processor[Media, struct{}, MediaUpdateDTO, MediaResponseDTO]
	service   *MediaService
	converter *MediaConverter
	config    *Config
}

// writeGuards is the chain every mutating media route carries: resolve the
// identity, load its roles, demand one, then demand a write role.
func writeGuards(db database.Database, config *Config) []fiber.Handler {
	chain := []fiber.Handler{}
	if config.AuthMiddleware != nil {
		chain = append(chain, config.AuthMiddleware)
	}
	return append(chain,
		rbac.RoleLoader(db, config.RoleHierarchy),
		rbac.RequireAuthenticated(),
		rbac.RequireAnyRole(config.RoleHierarchy, config.SuperuserRole, config.WriteRoles...),
	)
}

// readChain mounts only the identity resolver, so a public read still records
// who is asking when a token happens to be present.
func readChain(config *Config) []fiber.Handler {
	if config.AuthMiddleware != nil {
		return []fiber.Handler{config.AuthMiddleware}
	}
	return nil
}

// mount registers handler at the end of chain. Fiber v3 takes the first element
// positionally and the rest variadically, running them in the order given.
func mount(register func(string, any, ...any) fiber.Router, path string, chain []fiber.Handler, h fiber.Handler) {
	all := make([]any, 0, len(chain)+1)
	for _, m := range chain {
		all = append(all, m)
	}
	all = append(all, h)
	register(path, all[0], all[1:]...)
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

	write := writeGuards(db, config)
	read := readChain(config)

	mount(router.Get, "/media", read, h.GetAll)
	mount(router.Get, "/media/:id", read, h.GetByID)
	mount(router.Get, "/media/:id/download", read, h.Download)
	mount(router.Post, "/media", write, h.Upload)
	mount(router.Put, "/media/:id", write, h.Update)
	mount(router.Delete, "/media/:id", write, h.Delete)
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
