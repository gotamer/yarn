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
	feedCacheVersion = 16 // increase this if breaking changes occur to cache file.

	localViewKey    = "local"
	discoverViewKey = "discover"
)

// FilterFunc ...
type FilterFunc func(twt types.Twt) bool

// GroupFunc ...
type GroupFunc func(twt types.Twt) []string

func FilterOutFeedsAndBotsFactory(conf *Config) FilterFunc {
	isLocal := IsLocalURLFactory(conf)
	return func(twt types.Twt) bool {
		twter := twt.Twter()
		if strings.HasPrefix(twter.URL, "https://feeds.twtxt.net") {
			return false
		}
		if strings.HasPrefix(twter.URL, "https://search.twtxt.net") {
			return false
		}
		if isLocal(twter.URL) && HasString(automatedFeeds, twter.Nick) {
			return false
		}
		return true
	}
}

func FilterByMentionFactory(u *User) FilterFunc {
	return func(twt types.Twt) bool {
		for _, mention := range twt.Mentions() {
			if u.Is(mention.Twter().URL) {
				return true
			}
		}
		return false
	}
}

func GroupBySubject(twt types.Twt) []string {
	subject := strings.ToLower(twt.Subject().String())
	if subject == "" {
		return nil
	}
	return []string{subject}
}

func GroupByTag(twt types.Twt) (res []string) {
	var tagsList types.TagList = twt.Tags()
	seenTags := make(map[string]bool)
	for _, tag := range tagsList {
		tagText := strings.ToLower(tag.Text())
		if _, seenTag := seenTags[tagText]; !seenTag {
			res = append(res, tagText)
			seenTags[tagText] = true
		}
	}
	return
}

func FilterTwtsBy(twts types.Twts, f FilterFunc) (res types.Twts) {
	for _, twt := range twts {
		if f(twt) {
			res = append(res, twt)
		}
	}
	return
}

func GroupTwtsBy(twts types.Twts, g GroupFunc) (res map[string]types.Twts) {
	res = make(map[string]types.Twts)
	for _, twt := range twts {
		for _, key := range g(twt) {
			res[key] = append(res[key], twt)
		}
	}
	return
}

func UniqTwts(twts types.Twts) (res types.Twts) {
	seenTwts := make(map[string]bool)
	for _, twt := range twts {
		if _, seenTwt := seenTwts[twt.Hash()]; !seenTwt {
			res = append(res, twt)
			seenTwts[twt.Hash()] = true
		}
	}
	return
}

// Cached ...
type Cached struct {
	mu sync.RWMutex

	Twts         types.Twts
	LastModified string
}

func NewCached(twts types.Twts, lastModified string) *Cached {
	return &Cached{
		Twts:         twts,
		LastModified: lastModified,
	}
}

// Update ...
func (cached *Cached) Update(url, lastmodiied string, twts types.Twts) {
	// Avoid overwriting a cached Feed with no Twts
	if len(twts) == 0 {
		return
	}

	cached.mu.Lock()
	defer cached.mu.Unlock()

	cached.Twts = twts
	cached.LastModified = lastmodiied
}

// Cache ...
type Cache struct {
	mu sync.RWMutex

	conf *Config

	Version int

	List  *Cached
	Map   map[string]types.Twt
	Feeds map[string]*Cached
	Views map[string]*Cached
}

func NewCache(conf *Config) *Cache {
	return &Cache{
		conf:  conf,
		Map:   make(map[string]types.Twt),
		Feeds: make(map[string]*Cached),
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
			cache.Feeds = make(map[string]*Cached)
			return cache, nil
		}
	}

	log.Infof("Cache version %d", cache.Version)
	if cache.Version != feedCacheVersion {
		log.Errorf("Cache version mismatch. Expect = %d, Got = %d. Removing old cache.", feedCacheVersion, cache.Version)
		os.Remove(fn)
		cache.Version = feedCacheVersion
		cache.Feeds = make(map[string]*Cached)
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

	seenFeeds := make(map[string]bool)
	for feed := range feeds {
		// Normalize URLs
		feed.URL = NormalizeURL(feed.URL)

		// Skip feeds we've already fetched by URI
		// (but possibly referenced by different alias)
		if _, seenFeed := seenFeeds[feed.URL]; seenFeed {
			continue
		}

		// Skip feeds that are blacklisted.
		if cache.conf.BlacklistedFeed(feed.URL) {
			log.Warnf("attempt to fetch blacklisted feed %s", feed)
			continue
		}

		wg.Add(1)
		seenFeeds[feed.URL] = true
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
					log.Warnf("feed %s has %d posts in the future, possible bad client or misconfigured timezone", feed, len(future))
				}

				// If N == 0 we possibly exceeded conf.MaxFetchLimit when
				// reading this feed. Log it and bump a cache_limited counter
				if limitedReader.N <= 0 {
					log.Warnf("feed size possibly exceeds MaxFetchLimit of %s for %s", humanize.Bytes(uint64(conf.MaxFetchLimit)), feed)
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

				cache.UpdateFeed(feed.URL, "", twts)

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
			if cached, ok := cache.Feeds[feed.URL]; ok {
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
				if cached, ok := cache.Feeds[feed.URL]; ok {
					cache.Feeds[actualurl] = cached
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
					log.Warnf("feed %s has %d posts in the future, possible bad client or misconfigured timezone", feed, len(future))
				}

				// If N == 0 we possibly exceeded conf.MaxFetchLimit when
				// reading this feed. Log it and bump a cache_limited counter
				if limitedReader.N <= 0 {
					log.Warnf("feed size possibly exceeds MaxFetchLimit of %s for %s", humanize.Bytes(uint64(conf.MaxFetchLimit)), feed)
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
				cache.UpdateFeed(feed.URL, lastmodified, twts)
			case http.StatusNotModified: // 304
				cache.mu.RLock()
				if _, ok := cache.Feeds[feed.URL]; ok {
					twts = cache.Feeds[feed.URL].Twts
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
	metrics.Gauge("cache", "feeds").Set(float64(cache.FeedCount()))
	metrics.Gauge("cache", "twts").Set(float64(cache.TwtCount()))
}

// Lookup ...
func (cache *Cache) Lookup(hash string) (types.Twt, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	twt, ok := cache.Map[hash]
	if ok {
		return twt, true
	}
	return types.NilTwt, false
}

func (cache *Cache) FeedCount() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return len(cache.Feeds)
}

func (cache *Cache) TwtCount() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return len(cache.List.Twts)
}

// Refresh ...
func (cache *Cache) Refresh() {
	var allTwts types.Twts

	cache.mu.RLock()
	for _, cached := range cache.Feeds {
		allTwts = append(allTwts, cached.Twts...)
	}
	cache.mu.RUnlock()

	allTwts = UniqTwts(allTwts)
	sort.Sort(allTwts)

	//
	// Generate some default views...
	//

	var (
		localTwts    types.Twts
		discoverTwts types.Twts
	)

	twtMap := make(map[string]types.Twt)

	isLocalURL := IsLocalURLFactory(cache.conf)
	filterOutFeedsAndBots := FilterOutFeedsAndBotsFactory(cache.conf)
	for _, twt := range allTwts {
		twtMap[twt.Hash()] = twt

		if isLocalURL(twt.Twter().URL) {
			localTwts = append(localTwts, twt)
		}

		if filterOutFeedsAndBots(twt) {
			discoverTwts = append(discoverTwts, twt)
		}
	}

	tags := GroupTwtsBy(allTwts, GroupByTag)
	subjects := GroupTwtsBy(allTwts, GroupBySubject)

	// XXX: I _think_ this is a big of a hack.
	// Insert at the top of all subjet views the origina Twt (if any)
	// This is mostly to support "forked" conversations
	for k, v := range subjects {
		hash := ExtractHashFromSubject(k)
		if twt, ok := twtMap[hash]; ok {
			if len(v) > 0 && v[(len(v)-1)].Hash() != twt.Hash() {
				subjects[k] = append(subjects[k], twt)
			}
		}
	}

	cache.mu.Lock()
	cache.List = NewCached(allTwts, "")
	cache.Map = twtMap
	cache.Views = map[string]*Cached{
		localViewKey:    NewCached(localTwts, ""),
		discoverViewKey: NewCached(discoverTwts, ""),
	}
	for k, v := range tags {
		cache.Views["tag:"+k] = NewCached(v, "")
	}
	for k, v := range subjects {
		cache.Views["subject:"+k] = NewCached(v, "")
	}
	cache.mu.Unlock()
}

// UpdateFeed ...
func (cache *Cache) UpdateFeed(url, lastmodified string, twts types.Twts) {
	cache.mu.RLock()
	cached, ok := cache.Feeds[url]
	cache.mu.RUnlock()

	if !ok {
		cache.mu.Lock()
		cache.Feeds[url] = NewCached(twts, lastmodified)
		cache.mu.Unlock()
	} else {
		cached.Update(url, lastmodified, twts)
	}
}

// GetAll ...
func (cache *Cache) GetAll(refresh bool) types.Twts {
	cache.mu.RLock()
	cached := cache.List
	cache.mu.RUnlock()

	if cached != nil && !refresh {
		return cached.Twts
	}

	cache.Refresh()
	return cache.List.Twts
}

func (cache *Cache) FilterBy(f FilterFunc) types.Twts {
	return FilterTwtsBy(cache.GetAll(false), f)
}

func (cache *Cache) GroupBy(g GroupFunc) (res map[string]types.Twts) {
	return GroupTwtsBy(cache.GetAll(false), g)
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

	twts := cache.FilterBy(FilterByMentionFactory(u))

	cache.mu.Lock()
	cache.Views[key] = NewCached(twts, "")
	cache.mu.Unlock()

	return twts
}

// IsCached ...
func (cache *Cache) IsCached(url string) bool {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	_, ok := cache.Feeds[url]
	return ok
}

// GetByView ...
func (cache *Cache) GetByView(key string) types.Twts {
	cache.mu.RLock()
	cached, ok := cache.Views[key]
	cache.mu.RUnlock()

	if ok {
		return cached.Twts
	}
	return nil
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
	twts = FilterTwts(u, twts)
	sort.Sort(twts)

	cache.mu.Lock()
	cache.Views[key] = NewCached(twts, "")
	cache.mu.Unlock()

	return twts
}

// GetByUserView ...
func (cache *Cache) GetByUserView(u *User, view string, refresh bool) types.Twts {
	if u == nil {
		return cache.GetByView(view)
	}

	key := fmt.Sprintf("%s:%s", u.Username, view)

	cache.mu.RLock()
	cached, ok := cache.Views[key]
	cache.mu.RUnlock()

	if ok && !refresh {
		return cached.Twts
	}

	twts := FilterTwts(u, cache.GetByView(view))
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

	if cached, ok := cache.Feeds[url]; ok {
		return cached.Twts
	}
	return types.Twts{}
}

// DeleteUserViews ...
func (cache *Cache) DeleteUserViews(u *User) {
	cache.mu.Lock()
	delete(cache.Views, fmt.Sprintf("user:%s", u.Username))
	delete(cache.Views, fmt.Sprintf("discover:%s", u.Username))
	delete(cache.Views, fmt.Sprintf("mentions:%s", u.Username))
	cache.mu.Unlock()
}

// DeleteFeeds ...
func (cache *Cache) DeleteFeeds(feeds types.Feeds) {
	cache.mu.Lock()
	for feed := range feeds {
		delete(cache.Feeds, feed.URL)
	}
	cache.mu.Unlock()
	cache.Refresh()
}
