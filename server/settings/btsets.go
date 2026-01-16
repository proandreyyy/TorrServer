package settings

import (
	"encoding/json"
	"io"
	"io/fs"

	"path/filepath"
	"strings"

	"server/log"
)

type TorznabConfig struct {
	Host string
	Key  string
	Name string
}

const (
	cacheDefaultsVersion = 1
	cacheSizeDefault     = 332 * 1024 * 1024
	preloadCacheDefault  = 10
	cacheDefaultPath     = "/Library/Caches/TorrServerCache"
)

type BTSets struct {
	// Cache
	CacheSize       int64 // in byte, def 332 MB
	ReaderReadAHead int   // in percent, 5%-100%, [...S__X__E...] [S-E] not clean
	PreloadCache    int   // in percent
	CacheDefaultsVersion int // cache defaults migration marker

	// Disk
	UseDisk           bool
	TorrentsSavePath  string
	RemoveCacheOnDrop bool

	// Torrent
	ForceEncrypt             bool
	RetrackersMode           int  // 0 - don`t add, 1 - add retrackers (def), 2 - remove retrackers 3 - replace retrackers
	TorrentDisconnectTimeout int  // in seconds
	EnableDebug              bool // debug logs

	// DLNA
	EnableDLNA   bool
	FriendlyName string

	// Rutor
	EnableRutorSearch bool

	// Torznab
	EnableTorznabSearch bool
	TorznabUrls         []TorznabConfig

	// BT Config
	EnableIPv6        bool
	DisableTCP        bool
	DisableUTP        bool
	DisableUPNP       bool
	DisableDHT        bool
	DisablePEX        bool
	DisableUpload     bool
	DownloadRateLimit int // in kb, 0 - inf
	UploadRateLimit   int // in kb, 0 - inf
	ConnectionsLimit  int
	PeersListenPort   int

	// HTTPS
	SslPort int
	SslCert string
	SslKey  string

	// Reader
	ResponsiveMode bool // enable Responsive reader (don't wait pieceComplete)

	// FS
	ShowFSActiveTorr bool

	// Storage preferences
	StoreSettingsInJson bool
	StoreViewedInJson   bool
}

func (v *BTSets) String() string {
	buf, _ := json.Marshal(v)
	return string(buf)
}

var BTsets *BTSets

func SetBTSets(sets *BTSets) {
	if ReadOnly {
		return
	}
	// failsafe checks (use defaults)
	if sets.CacheDefaultsVersion == 0 {
		if BTsets != nil && BTsets.CacheDefaultsVersion > 0 {
			sets.CacheDefaultsVersion = BTsets.CacheDefaultsVersion
		} else {
			sets.CacheDefaultsVersion = cacheDefaultsVersion
		}
	}
	if sets.CacheSize == 0 {
		sets.CacheSize = cacheSizeDefault
	}
	if sets.ConnectionsLimit == 0 {
		sets.ConnectionsLimit = 25
	}
	if sets.TorrentDisconnectTimeout == 0 {
		sets.TorrentDisconnectTimeout = 30
	}

	if sets.ReaderReadAHead < 5 {
		sets.ReaderReadAHead = 5
	}
	if sets.ReaderReadAHead > 100 {
		sets.ReaderReadAHead = 100
	}

	if sets.PreloadCache < 0 {
		sets.PreloadCache = 0
	}
	if sets.PreloadCache > 100 {
		sets.PreloadCache = 100
	}

	if sets.TorrentsSavePath == "" {
		sets.UseDisk = false
	} else if sets.UseDisk {
		BTsets = sets

		go filepath.WalkDir(sets.TorrentsSavePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() && strings.ToLower(d.Name()) == ".tsc" {
				BTsets.TorrentsSavePath = path
				log.TLogln("Find directory \"" + BTsets.TorrentsSavePath + "\", use as cache dir")
				return io.EOF
			}
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		})
	}

	BTsets = sets
	buf, err := json.Marshal(BTsets)
	if err != nil {
		log.TLogln("Error marshal btsets", err)
		return
	}
	tdb.Set("Settings", "BitTorr", buf)
}

func SetDefaultConfig() {
	sets := new(BTSets)
	sets.CacheSize = cacheSizeDefault // 332 MB
	sets.PreloadCache = preloadCacheDefault
	sets.UseDisk = true
	sets.TorrentsSavePath = cacheDefaultPath
	sets.RemoveCacheOnDrop = true
	sets.CacheDefaultsVersion = cacheDefaultsVersion
	sets.ConnectionsLimit = 25
	sets.RetrackersMode = 1
	sets.TorrentDisconnectTimeout = 30
	sets.ReaderReadAHead = 95 // 95%
	sets.ResponsiveMode = true
	sets.ShowFSActiveTorr = true
	sets.StoreSettingsInJson = true
	BTsets = sets
	if !ReadOnly {
		buf, err := json.Marshal(BTsets)
		if err != nil {
			log.TLogln("Error marshal btsets", err)
			return
		}
		tdb.Set("Settings", "BitTorr", buf)
	}
}

func loadBTSets() {
	buf := tdb.Get("Settings", "BitTorr")
	if len(buf) > 0 {
		err := json.Unmarshal(buf, &BTsets)
		if err == nil {
			if BTsets.ReaderReadAHead < 5 {
				BTsets.ReaderReadAHead = 5
			}
			if applyCacheDefaultsMigration(BTsets) {
				SetBTSets(BTsets)
			}
			return
		}
		log.TLogln("Error unmarshal btsets", err)
	}
	// initialize defaults on error
	SetDefaultConfig()
}

func applyCacheDefaultsMigration(sets *BTSets) bool {
	if sets.CacheDefaultsVersion >= cacheDefaultsVersion {
		return false
	}
	sets.CacheSize = cacheSizeDefault
	sets.PreloadCache = preloadCacheDefault
	sets.UseDisk = true
	sets.TorrentsSavePath = cacheDefaultPath
	sets.RemoveCacheOnDrop = true
	sets.CacheDefaultsVersion = cacheDefaultsVersion
	log.TLogln("Applied cache defaults migration")
	return true
}
