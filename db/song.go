package db

import (
	"encoding/json"
	"fmt"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const (
	SongBucket   = "SongBucket"
	SongBucketV2 = "SongBucketV2"
)

type DBer interface {
	// Open(path string, mode fs.FileMode, options *bolt.Options)
	Close() error
	OldListSongs() ([]*model.Song, error) // Still need for migrate

	// V2
	GetSong(rfid string) (*model.Song, error)
	ListSongs() ([]*model.Song, error)
	UpdateSong(song *model.Song) error
	DeleteSong(id string) error
	SongExists(id string) (bool, error)

	// RFID stuff
	GetRFIDSong(rfid string) (*model.RFIDSong, error)
	GetSongRFID(songID string) (*model.RFIDSong, error)
	AddRFIDSong(rfid, songID string) error
	RemoveRFIDSong(rfid, songID string) error
	DeleteRFID(id string) error
	ListRFIDSongs() ([]*model.RFIDSong, error)
	RFIDExists(rfid string) (bool, error)
	DeleteSongFromRFID(songID string) error
}

type SongDB struct {
	db *bolt.DB
}

func (s *SongDB) Close() error {
	return s.db.Close()
}

func (s *SongDB) OldListSongs() ([]*model.Song, error) {
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

func (s *SongDB) GetSong(songID string) (*model.Song, error) {
	var song *model.Song
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		v := b.Get([]byte(songID))
		if v == nil {
			// TODO: return err not found
			return nil
		}
		err := json.Unmarshal(v, &song)
		if err != nil {
			return err
		}
		return nil
	})
	if song == nil {
		// TODO: return err not found
	}
	return song, err
}

func (s *SongDB) ListSongs() ([]*model.Song, error) {
	songs := []*model.Song{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
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
		b := tx.Bucket([]byte(SongBucketV2))

		buf, err := json.Marshal(song)
		if err != nil {
			return err
		}
		return b.Put([]byte(song.ID), buf)
	})
}

func (s *SongDB) DeleteSong(songID string) error {
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		return b.Delete([]byte(songID))
	})
	if err != nil {
		return err
	}
	return s.DeleteSongFromRFID(songID)
}

//
//
//

func NewSongDB(path string) (DBer, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(SongBucket))
		if err != nil {
			return fmt.Errorf("create bucke(%s)t: %s", SongBucket, err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(SongBucketV2))
		if err != nil {
			return fmt.Errorf("create bucke(%s)t: %s", SongBucketV2, err)
		}

		_, err = tx.CreateBucketIfNotExists([]byte(RFIDBucket))
		if err != nil {
			return fmt.Errorf("create bucke(%s)t: %s", RFIDBucket, err)
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

func (s *SongDB) SongExists(id string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(SongBucketV2))
		v := b.Get([]byte(id))
		exists = v != nil
		return nil
	})
	return exists, err
}
