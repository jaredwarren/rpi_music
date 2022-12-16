package db

import (
	"encoding/json"
	"fmt"

	"github.com/jaredwarren/rpi_music/model"
	bolt "go.etcd.io/bbolt"
)

const (
	RFIDBucket = "RFIDBucket"
)

func (s *SongDB) ListRFIDSongs() ([]*model.RFIDSong, error) {
	rss := []*model.RFIDSong{}
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var rs *model.RFIDSong
			err := json.Unmarshal(v, &rs)
			if err != nil {
				return err
			}
			rss = append(rss, rs)
		}
		return nil
	})
	return rss, err
}

func (s *SongDB) GetRFIDSong(rfid string) (*model.RFIDSong, error) {
	var rs *model.RFIDSong
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		if v == nil {
			return nil
		}
		err := json.Unmarshal(v, &rs)
		if err != nil {
			return err
		}
		return nil
	})
	if rs == nil {
		// TODO: return err not found
	}
	return rs, err
}

func (s *SongDB) AddRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("AddRFIDSong need rfid(%s) and songID(%s)", rfid, songID)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))

		// find existing
		v := b.Get([]byte(rfid))
		if v == nil {
			// insert new
			buf, err := json.Marshal(&model.RFIDSong{
				RFID:  rfid,
				Songs: []string{songID},
			})
			if err != nil {
				return err
			}
			return b.Put([]byte(rfid), buf)
		}

		// Update
		var rs *model.RFIDSong
		err := json.Unmarshal(v, &rs)
		if err != nil {
			return err
		}

		// don't add duplicates
		exists := false
		for _, v := range rs.Songs {
			if v == songID {
				exists = true
				break
			}
		}
		if exists {
			return nil
		}

		rs.Songs = append(rs.Songs, songID)

		// re-insert
		buf, err := json.Marshal(rs)
		if err != nil {
			return err
		}
		return b.Put([]byte(rfid), buf)
	})
}

func (s *SongDB) RemoveRFIDSong(rfid, songID string) error {
	if rfid == "" || songID == "" {
		return fmt.Errorf("RemoveRFIDSong need rfid(%s) and songID(%s)", rfid, songID)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))

		// find existing
		v := b.Get([]byte(rfid))
		if v == nil {
			return nil
		}

		// Update
		var rs *model.RFIDSong
		err := json.Unmarshal(v, &rs)
		if err != nil {
			return err
		}

		// ignore if not found
		exists := false
		for i, v := range rs.Songs {
			if v == songID {
				exists = true
				rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
				break
			}
		}
		if !exists {
			return nil
		}

		// re-insert
		buf, err := json.Marshal(rs)
		if err != nil {
			return err
		}
		return b.Put([]byte(rfid), buf)
	})
}

func (s *SongDB) DeleteRFID(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		return b.Delete([]byte(id)) // note: needs to "key"
	})
}
