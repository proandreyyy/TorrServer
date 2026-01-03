package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/log"

	bolt "go.etcd.io/bbolt"
)

type TDB struct {
	Path string
	db   *bolt.DB
}

var globalBboltDB TorrServerDB

func NewTDB() TorrServerDB {
	if globalBboltDB != nil {
		return globalBboltDB // Return existing instance
	}
	dbPath := filepath.Join(Path, "config.db")
	open := func() (*bolt.DB, error) {
		// Small timeout to avoid hanging on stale locks on tvOS sandbox FS.
		return bolt.Open(dbPath, 0o666, &bolt.Options{Timeout: 1500 * time.Millisecond})
	}

	db, err := open()
	if err != nil {
		log.TLogln("bbolt open failed:", err)
		// Try to recover from stale locks/corrupt file: move old DB aside and recreate.
		_ = os.Remove(dbPath + ".lock")
		recoveredPath := fmt.Sprintf("%s.bak.%d", dbPath, time.Now().Unix())
		if renameErr := os.Rename(dbPath, recoveredPath); renameErr == nil {
			log.TLogln("renamed broken DB to", recoveredPath)
		}
		db, err = open()
		if err != nil {
			log.TLogln("failed to recreate config.db:", err)
			return nil
		}
		log.TLogln("recreated empty config.db")
	}

	tdb := new(TDB)
	tdb.db = db
	tdb.Path = Path
	globalBboltDB = tdb
	return globalBboltDB
}

func (v *TDB) CloseDB() {
	if v.db != nil {
		v.db.Close()
		v.db = nil
	}
}

func (v *TDB) Get(xpath, name string) []byte {
	spath := strings.Split(xpath, "/")
	if len(spath) == 0 {
		return nil
	}
	var ret []byte
	err := v.db.View(func(tx *bolt.Tx) error {
		buckt := tx.Bucket([]byte(spath[0]))
		if buckt == nil {
			return nil
		}

		for i, p := range spath {
			if i == 0 {
				continue
			}
			buckt = buckt.Bucket([]byte(p))
			if buckt == nil {
				return nil
			}
		}

		data := buckt.Get([]byte(name))
		if data != nil {
			// CRITICAL: Copy the data before returning
			ret = make([]byte, len(data))
			copy(ret, data)
		}
		return nil
	})
	if err != nil {
		log.TLogln("Error get sets", xpath+"/"+name, ", error:", err)
	}

	return ret
}

func (v *TDB) Set(xpath, name string, value []byte) {
	spath := strings.Split(xpath, "/")
	if len(spath) == 0 {
		return
	}
	err := v.db.Update(func(tx *bolt.Tx) error {
		buckt, err := tx.CreateBucketIfNotExists([]byte(spath[0]))
		if err != nil {
			return err
		}

		for i, p := range spath {
			if i == 0 {
				continue
			}
			buckt, err = buckt.CreateBucketIfNotExists([]byte(p))
			if err != nil {
				return err
			}
		}

		return buckt.Put([]byte(name), value)
	})
	if err != nil {
		log.TLogln("Error put sets", xpath+"/"+name, ", error:", err)
		log.TLogln("value:", value)
	}
}

func (v *TDB) List(xpath string) []string {
	spath := strings.Split(xpath, "/")
	if len(spath) == 0 {
		return nil
	}
	var ret []string
	err := v.db.View(func(tx *bolt.Tx) error {
		buckt := tx.Bucket([]byte(spath[0]))
		if buckt == nil {
			return nil
		}

		for i, p := range spath {
			if i == 0 {
				continue
			}
			buckt = buckt.Bucket([]byte(p))
			if buckt == nil {
				return nil
			}
		}

		buckt.ForEach(func(k, _ []byte) error {
			if len(k) > 0 {
				ret = append(ret, string(k))
			}
			return nil
		})

		return nil
	})
	if err != nil {
		log.TLogln("Error list sets", xpath, ", error:", err)
	}

	return ret
}

func (v *TDB) Rem(xpath, name string) {
	spath := strings.Split(xpath, "/")
	if len(spath) == 0 {
		return
	}
	err := v.db.Update(func(tx *bolt.Tx) error {
		buckt := tx.Bucket([]byte(spath[0]))
		if buckt == nil {
			return nil
		}

		for i, p := range spath {
			if i == 0 {
				continue
			}
			buckt = buckt.Bucket([]byte(p))
			if buckt == nil {
				return nil
			}
		}

		return buckt.Delete([]byte(name))
	})
	if err != nil {
		log.TLogln("Error rem sets", xpath+"/"+name, ", error:", err)
	}
}

func (v *TDB) Clear(xPath string) {
	spath := strings.Split(xPath, "/")
	if len(spath) == 0 {
		return
	}

	err := v.db.Update(func(tx *bolt.Tx) error {
		buckt := tx.Bucket([]byte(spath[0]))
		if buckt == nil {
			return nil
		}

		for i, p := range spath {
			if i == 0 {
				continue
			}
			buckt = buckt.Bucket([]byte(p))
			if buckt == nil {
				return nil
			}
		}

		// Delete all entries in this bucket
		return buckt.ForEach(func(k, _ []byte) error {
			return buckt.Delete(k)
		})
	})

	if err != nil {
		log.TLogln("Error clear xPath", xPath, ", error:", err)
	}
}
