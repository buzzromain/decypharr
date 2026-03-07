package qbit

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"github.com/sirrobot01/decypharr/internal/config"
	"github.com/sirrobot01/decypharr/internal/utils"
	"github.com/sirrobot01/decypharr/pkg/arr"
	"github.com/sirrobot01/decypharr/pkg/manager"
	"github.com/sirrobot01/decypharr/pkg/storage"
)

// All torrent-related helpers goes here
func (q *QBit) addMagnet(ctx context.Context, url string, arr *arr.Arr, debrid string, action config.DownloadAction, callbackURL string, rmTrackerUrls, skipMultiSeason bool) error {
	magnet, err := utils.GetMagnetFromUrl(url, rmTrackerUrls)
	if err != nil {
		return fmt.Errorf("error parsing magnet link: %w", err)
	}

	importReq := manager.NewTorrentRequest(debrid, q.downloadFolder, magnet, arr, action, arr.DownloadUncached, callbackURL, manager.ImportTypeQBit, skipMultiSeason)

	err = q.manager.AddNewTorrent(ctx, importReq)
	if err != nil {
		return fmt.Errorf("failed to process torrent: %w", err)
	}
	return nil
}

func (q *QBit) addTorrent(ctx context.Context, fileHeader *multipart.FileHeader, arr *arr.Arr, debrid string, action config.DownloadAction, callbackURL string, rmTrackerUrls, skipMultiSeason bool) error {
	file, _ := fileHeader.Open()
	defer file.Close()
	var reader io.Reader = file
	magnet, err := utils.GetMagnetFromFile(reader, fileHeader.Filename, rmTrackerUrls)
	if err != nil {
		return fmt.Errorf("error reading file: %s \n %w", fileHeader.Filename, err)
	}
	importReq := manager.NewTorrentRequest(debrid, q.downloadFolder, magnet, arr, action, arr.DownloadUncached, callbackURL, manager.ImportTypeQBit, skipMultiSeason)
	err = q.manager.AddNewTorrent(ctx, importReq)
	if err != nil {
		return fmt.Errorf("failed to process torrent: %w", err)
	}
	return nil
}

func (q *QBit) ResumeTorrent(t *storage.Entry) bool {
	return true
}

func (q *QBit) PauseTorrent(t *storage.Entry) bool {
	return true
}

func (q *QBit) RefreshTorrent(t *storage.Entry) bool {
	return true
}

func (q *QBit) GetTorrentProperties(t *storage.Entry) *TorrentProperties {
	return &TorrentProperties{
		AdditionDate:       t.AddedOn.Unix(),
		Comment:            "Provider Blackhole <https://github.com/sirrobot01/decypharr>",
		CreatedBy:          "Provider Blackhole <https://github.com/sirrobot01/decypharr>",
		CreationDate:       t.AddedOn.Unix(),
		DlLimit:            -1,
		UpLimit:            -1,
		DlSpeed:            t.Speed,
		UpSpeed:            t.Speed,
		TotalSize:          t.Size,
		TotalUploaded:      t.Bytes,
		TotalDownloaded:    t.Bytes,
		LastSeen:           time.Now().Unix(),
		NbConnectionsLimit: 100,
		Peers:              0,
		PeersTotal:         2,
		SeedingTime:        1,
		Seeds:              100,
		ShareRatio:         100,
	}
}

func (q *QBit) setTorrentTags(t *storage.Entry, tags []string) {
	q.mu.Lock()
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if !utils.Contains(t.Tags, tag) {
			t.Tags = append(t.Tags, tag)
		}
		if !utils.Contains(q.tags, tag) {
			q.tags = append(q.tags, tag)
		}
	}
	q.mu.Unlock()
	_ = q.manager.Queue().Update(t)
}

func (q *QBit) removeTorrentTags(t *storage.Entry, tags []string) bool {
	newTorrentTags := utils.RemoveItem(t.Tags, tags...)
	q.mu.Lock()
	q.tags = utils.RemoveItem(q.tags, tags...)
	q.mu.Unlock()
	t.Tags = newTorrentTags
	_ = q.manager.Queue().Update(t)
	return true
}

func (q *QBit) addTags(tags []string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if !utils.Contains(q.tags, tag) {
			q.tags = append(q.tags, tag)
		}
	}
	return true
}
