
package torrstor

import "github.com/anacrolix/torrent/storage"

var _ storage.TorrentImpl = (*Cache)(nil)
