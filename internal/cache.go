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
	"time"

	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/types"
	"github.com/dustin/go-humanize"
	sync "github.com/sasha-s/go-deadlock"
	log "github.com/sirupsen/logrus"
)

const (
	feedCacheFile    = "cache"
	feedCacheVersion = 6 // increase this if breaking changes occur to cache file.

	localViewKey    = "local"
	discoverViewKey = "discover"
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
	LastModified string
}

func NewCached(twts types.Twts, lastModified string) *Cached {
	return &Cached{
		cache:        make(types.TwtMap),
		Twts:         twts,
		LastModified: lastModified,
	}
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

// Cache ...
type Cache struct {
	mu   sync.RWMutex
	conf *Config

	Version int

	All   *Cached
	Twts  map[string]*Cached
	Views map[string]*Cached
}

func NewCache(conf *Config) *Cache {
	return &Cache{
		conf:  conf,
		Twts:  make(map[string]*Cached),
		Views: make(map[string]*Cached),
	}
}

// Store ...
func (cache *Cache) Store(conf *Config) error {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	b := new(bytes.Buffer)
	enc := gob.NewEncoder(b)
	err := enc.Encode(cache)

	if err != nil {
		log.WithError(err).Error("error encoding cache")
		return err
	}

	fn := filepath.Join(conf.Data, feedCacheFile)
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0666)
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
func LoadCache(conf *Config) (*Cache, error) {
	cache := NewCache(conf)
	cache.mu.Lock()
	defer cache.mu.Unlock()

	fn := filepath.Join(conf.Data, feedCacheFile)
	f, err := os.Open(fn)
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
			os.Remove(fn)
			cache.Version = feedCacheVersion
			cache.Twts = make(map[string]*Cached)
			return cache, nil
		}
	}

	log.Infof("Cache version %d", cache.Version)
	if cache.Version != feedCacheVersion {
		log.Errorf("Cache version mismatch. Expect = %d, Got = %d. Removing old cache.", feedCacheVersion, cache.Version)
		os.Remove(fn)
		cache.Version = feedCacheVersion
		cache.Twts = make(map[string]*Cached)
	}

	return cache, nil
}

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
	var fetchers = make(chan struct{}, conf.MaxCacheFetchers)

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
				cache.Twts[feed.URL] = NewCached(twts, "")
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
				if cached.LastModified != "" {
					headers.Set("If-Modified-Since", cached.LastModified)
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
				cache.Twts[feed.URL] = NewCached(twts, lastmodified)
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

	// Bust and repopulate twts for GetAll()
	cache.Refresh()
	metrics.Gauge("cache", "feeds").Set(float64(cache.Feeds()))
	metrics.Gauge("cache", "twts").Set(float64(cache.Count()))
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

func (cache *Cache) Feeds() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return len(cache.Twts)
}

func (cache *Cache) Count() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return len(cache.All.Twts)
}

// Refresh ...
func (cache *Cache) Refresh() {
	var allTwts types.Twts

	cache.mu.RLock()
	for _, cached := range cache.Twts {
		allTwts = append(allTwts, cached.Twts...)
	}
	cache.mu.RUnlock()

	sort.Sort(allTwts)

	//
	// Generate some default views...
	//

	var (
		localTwts    types.Twts
		discoverTwts types.Twts
	)

	isLocalURL := IsLocalURLFactory(cache.conf)
	filterOutFeedsAndBots := FilterOutFeedsAndBotsFactory(cache.conf)
	for _, twt := range allTwts {
		if isLocalURL(twt.Twter().URL) {
			localTwts = append(localTwts, twt)
		}
		if filterOutFeedsAndBots(twt) {
			discoverTwts = append(discoverTwts, twt)
		}
	}

	cache.mu.Lock()
	cache.Views = map[string]*Cached{
		localViewKey:    NewCached(localTwts, ""),
		discoverViewKey: NewCached(discoverTwts, ""),
	}
	cache.All = NewCached(allTwts, "")
	cache.mu.Unlock()
}

// GetAll ...
func (cache *Cache) GetAll(refresh bool) types.Twts {
	cache.mu.RLock()
	cached := cache.All
	cache.mu.RUnlock()

	if cached != nil && !refresh {
		return cached.Twts
	}

	cache.Refresh()
	return cache.All.Twts
}

// FilterBy ...
func (cache *Cache) FilterBy(f FilterFunc) types.Twts {
	var filteredtwts types.Twts

	allTwts := cache.GetAll(false)
	for _, twt := range allTwts {
		if f(twt) {
			filteredtwts = append(filteredtwts, twt)
		}
	}

	return filteredtwts
}

// GetMentions ...
func (cache *Cache) GetMentions(u *User, refresh bool) types.Twts {
	key := fmt.Sprintf("mentions:%s", u.Username)

	cache.mu.RLock()
	cached, ok := cache.Views[key]
	cache.mu.RUnlock()

	if ok && !refresh {
		return cached.Twts
	}

	var twts types.Twts

	seen := make(map[string]bool)

	allTwts := cache.GetAll(false)

	// Search for @mentions in the cache against all Twts (local, followed and even external if any)
	for _, twt := range allTwts {
		for _, mention := range twt.Mentions() {
			if u.Is(mention.Twter().URL) && !seen[twt.Hash()] {
				twts = append(twts, twt)
				seen[twt.Hash()] = true
			}
		}
	}

	sort.Sort(twts)

	cache.mu.Lock()
	cache.Views[key] = NewCached(twts, "")
	cache.mu.Unlock()

	return twts
}

// IsCached ...
func (cache *Cache) IsCached(url string) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	_, ok := cache.Twts[url]
	return ok
}

// GetByView ...
func (cache *Cache) GetByView(key string) types.Twts {
	cache.mu.RLock()
	cached := cache.Views[key]
	cache.mu.RUnlock()

	return cached.Twts
}

// GetByUser ...
func (cache *Cache) GetByUser(u *User, refresh bool) types.Twts {
	key := fmt.Sprintf("user:%s", u.Username)

	cache.mu.RLock()
	cached, ok := cache.Views[key]
	cache.mu.RUnlock()

	if ok && !refresh {
		return cached.Twts
	}

	var twts types.Twts

	for feed := range u.Sources() {
		twts = append(twts, cache.GetByURL(feed.URL)...)
	}

	sort.Sort(twts)

	cache.mu.Lock()
	cache.Views[key] = NewCached(twts, "")
	cache.mu.Unlock()

	return twts
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

	allTwts := cache.GetAll(false)

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

	allTwts := cache.GetAll(false)

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

// DeleteUserViews ...
func (cache *Cache) DeleteUserViews(u *User) {
	cache.mu.Lock()
	delete(cache.Views, fmt.Sprintf("user:%s", u.Username))
	delete(cache.Views, fmt.Sprintf("mentions:%s", u.Username))
	cache.mu.Unlock()
}

// DeleteFeeds ...
func (cache *Cache) DeleteFeeds(feeds types.Feeds) {
	cache.mu.Lock()
	for feed := range feeds {
		delete(cache.Twts, feed.URL)
	}
	cache.mu.Unlock()
	cache.Refresh()
}
