package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/nicolasbonnici/gorest/crud"
	"github.com/nicolasbonnici/gorest/database"
	"github.com/nicolasbonnici/gorest/query"
)

var ErrFileTooLarge = errors.New("file exceeds maximum allowed size")

type MediaService struct {
	db      database.Database
	config  *Config
	storage Storage
	crud    *crud.CRUD[Media]
}

func NewMediaService(db database.Database, config *Config, storage Storage) *MediaService {
	return &MediaService{
		db:      db,
		config:  config,
		storage: storage,
		crud:    crud.New[Media](db),
	}
}

func (s *MediaService) Storage() Storage { return s.storage }

// Upload reads the multipart file fully into memory (bounded by MaxFileSize),
// sniffs its real MIME type from the bytes rather than trusting the client
// header, persists the bytes to the storage backend, then records the row.
// Buffering is deliberate: the checksum and content sniff both need to see the
// bytes before the write, and the size ceiling keeps the buffer bounded.
func (s *MediaService) Upload(ctx context.Context, header *multipart.FileHeader, name string, userID uuid.UUID) (*Media, error) {
	if header.Size > s.config.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	src, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()

	// LimitReader guards against a header that understates the real size.
	data, err := io.ReadAll(io.LimitReader(src, s.config.MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > s.config.MaxFileSize {
		return nil, ErrFileTooLarge
	}

	detected := mimetype.Detect(data)
	mimeType := detected.String()
	if !s.config.IsAllowedMime(mimeType) {
		return nil, fmt.Errorf("mime type %s is not allowed", mimeType)
	}

	ext := extensionFor(header.Filename, detected.Extension())
	sum := sha256.Sum256(data)
	checksum := hex.EncodeToString(sum[:])

	id := uuid.New()
	key := storageKey(id, ext)

	if err := s.storage.Save(ctx, key, bytes.NewReader(data), int64(len(data)), mimeType); err != nil {
		return nil, fmt.Errorf("store file: %w", err)
	}

	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = header.Filename
	}
	if displayName == "" {
		displayName = id.String() + ext
	}

	m := Media{
		ID:            id,
		Name:          displayName,
		StorageKey:    key,
		StorageDriver: s.storage.Driver(),
		MimeType:      mimeType,
		Kind:          KindForMime(mimeType, s.config.KindOverrides),
		Extension:     strings.TrimPrefix(ext, "."),
		Size:          int64(len(data)),
		Checksum:      checksum,
		UserID:        userID,
		CreatedAt:     time.Now().UTC(),
	}

	if err := s.crud.Create(ctx, m); err != nil {
		// Roll back the stored bytes so a failed insert doesn't orphan a file.
		_ = s.storage.Delete(ctx, key)
		return nil, err
	}

	stored, err := s.crud.GetByID(ctx, id)
	if err != nil {
		return &m, nil
	}
	return stored, nil
}

func (s *MediaService) GetByID(ctx context.Context, id uuid.UUID) (*Media, error) {
	return s.crud.GetByID(ctx, id)
}

func (s *MediaService) Open(ctx context.Context, m *Media) (io.ReadCloser, error) {
	return s.storage.Open(ctx, m.StorageKey)
}

// Rename updates only the display name (and updated_at) with a targeted UPDATE.
// A full-row CRUD update would rewrite the immutable storage columns from a
// partial DTO and could blank them, so the mutable field is written directly.
func (s *MediaService) Rename(ctx context.Context, id uuid.UUID, name string) (*Media, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("name is required")
	}

	sql, args, err := query.New(s.db.Dialect()).
		Update("media").
		Set("name", name).
		Set("updated_at", time.Now().UTC()).
		Where(query.Eq("id", id)).
		Build()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx, sql, args...); err != nil {
		return nil, err
	}
	return s.crud.GetByID(ctx, id)
}

// Delete removes the database row first; the stored bytes are best-effort
// cleaned up afterward so a storage hiccup can't leave a dangling row that the
// API still advertises.
func (s *MediaService) Delete(ctx context.Context, m *Media) error {
	if err := s.crud.Delete(ctx, m.ID); err != nil {
		return err
	}
	_ = s.storage.Delete(ctx, m.StorageKey)
	return nil
}

func storageKey(id uuid.UUID, ext string) string {
	now := time.Now().UTC()
	return fmt.Sprintf("%04d/%02d/%s%s", now.Year(), now.Month(), id.String(), ext)
}

func extensionFor(filename, sniffed string) string {
	if ext := filepath.Ext(filename); ext != "" {
		return strings.ToLower(ext)
	}
	return strings.ToLower(sniffed)
}
