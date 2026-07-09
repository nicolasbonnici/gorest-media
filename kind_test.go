package media

import "testing"

func TestKindForMime(t *testing.T) {
	tests := []struct {
		mime string
		want string
	}{
		{"image/png", KindImage},
		{"image/jpeg", KindImage},
		{"video/mp4", KindVideo},
		{"audio/mpeg", KindAudio},
		{"application/pdf", KindDocument},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", KindDocument},
		{"text/csv", KindSpreadsheet},
		{"application/vnd.ms-excel", KindSpreadsheet},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", KindSpreadsheet},
		{"application/zip", KindArchive},
		{"application/gzip", KindArchive},
		{"text/plain", KindDocument},
		{"application/octet-stream", KindOther},
		{"", KindOther},
		{"image/png; charset=binary", KindImage},
		{"  IMAGE/PNG  ", KindImage},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			if got := KindForMime(tt.mime, nil); got != tt.want {
				t.Errorf("KindForMime(%q) = %q, want %q", tt.mime, got, tt.want)
			}
		})
	}
}

func TestKindForMimeOverrides(t *testing.T) {
	overrides := map[string]string{
		"application/octet-stream": "binary",
		"image/":                   "picture",
	}

	if got := KindForMime("application/octet-stream", overrides); got != "binary" {
		t.Errorf("exact override = %q, want binary", got)
	}
	if got := KindForMime("image/png", overrides); got != "picture" {
		t.Errorf("family override = %q, want picture", got)
	}
	if got := KindForMime("video/mp4", overrides); got != KindVideo {
		t.Errorf("unmatched override falls through, got %q", got)
	}
}
