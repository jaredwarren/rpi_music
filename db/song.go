package db

import (
	"encoding/json"
	"fmt"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const (
	SongBucket = "SongBucket"
)

type DBer interface {
	// Open(path string, mode fs.FileMode, options *bolt.Options)
	Close() error
	GetSong(id string) (*model.Song, error)
	ListSongs() ([]*model.Song, error)
	UpdateSong(song *model.Song) error
	DeleteSong(id string) error
	SongExists(id string) (bool, error)
}

func NewSongDB(path string) (DBer, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(SongBucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &SongDB{
		db: db,
	}, nil
}

type SongDB struct {
	db *bolt.DB
}

func (s *SongDB) Close() error {
	return s.db.Close()
}

func (s *SongDB) GetSong(id string) (*model.Song, error) {
	var song *model.Song
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		v := b.Get([]byte(id))
		if v == nil {
			return nil
		}
		err := json.Unmarshal(v, &song)
		if err != nil {
			return err
		}
		return nil
	})
	return song, err
}

func (s *SongDB) ListSongs() ([]*model.Song, error) {
	songs := []*model.Song{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var song *model.Song
			err := json.Unmarshal(v, &song)
			if err != nil {
				return err
			}
			songs = append(songs, song)
		}
		return nil
	})
	return songs, err
}

func (s *SongDB) UpdateSong(song *model.Song) error {
	if song.ID == "" {
		return fmt.Errorf("song ID required")
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))

		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
}

func (s *SongDB) DeleteSong(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		return b.Delete([]byte(id)) // note: needs to "key"
	})
}

func (s *SongDB) SongExists(id string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucket))
		v := b.Get([]byte(id))
		exists = v != nil
		return nil
	})
	return exists, err
}
