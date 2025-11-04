package downloader

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	github "github.com/google/go-github/v50/github"
)

type ReleaseInfo struct {
	Launcher    string               `json:"launcher"`
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	PublishedAt time.Time            `json:"published_at"`
	Assets      []ReleaseAssetSimple `json:"assets"`
}

type ReleaseAssetSimple struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int    `json:"size"`
}

// DownloadLatest downloads assets for the latest release to destBase/launcher/tag, returns index.json path.
func DownloadLatest(ctx context.Context, launcher string, destBase string, proxyURL string, assetProxyURL string, xgetEnabled bool, xgetDomain string, rel *github.RepositoryRelease) (string, error) {
	if rel == nil {
		return "", errors.New("nil release")
	}
	version := rel.GetTagName()
	if version == "" {
		version = rel.GetName()
		if version == "" {
			version = fmt.Sprintf("%d", rel.GetID())
		}
	}
	dir := filepath.Join(destBase, launcher, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// 1. Prepare and write index.json
	var info ReleaseInfo
	info.Launcher = launcher
	info.TagName = rel.GetTagName()
	info.Name = rel.GetName()
	info.PublishedAt = rel.GetPublishedAt().Time
	for _, a := range rel.Assets {
		info.Assets = append(info.Assets, ReleaseAssetSimple{
			Name: a.GetName(),
			URL:  a.GetBrowserDownloadURL(), // Original URL
			Size: a.GetSize(),
		})
	}

	indexPath := filepath.Join(dir, "index.json")
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化 index.json 失败: %w", err)
	}
	if err := os.WriteFile(indexPath, b, 0o644); err != nil {
		return "", fmt.Errorf("写入 index.json 失败: %w", err)
	}
	log.Printf("已将版本信息写入 %s", indexPath)

	// Create a new HTTP client with proxy if specified
	client := &http.Client{Timeout: 5 * time.Minute}
	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return "", fmt.Errorf("解析代理URL失败: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxy)}
	}

	// 2. Loop through assets and download if necessary
	for _, asset := range rel.Assets {
		name := asset.GetName()
		outfile := filepath.Join(dir, name)

		// Check if file exists and is valid
		if fileInfo, err := os.Stat(outfile); err == nil {
			if fileInfo.Size() == int64(asset.GetSize()) {
				log.Printf("文件 %s 已存在且大小一致，跳过下载。", name)
				continue // Skip to next asset
			}
			log.Printf("文件 %s 已存在但大小不一致 (本地: %d, 远程: %d)，将重新下载。", name, fileInfo.Size(), asset.GetSize())
		}

		// Use the same robust download logic as before
		if err := func() error {
			url := asset.GetBrowserDownloadURL()
			if url != "" && assetProxyURL != "" {
				url = assetProxyURL + url
			}
			if url != "" && xgetEnabled && strings.HasPrefix(url, "https://github.com/") {
				url = strings.Replace(url, "https://github.com/", xgetDomain+"/gh/", 1)
			}
			if url == "" {
				// asset API URL requires auth header; skip for simplicity
				log.Printf("资源 %s 没有下载链接，跳过", name)
				return nil // Not an error, just skip this asset
			}
			if name == "" {
				name = filepath.Base(url)
			}
			log.Printf("开始下载 %s 到 %s", url, outfile)

			// download atomically
			partial := outfile + ".partial"

			var resp *http.Response
			var err error
			// simple retry
			for i := 0; i < 3; i++ {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
				if err != nil {
					return err
				}
				resp, err = client.Do(req)
				if err == nil && resp.StatusCode == http.StatusOK {
					break
				}
				if resp != nil {
					resp.Body.Close()
				}
				log.Printf("下载 %s 失败，5秒后重试...", url)
				time.Sleep(5 * time.Second)
			}
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("下载资源 %s 失败，状态码: %d", url, resp.StatusCode)
			}
			f, err := os.Create(partial)
			if err != nil {
				return err
			}
			defer func() {
				f.Close()
				os.Remove(partial) // removed if rename below succeeds
			}()

			// Log progress
			progressWriter := &progressWriter{
				total:      resp.ContentLength,
				fileName:   name,
				lastUpdate: time.Now(),
			}
			if _, err := io.Copy(f, io.TeeReader(resp.Body, progressWriter)); err != nil {
				return err
			}

			// close before rename
			if err := f.Close(); err != nil {
				return err
			}
			if err := os.Rename(partial, outfile); err != nil {
				return err
			}
			log.Printf("完成下载 %s", outfile)
			return nil
		}(); err != nil {
			log.Printf("下载 %s 失败: %v", name, err)
			return "", err // Make download failure a fatal error for the whole process
		}
	}

	return indexPath, nil
}

type progressWriter struct {
	total       int64
	written     int64
	fileName    string
	lastUpdate  time.Time
	lastWritten int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)

	now := time.Now()
	duration := now.Sub(pw.lastUpdate)
	if duration > time.Second || pw.written == pw.total {
		var speed float64
		if s := duration.Seconds(); s > 0 {
			speed = float64(pw.written-pw.lastWritten) / s
		}
		pw.lastWritten = pw.written
		pw.lastUpdate = now

		percentage := float64(pw.written) / float64(pw.total) * 100
		if pw.total > 0 {
			log.Printf("正在下载 %s: %d / %d 字节 (%.2f%%) - %.2f KB/s", pw.fileName, pw.written, pw.total, percentage, speed/1024)
		} else {
			log.Printf("正在下载 %s: %d 字节 - %.2f KB/s", pw.fileName, pw.written, speed/1024)
		}
	}

	return n, nil
}
