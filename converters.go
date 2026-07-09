package media

type MediaConverter struct {
	storage Storage
}

func NewMediaConverter(storage Storage) *MediaConverter {
	return &MediaConverter{storage: storage}
}

// CreateDTOToModel and UpdateDTOToModel exist only to satisfy the processor's
// ModelConverter interface. Writes never flow through the processor: uploads
// are multipart (handled by MediaService.Upload) and renames go through
// MediaService.Rename, so the processor is wired for list/get reads only.
func (c *MediaConverter) CreateDTOToModel(struct{}) Media { return Media{} }

func (c *MediaConverter) UpdateDTOToModel(dto MediaUpdateDTO) Media {
	return Media{Name: dto.Name}
}

func (c *MediaConverter) ModelToResponseDTO(model Media) MediaResponseDTO {
	return MediaResponseDTO{
		ID:        model.ID,
		Name:      model.Name,
		MimeType:  model.MimeType,
		Kind:      model.Kind,
		Extension: model.Extension,
		Size:      model.Size,
		Checksum:  model.Checksum,
		URL:       c.publicURL(model),
		UserID:    model.UserID,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func (c *MediaConverter) ModelsToResponseDTOs(models []Media) []MediaResponseDTO {
	dtos := make([]MediaResponseDTO, len(models))
	for i, model := range models {
		dtos[i] = c.ModelToResponseDTO(model)
	}
	return dtos
}

// publicURL prefers the backend's own address (a CDN edge) and falls back to
// this API's download route when the backend streams through us (local disk).
func (c *MediaConverter) publicURL(model Media) string {
	if c.storage != nil {
		if u := c.storage.URL(model.StorageKey); u != "" {
			return u
		}
	}
	return "/media/" + model.ID.String() + "/download"
}
