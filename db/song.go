package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const SongBucketV2 = "SongBucketV2"

// SongStore is the read/write interface for song records.
type SongStore interface {
	GetSong(songID string) (*model.Song, error)
	ListSongs() ([]*model.Song, error)
	CreateSong(song *model.Song) error
	UpdateSong(song *model.Song) error
	DeleteSong(id string) error
	SongExists(id string) (bool, error)
}

// ErrNotFound is returned when a song or resource is not found in the database.
var ErrNotFound = errors.New("db: not found")

// DBer is the full database interface embedding both song and RFID stores.
type DBer interface {
	SongStore
	RFIDStore
	Close() error
}

// SongDB is a BoltDB-backed implementation of DBer.
type SongDB struct {
	db *bolt.DB
}

// NewSongDB opens the database at path and ensures required buckets exist.
func NewSongDB(path string) (DBer, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{SongBucketV2, RFIDBucket} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("create bucket %q: %w", name, err)
			}
		}
		return nil
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &SongDB{db: db}, nil
}

// Close closes the database connection.
func (s *SongDB) Close() error {
	return s.db.Close()
}

func (s *SongDB) GetSong(songID string) (*model.Song, error) {
	var song *model.Song
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		v := b.Get([]byte(songID))
		if v == nil {
			return ErrNotFound
		}
		song = &model.Song{}
		return json.Unmarshal(v, song)
	})
	if err != nil {
		return nil, err
	}
	return song, nil
}

func (s *SongDB) ListSongs() ([]*model.Song, error) {
	var songs []*model.Song
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		return b.ForEach(func(k, v []byte) error {
			var song model.Song
			if err := json.Unmarshal(v, &song); err != nil {
				return err
			}
			songs = append(songs, &song)
			return nil
		})
	})
	return songs, err
}

func (s *SongDB) CreateSong(song *model.Song) error {
	if song.ID == "" {
		return fmt.Errorf("song ID required")
	}
	now := time.Now()
	song.CreatedAt = now
	song.UpdatedAt = now

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
}

func (s *SongDB) UpdateSong(song *model.Song) error {
	if song.ID == "" {
		return fmt.Errorf("song ID required")
	}
	song.UpdatedAt = time.Now()

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
}

func (s *SongDB) DeleteSong(songID string) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(SongBucketV2)).Delete([]byte(songID))
	}); err != nil {
		return err
	}
	return s.DeleteSongFromRFID(songID)
}

func (s *SongDB) SongExists(id string) (bool, error) {
	var exists bool
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		exists = b.Get([]byte(id)) != nil
		return nil
	})
	return exists, err
}
