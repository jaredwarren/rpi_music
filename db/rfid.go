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

func (s *SongDB) RFIDExists(rfid string) (bool, error) {
	exists := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		v := b.Get([]byte(rfid))
		exists = v != nil
		return nil
	})
	return exists, err
}

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

// GetRFIDSong
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

		// if not songs, delete key
		if len(rs.Songs) == 0 {
			return b.Delete([]byte(rfid))
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

		// re-insert remaining songs
		buf, err := json.Marshal(rs)
		if err != nil {
			return err
		}
		return b.Put([]byte(rfid), buf)
	})
}

func (s *SongDB) GetSongRFID(songID string) (*model.RFIDSong, error) {
	var respRfid *model.RFIDSong
	if songID == "" {
		return nil, fmt.Errorf("GetSongRFID need songID(%s)", songID)
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var rs *model.RFIDSong
			err := json.Unmarshal(v, &rs)
			if err != nil {
				return err
			}

			for _, s := range rs.Songs {
				if s == songID {
					respRfid = rs
					return nil
				}
			}
		}
		return nil
	})
	return respRfid, err
}

func (s *SongDB) DeleteSongFromRFID(songID string) error {
	if songID == "" {
		return fmt.Errorf("DeleteSongFromRFID need songID(%s)", songID)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var rs *model.RFIDSong
			err := json.Unmarshal(v, &rs)
			if err != nil {
				return err
			}

			updated := false
			for i, s := range rs.Songs {
				if s == songID {
					rs.Songs = append(rs.Songs[:i], rs.Songs[i+1:]...)
					updated = true
					break
				}
			}
			if updated {
				// if not songs, delete key
				if len(rs.Songs) == 0 {
					err := b.Delete([]byte(rs.RFID))
					if err != nil {
						return err
					}
				} else {
					// re-insert remaining songs
					buf, err := json.Marshal(rs)
					if err != nil {
						return err
					}
					err = b.Put([]byte(rs.RFID), buf)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (s *SongDB) DeleteRFID(id string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(RFIDBucket))
		return b.Delete([]byte(id)) // note: needs to "key"
	})
}
