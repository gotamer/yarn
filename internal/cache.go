package internal

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"

	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/types"
)

const (
	feedCacheFile    = "cache"
	feedCacheVersion = 1 // increase this if breaking changes occur to cache file.
)

// FilterFunc...
type FilterFunc func(twt types.Twt) bool

func FilterOutFeedsAndBotsFactory(conf *Config) FilterFunc {
	seen := make(map[string]bool)
	isLocal := IsLocalURLFactory(conf)
	return func(twt types.Twt) bool {
		if seen[twt.Hash()] {
			return false
		}
		seen[twt.Hash()] = true

		twter := twt.Twter()
		if strings.HasPrefix(twter.URL, "https://feeds.twtxt.net") {
			return false
		}
		if strings.HasPrefix(twter.URL, "https://search.twtxt.net") {
			return false
		}
		if isLocal(twter.URL) && HasString(twtxtBots, twter.Nick) {
			return false
		}
		return true
	}
}

// Cached ...
type Cached struct {
	mu           sync.RWMutex
	cache        types.TwtMap
	Twts         types.Twts
	Lastmodified string
}

// Lookup ...
func (cached *Cached) Lookup(hash string) (types.Twt, bool) {
	cached.mu.RLock()
	twt, ok := cached.cache[hash]
	cached.mu.RUnlock()
	if ok {
		return twt, true
	}

	for _, twt := range cached.Twts {
		if twt.Hash() == hash {
			cached.mu.Lock()
			if cached.cache == nil {
				cached.cache = make(map[string]types.Twt)
			}
			cached.cache[hash] = twt
			cached.mu.Unlock()
			return twt, true
		}
	}

	return types.NilTwt, false
}

// OldCache ...
type OldCache map[string]*Cached

// Cache ...
type Cache struct {
	mu      sync.RWMutex
	Version int
	Twts    map[string]*Cached
}

// Store ...
func (cache *Cache) Store(path string) error {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)

	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	f, err := os.OpenFile(filepath.Join(path, feedCacheFile), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening cache file for writing")
		return err
	}

	defer f.Close()

	if _, err = f.Write(b.Bytes()); err != nil {
		log.WithError(err).Error("error writing cache file")
		return err
	}
	return nil
}

// LoadCache ...
func LoadCache(path string) (*Cache, error) {
	cache := &Cache{
		Twts: make(map[string]*Cached),
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()

	f, err := os.Open(filepath.Join(path, feedCacheFile))
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading cache, cache file found but unreadable")
			return nil, err
		}
		cache.Version = feedCacheVersion
		return cache, nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	err = dec.Decode(&cache)
	if err != nil {
		if strings.Contains(err.Error(), "wrong type") {
			log.WithError(err).Error("error decoding cache. removing corrupt file.")
			// Remove invalid cache file.
			os.Remove(filepath.Join(path, feedCacheFile))
			cache.Version = feedCacheVersion
			cache.Twts = make(map[string]*Cached)

			return cache, nil
		}

		log.WithError(err).Error("error decoding cache (trying OldCache)")

		_, _ = f.Seek(0, io.SeekStart)
		oldcache := make(OldCache)
		dec := gob.NewDecoder(f)
		err = dec.Decode(&oldcache)
		if err != nil {
			log.WithError(err).Error("error decoding cache. removing corrupt file.")
			// Remove invalid cache file.
			os.Remove(filepath.Join(path, feedCacheFile))
			cache.Version = feedCacheVersion
			cache.Twts = make(map[string]*Cached)

			return cache, nil
		}
		cache.Version = feedCacheVersion
		for url, cached := range oldcache {
			cache.Twts[url] = cached
		}
	}

	log.Infof("Cache version %d", cache.Version)
	if cache.Version != feedCacheVersion {
		log.Errorf("Cache version mismatch. Expect = %d, Got = %d. Removing old cache.", feedCacheVersion, cache.Version)
		os.Remove(filepath.Join(path, feedCacheFile))
		cache.Version = feedCacheVersion
		cache.Twts = make(map[string]*Cached)
	}

	return cache, nil
}

const maxfetchers = 50

// FetchTwts ...
func (cache *Cache) FetchTwts(conf *Config, archive Archiver, feeds types.Feeds, publicFollowers map[types.Feed][]string) {
	stime := time.Now()
	defer func() {
		metrics.Gauge(
			"cache",
			"last_processed_seconds",
		).Set(
			float64(time.Since(stime) / 1e9),
		)
	}()

	isLocalURL := IsLocalURLFactory(conf)

	// buffered to let goroutines write without blocking before the main thread
	// begins reading
	twtsch := make(chan types.Twts, len(feeds))

	var wg sync.WaitGroup
	// max parallel http fetchers
	var fetchers = make(chan struct{}, maxfetchers)

	metrics.Gauge("cache", "sources").Set(float64(len(feeds)))

	seen := make(map[string]bool)
	for feed := range feeds {
		// Skip feeds we've already fetched by URI
		// (but possibly referenced by different nicknames/aliases)
		if _, ok := seen[feed.URL]; ok {
			continue
		}

		wg.Add(1)
		seen[feed.URL] = true
		fetchers <- struct{}{}

		// anon func takes needed variables as arg, avoiding capture of iterator variables
		go func(feed types.Feed) {
			defer func() {
				<-fetchers
				wg.Done()
			}()

			// Handle Gopher feeds
			// TODO: Refactor this into some kind of sensible interface
			if strings.HasPrefix(feed.URL, "gopher://") {
				res, err := RequestGopher(conf, feed.URL)
				if err != nil {
					log.WithError(err).Errorf("error fetching feed %s", feed)
					twtsch <- nil
					return
				}

				limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}

				twter := types.Twter{Nick: feed.Nick}
				if isLocalURL(feed.URL) {
					twter.URL = URLForUser(conf.BaseURL, feed.Nick)
					twter.Avatar = URLForAvatar(conf.BaseURL, feed.Nick, "")
				} else {
					twter.URL = feed.URL
					avatar := GetExternalAvatar(conf, twter)
					if avatar != "" {
						twter.Avatar = URLForExternalAvatar(conf, feed.URL)
					}
				}

				tf, err := types.ParseFile(limitedReader, twter)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed %s", feed)
					twtsch <- nil
					return
				}
				twter = tf.Twter()
				if !isLocalURL(twter.Avatar) {
					_ = GetExternalAvatar(conf, twter)
				}
				future, twts, old := types.SplitTwts(tf.Twts(), conf.MaxCacheTTL, conf.MaxCacheItems)
				if len(future) > 0 {
					log.Warnf(
						"feed %s has %d posts in the future, possible bad client or misconfigured timezone",
						feed, len(future),
					)
				}

				// If N == 0 we possibly exceeded conf.MaxFetchLimit when
				// reading this feed. Log it and bump a cache_limited counter
				if limitedReader.N <= 0 {
					log.Warnf(
						"feed size possibly exceeds MaxFetchLimit of %s for %s",
						humanize.Bytes(uint64(conf.MaxFetchLimit)),
						feed,
					)
					metrics.Counter("cache", "limited").Inc()
				}

				// Archive twts (opportunistically)
				archiveTwts := func(twts []types.Twt) {
					for _, twt := range twts {
						if !archive.Has(twt.Hash()) {
							if err := archive.Archive(twt); err != nil {
								log.WithError(err).Errorf("error archiving twt %s aborting", twt.Hash())
								metrics.Counter("archive", "error").Inc()
							} else {
								metrics.Counter("archive", "size").Inc()
							}
						}
					}
				}
				archiveTwts(old)
				archiveTwts(twts)

				cache.mu.Lock()
				cache.Twts[feed.URL] = &Cached{
					cache:        make(map[string]types.Twt),
					Twts:         twts,
					Lastmodified: "",
				}
				cache.mu.Unlock()

				twtsch <- twts
				return
			}

			headers := make(http.Header)

			if publicFollowers != nil {
				feedFollowers := publicFollowers[feed]

				// if no users are publicly following this feed, we rely on the
				// default User-Agent set in the `Request(…)` down below
				if len(feedFollowers) > 0 {
					var userAgent string
					if len(feedFollowers) == 1 {
						userAgent = fmt.Sprintf(
							"yarnd/%s (+%s; @%s)",
							yarn.FullVersion(),
							URLForUser(conf.BaseURL, feedFollowers[0]), feedFollowers[0],
						)
					} else {
						userAgent = fmt.Sprintf(
							"yarnd/%s (~%s; contact=%s)",
							yarn.FullVersion(),
							URLForWhoFollows(conf.BaseURL, feed, len(feedFollowers)),
							URLForPage(conf.BaseURL, "support"),
						)
					}
					headers.Set("User-Agent", userAgent)
				}
			}

			cache.mu.RLock()
			if cached, ok := cache.Twts[feed.URL]; ok {
				if cached.Lastmodified != "" {
					headers.Set("If-Modified-Since", cached.Lastmodified)
				}
			}
			cache.mu.RUnlock()

			res, err := Request(conf, http.MethodGet, feed.URL, headers)
			if err != nil {
				log.WithError(err).Errorf("error fetching feed %s", feed)
				twtsch <- nil
				return
			}
			defer res.Body.Close()

			actualurl := res.Request.URL.String()
			if actualurl != feed.URL {
				log.WithError(err).Warnf("feed for %s changed from %s to %s", feed.Nick, feed.URL, actualurl)
				cache.mu.Lock()
				if cached, ok := cache.Twts[feed.URL]; ok {
					cache.Twts[actualurl] = cached
				}
				cache.mu.Unlock()
				feed.URL = actualurl
			}

			if feed.URL == "" {
				log.WithField("feed", feed).Warn("empty url")
				twtsch <- nil
				return
			}

			var twts types.Twts

			switch res.StatusCode {
			case http.StatusOK: // 200
				limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}

				twter := types.Twter{Nick: feed.Nick}
				if isLocalURL(feed.URL) {
					twter.URL = URLForUser(conf.BaseURL, feed.Nick)
					twter.Avatar = URLForAvatar(conf.BaseURL, feed.Nick, "")
				} else {
					twter.URL = feed.URL
					avatar := GetExternalAvatar(conf, twter)
					if avatar != "" {
						twter.Avatar = URLForExternalAvatar(conf, feed.URL)
					}
				}

				tf, err := types.ParseFile(limitedReader, twter)
				if err != nil {
					log.WithError(err).Errorf("error parsing feed %s", feed)
					twtsch <- nil
					return
				}
				twter = tf.Twter()
				if !isLocalURL(twter.Avatar) {
					_ = GetExternalAvatar(conf, twter)
				}
				future, twts, old := types.SplitTwts(tf.Twts(), conf.MaxCacheTTL, conf.MaxCacheItems)
				if len(future) > 0 {
					log.Warnf(
						"feed %s has %d posts in the future, possible bad client or misconfigured timezone",
						feed, len(future),
					)
				}

				// If N == 0 we possibly exceeded conf.MaxFetchLimit when
				// reading this feed. Log it and bump a cache_limited counter
				if limitedReader.N <= 0 {
					log.Warnf(
						"feed size possibly exceeds MaxFetchLimit of %s for %s",
						humanize.Bytes(uint64(conf.MaxFetchLimit)),
						feed,
					)
					metrics.Counter("cache", "limited").Inc()
				}

				// Archive twts (opportunistically)
				archiveTwts := func(twts []types.Twt) {
					for _, twt := range twts {
						if !archive.Has(twt.Hash()) {
							if err := archive.Archive(twt); err != nil {
								log.WithError(err).Errorf("error archiving twt %s aborting", twt.Hash())
								metrics.Counter("archive", "error").Inc()
							} else {
								metrics.Counter("archive", "size").Inc()
							}
						}
					}
				}
				archiveTwts(old)
				archiveTwts(twts)

				lastmodified := res.Header.Get("Last-Modified")
				cache.mu.Lock()
				cache.Twts[feed.URL] = &Cached{
					cache:        make(map[string]types.Twt),
					Twts:         twts,
					Lastmodified: lastmodified,
				}
				cache.mu.Unlock()
			case http.StatusNotModified: // 304
				cache.mu.RLock()
				if _, ok := cache.Twts[feed.URL]; ok {
					twts = cache.Twts[feed.URL].Twts
				}
				cache.mu.RUnlock()
			}

			twtsch <- twts
		}(feed)
	}

	// close twts channel when all goroutines are done
	go func() {
		wg.Wait()
		close(twtsch)
	}()

	for range twtsch {
	}

	cache.mu.RLock()
	metrics.Gauge("cache", "feeds").Set(float64(len(cache.Twts)))
	count := 0
	for _, cached := range cache.Twts {
		count += len(cached.Twts)
	}
	cache.mu.RUnlock()
	metrics.Gauge("cache", "twts").Set(float64(count))
}

// Lookup ...
func (cache *Cache) Lookup(hash string) (types.Twt, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	for _, cached := range cache.Twts {
		twt, ok := cached.Lookup(hash)
		if ok {
			return twt, true
		}
	}
	return types.NilTwt, false
}

func (cache *Cache) Count() int {
	var count int
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	for _, cached := range cache.Twts {
		count += len(cached.Twts)
	}

	return count
}

// GetAll ...
func (cache *Cache) GetAll() types.Twts {
	var alltwts types.Twts
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	for _, cached := range cache.Twts {
		alltwts = append(alltwts, cached.Twts...)
	}

	sort.Sort(alltwts)
	return alltwts
}

// FilterBy ...
func (cache *Cache) FilterBy(f FilterFunc) types.Twts {
	var filteredtwts types.Twts

	alltwts := cache.GetAll()
	for _, twt := range alltwts {
		if f(twt) {
			filteredtwts = append(filteredtwts, twt)
		}
	}

	return filteredtwts
}

// GetMentions ...
func (cache *Cache) GetMentions(u *User) (twts types.Twts) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	seen := make(map[string]bool)

	// Search for @mentions in the cache against all Twts (local, followed and even external if any)
	for _, twt := range cache.GetAll() {
		for _, mention := range twt.Mentions() {
			if u.Is(mention.Twter().URL) && !seen[twt.Hash()] {
				twts = append(twts, twt)
				seen[twt.Hash()] = true
			}
		}
	}

	return
}

// GetByPrefix ...
func (cache *Cache) GetByPrefix(prefix string, refresh bool) types.Twts {
	key := fmt.Sprintf("prefix:%s", prefix)
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cached, ok := cache.Twts[key]
	if ok && !refresh {
		return cached.Twts
	}

	var twts types.Twts

	for url, cached := range cache.Twts {
		if strings.HasPrefix(url, prefix) {
			twts = append(twts, cached.Twts...)
		}
	}

	sort.Sort(twts)
	cache.Twts[key] = &Cached{
		cache:        make(map[string]types.Twt),
		Twts:         twts,
		Lastmodified: time.Now().Format(time.RFC3339),
	}

	return twts
}

// IsCached ...
func (cache *Cache) IsCached(url string) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	_, ok := cache.Twts[url]
	return ok
}

// GetByURL ...
func (cache *Cache) GetByURL(url string) types.Twts {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if cached, ok := cache.Twts[url]; ok {
		return cached.Twts
	}
	return types.Twts{}
}

// GetTwtsInConversation ...
func (cache *Cache) GetTwtsInConversation(hash string, replyTo types.Twt) types.Twts {
	subject := fmt.Sprintf("(#%s)", hash)
	return cache.GetBySubject(subject, replyTo)
}

// GetBySubject ...
func (cache *Cache) GetBySubject(subject string, replyTo types.Twt) types.Twts {
	var result types.Twts

	// TODO: Improve this by making this an O(1) lookup on the tag
	// XXX: But maybe this won't matter so much since the active cache
	//      is held in memory and is usually kept fairly small? 🤷‍♂️
	allTwts := cache.GetAll()

	seen := make(map[string]bool)
	for _, twt := range allTwts {
		if twt.Subject().String() == subject && !seen[twt.Hash()] {
			result = append(result, twt)
			seen[twt.Hash()] = true
		}
	}
	if !seen[replyTo.Hash()] {
		result = append(result, replyTo)
	}
	return result
}

// GetByTag ...
func (cache *Cache) GetByTag(tag string) types.Twts {
	var result types.Twts

	// TODO: Improve this by making this an O(1) lookup on the tag
	// XXX: But maybe this won't matter so much since the active cache
	//      is held in memory and is usually kept fairly small? 🤷‍♂️
	allTwts := cache.GetAll()

	seen := make(map[string]bool)
	for _, twt := range allTwts {
		var tags types.TagList = twt.Tags()
		if HasString(UniqStrings(tags.Tags()), tag) && !seen[twt.Hash()] {
			result = append(result, twt)
			seen[twt.Hash()] = true
		}
	}
	return result
}

// Delete ...
func (cache *Cache) Delete(feeds types.Feeds) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for feed := range feeds {
		delete(cache.Twts, feed.URL)
	}
}
