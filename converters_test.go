package media

import (
	"testing"

	"github.com/google/uuid"
)

func TestConverterURLLocalFallsBackToDownloadRoute(t *testing.T) {
	local, _ := NewLocalStorage(t.TempDir())
	c := NewMediaConverter(local)

	m := Media{ID: uuid.New(), StorageKey: "2026/07/x.png"}
	dto := c.ModelToResponseDTO(m)
	if want := "/media/" + m.ID.String() + "/download"; dto.URL != want {
		t.Errorf("URL = %q, want %q", dto.URL, want)
	}
}

func TestConverterURLCDNUsesPublicURL(t *testing.T) {
	cdn := NewCDNStorage("https://up.example.com", "https://cdn.example.com", "")
	c := NewMediaConverter(cdn)

	m := Media{ID: uuid.New(), StorageKey: "2026/07/x.png"}
	dto := c.ModelToResponseDTO(m)
	if want := "https://cdn.example.com/2026/07/x.png"; dto.URL != want {
		t.Errorf("URL = %q, want %q", dto.URL, want)
	}
}

func TestConverterModelsToResponseDTOs(t *testing.T) {
	local, _ := NewLocalStorage(t.TempDir())
	c := NewMediaConverter(local)

	models := []Media{
		{ID: uuid.New(), Name: "a", Kind: KindImage},
		{ID: uuid.New(), Name: "b", Kind: KindVideo},
	}
	dtos := c.ModelsToResponseDTOs(models)
	if len(dtos) != 2 || dtos[0].Name != "a" || dtos[1].Kind != KindVideo {
		t.Errorf("unexpected conversion: %+v", dtos)
	}
}

func TestConverterInterfaceStubs(t *testing.T) {
	c := NewMediaConverter(nil)
	if got := c.CreateDTOToModel(struct{}{}); got != (Media{}) {
		t.Errorf("CreateDTOToModel should be a zero-value stub, got %+v", got)
	}
	if got := c.UpdateDTOToModel(MediaUpdateDTO{Name: "x"}); got.Name != "x" {
		t.Errorf("UpdateDTOToModel Name = %q", got.Name)
	}
}
