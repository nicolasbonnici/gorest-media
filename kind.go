package media

import "strings"

const (
	KindImage       = "image"
	KindVideo       = "video"
	KindAudio       = "audio"
	KindDocument    = "document"
	KindSpreadsheet = "spreadsheet"
	KindArchive     = "archive"
	KindOther       = "other"
)

// exactKinds classifies MIME types whose family prefix would be misleading
// (e.g. text/csv is a spreadsheet, not a plain document; the OpenXML types all
// share application/ but split into document vs. spreadsheet).
var exactKinds = map[string]string{
	"application/pdf":    KindDocument,
	"application/msword": KindDocument,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": KindDocument,
	"application/rtf": KindDocument,

	"text/csv":                 KindSpreadsheet,
	"application/csv":          KindSpreadsheet,
	"application/vnd.ms-excel": KindSpreadsheet,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": KindSpreadsheet,

	"application/zip":              KindArchive,
	"application/gzip":             KindArchive,
	"application/x-tar":            KindArchive,
	"application/x-7z-compressed":  KindArchive,
	"application/x-rar-compressed": KindArchive,
}

// familyKinds classifies by the leading "type/" segment for the media types
// where the whole family maps to one kind.
var familyKinds = map[string]string{
	"image/": KindImage,
	"video/": KindVideo,
	"audio/": KindAudio,
}

// KindForMime classifies a MIME type into a coarse media kind. Overrides win
// over the built-ins, so an operator can reclassify or add a format purely via
// config. An override key ending in "/" matches a whole family.
func KindForMime(mime string, overrides map[string]string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if mime == "" {
		return KindOther
	}
	if i := strings.IndexByte(mime, ';'); i >= 0 {
		mime = strings.TrimSpace(mime[:i])
	}

	if k, ok := overrides[mime]; ok {
		return k
	}
	for prefix, k := range overrides {
		if strings.HasSuffix(prefix, "/") && strings.HasPrefix(mime, prefix) {
			return k
		}
	}

	if k, ok := exactKinds[mime]; ok {
		return k
	}
	if i := strings.IndexByte(mime, '/'); i >= 0 {
		if k, ok := familyKinds[mime[:i+1]]; ok {
			return k
		}
	}
	if strings.HasPrefix(mime, "text/") {
		return KindDocument
	}
	return KindOther
}
