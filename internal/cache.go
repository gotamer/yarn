package internal

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	feedCacheVersion = 20 // increase this if breaking changes occur to cache file.

	localViewKey    = "local"
	discoverViewKey = "discover"

	podInfoUpdateTTL = time.Hour * 24
)

// FilterFunc ...
type FilterFunc func(twt types.Twt) bool

// GroupFunc ...
type GroupFunc func(twt types.Twt) []string

func FilterOutFeedsAndBotsFactory(conf *Config) FilterFunc {
	isLocal := IsLocalURLFactory(conf)
	return func(twt types.Twt) bool {
		twter := twt.Twter()
		if strings.HasPrefix(twter.URI, "https://feeds.twtxt.net") {
			return false
		}
		if strings.HasPrefix(twter.URI, "https://search.twtxt.net") {
			return false
		}
		if isLocal(twter.URI) && HasString(automatedFeeds, twter.Nick) {
			return false
		}
		return true
	}
}

func FilterByMentionFactory(u *User) FilterFunc {
	return func(twt types.Twt) bool {
		for _, mention := range twt.Mentions() {
			if u.Is(mention.Twter().URI) {
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

func ChunkTwts(twts types.Twts, chunkSize int) []types.Twts {
	var chunks []types.Twts
	for i := 0; i < len(twts); i += chunkSize {
		end := i + chunkSize

		// necessary check to avoid slicing beyond
		// slice capacity
		if end > len(twts) {
			end = len(twts)
		}

		chunks = append(chunks, twts[i:end])
	}

	return chunks
}

func FirstNTwts(twts types.Twts, n int) types.Twts {
	if n > len(twts) {
		return twts
	}
	return twts[:n]
}

// Cached ...
type Cached struct {
	mu sync.RWMutex

	Twts          types.Twts
	Dead          bool
	Errors        int
	LastError     string
	LastFetched   time.Time
	LastModified  string
	MovingAverage float64
}

func NewCached() *Cached {
	return &Cached{}
}

func NewCachedTwts(twts types.Twts, lastModified string) *Cached {
	return &Cached{
		Twts:         twts,
		LastModified: lastModified,
	}
}

// Inject ...
func (cached *Cached) Inject(url string, twt types.Twt) {
	cached.mu.Lock()
	defer cached.mu.Unlock()

	twts := UniqTwts(append(cached.Twts, twt))
	sort.Sort(twts)

	cached.Twts = twts
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

	// Calculate the moving average of a feed
	if cached.MovingAverage == 0.0 {
		cached.MovingAverage = 1.0
	} else {
		n := 0.0
		sum := 0.0
		for _, chunk := range ChunkTwts(FirstNTwts(twts, 6), 2) {
			if len(chunk) == 2 {
				dt := chunk[0].Created().Sub(chunk[1].Created())
				sum += dt.Seconds()
				n += 1
			}
		}
		avg := sum / n

		cached.MovingAverage = (cached.MovingAverage + avg) / 2
	}
}

// IsDead ...
func (cached *Cached) IsDead() bool {
	cached.mu.RLock()
	defer cached.mu.RUnlock()

	return cached.Dead
}

// GetTwts ...
func (cached *Cached) GetTwts() types.Twts {
	cached.mu.RLock()
	defer cached.mu.RUnlock()

	return cached.Twts
}

// GetLastModified ...
func (cached *Cached) GetLastModified() string {
	cached.mu.RLock()
	defer cached.mu.RUnlock()

	return cached.LastModified
}

// GetLastFetched ...
func (cached *Cached) GetLastFetched() time.Time {
	cached.mu.RLock()
	defer cached.mu.RUnlock()

	return cached.LastFetched
}

// GetMovingAverage ...
func (cached *Cached) GetMovingAverage() float64 {
	cached.mu.RLock()
	defer cached.mu.RUnlock()

	return cached.MovingAverage
}

// UpdateMovingAverage ...
func (cached *Cached) UpdateMovingAverage() {
	cached.mu.Lock()
	defer cached.mu.Unlock()

	if len(cached.Twts) > 0 {
		cached.MovingAverage = (cached.MovingAverage + time.Since(cached.Twts[0].Created()).Seconds()) / 2
	}
}

// SetError ...
func (cached *Cached) SetError(err error) {
	cached.mu.Lock()
	defer cached.mu.Unlock()

	if _, ok := err.(types.ErrDeadFeed); ok {
		cached.Dead = true
	}

	cached.Errors++
	cached.LastError = err.Error()
}

// SetLastFetched ...
func (cached *Cached) SetLastFetched() {
	cached.mu.Lock()
	defer cached.mu.Unlock()

	cached.LastFetched = time.Now()
}

type Peer struct {
	URI string `json:"-"`

	Name            string `json:"name"`
	Description     string `json:"description"`
	SoftwareVersion string `json:"software_version"`

	// Maybe we store future data about other peer pods in the future?
	// Right now the above is basically what is exposed now as the pod's name, description and what version of yarnd is running.
	// This information will likely be used for Pod Owner/Operators to manage Image Domain Whitelisting between pods and internal
	// automated operations like Pod Gossiping of Twts for things like Missing Root Twts for conversation views, etc.

	// lastSeen records the timestamp of when we last saw this pod.
	LastSeen time.Time `json:"-"`

	// lastUpdated is used to periodically re-check the peering pod's /info endpoint in case of changes.
	LastUpdated time.Time `json:"-"`
}

// XXX: Type aliases for backwards compatibility with Cache v19
type PodInfo Peer

func (p *Peer) IsZero() bool {
	return (p == nil) || (p.Name == "" && p.SoftwareVersion == "")
}

func (p *Peer) ShouldRefresh() bool {
	return time.Since(p.LastUpdated) > podInfoUpdateTTL
}

func (p *Peer) makeJsonRequest(conf *Config, path string) ([]byte, error) {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")

	res, err := Request(conf, http.MethodGet, p.URI+path, headers)
	if err != nil {
		log.WithError(err).Errorf("error making %s request to pod running at %s", path, p.URI)
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		log.Errorf("HTTP %s response for %s of pod running at %s", res.Status, path, p.URI)
		return nil, fmt.Errorf("non-success HTTP %s response for %s%s", res.Status, p.URI, path)
	}

	if ctype := res.Header.Get("Content-Type"); ctype != "" {
		mediaType, _, err := mime.ParseMediaType(ctype)
		if err != nil {
			log.WithError(err).Errorf("error parsing content type header '%s' for %s of pod running at %s", ctype, path, p.URI)
			return nil, err
		}
		if mediaType != "application/json" {
			log.Errorf("non-JSON response '%s' for %s of pod running at %s", ctype, path, p.URI)
			return nil, fmt.Errorf("non-JSON response content type '%s' for %s%s", ctype, p.URI, path)
		}
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.WithError(err).Errorf("error reading response body for %s of pod running at %s", path, p.URI)
		return nil, err
	}

	return data, nil
}

func (p *Peer) GetTwt(conf *Config, hash string) (types.Twt, error) {
	data, err := p.makeJsonRequest(conf, "/twt/"+hash)
	if err != nil {
		log.WithError(err).Errorf("error making /twt request for %s to peering pod %s", hash, p.URI)
		return nil, err
	}

	twt, err := types.DecodeJSON(data)
	if err != nil {
		log.WithError(err).Errorf("error deserializing Twt %s from peering pod %s", hash, p.URI)
		return nil, err
	}

	return twt, nil
}

type Peers []*Peer

func (peers Peers) Len() int           { return len(peers) }
func (peers Peers) Less(i, j int) bool { return strings.Compare(peers[i].Name, peers[j].Name) < 0 }
func (peers Peers) Swap(i, j int)      { peers[i], peers[j] = peers[j], peers[i] }

// Cache ...
type Cache struct {
	mu sync.RWMutex

	conf *Config

	Version int

	List  *Cached
	Map   map[string]types.Twt
	Peers map[string]*Peer
	Feeds map[string]*Cached
	Views map[string]*Cached

	Followers map[string]types.Followers
	Twters    map[string]*types.Twter
}

func NewCache(conf *Config) *Cache {
	return &Cache{
		conf: conf,

		Version: feedCacheVersion,

		Map:   make(map[string]types.Twt),
		Peers: make(map[string]*Peer),
		Feeds: make(map[string]*Cached),
		Views: make(map[string]*Cached),

		Followers: make(map[string]types.Followers),
		Twters:    make(map[string]*types.Twter),
	}
}

// FromOldCache attempts to load an oldver version of the on-disk cache stored
// at /path/to/data/cache -- If you change the way the `*Cache` is stored on disk
// by modifying `Cache.Store()` or any of the data structures, please modfy this
// function to support loading the previous version of the on-disk cache.
func FromOldCache(conf *Config) (*Cache, error) {
	cache := NewCache(conf)

	fn := filepath.Join(conf.Data, feedCacheFile)
	f, err := os.Open(fn)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading cache, cache file found but unreadable")
			return nil, err
		}
		return NewCache(conf), nil
	}
	defer f.Close()

	cleanupCorruptCache := func() (*Cache, error) {
		// Remove invalid cache file.
		os.Remove(fn)
		return NewCache(conf), nil
	}

	dec := gob.NewDecoder(f)

	if err := dec.Decode(&cache.Version); err != nil {
		log.WithError(err).Error("error decoding cache.Version, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Peers); err != nil {
		log.WithError(err).Error("error decoding cache.Peers, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Feeds); err != nil {
		log.WithError(err).Error("error decoding cache.Feeds, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Followers); err != nil {
		log.WithError(err).Warn("error decoding cache.Followers, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Twters); err != nil {
		log.WithError(err).Warn("error decoding cache.Twters, removing corrupt file")
		return cleanupCorruptCache()
	}

	log.Infof("Loaded old Cache v%d", cache.Version)

	// Migrate old Cache ...

	getLiteralTextFromTwt := func(twt types.Twt) string {
		var obj struct{ Text string }
		data, _ := json.Marshal(twt)
		json.Unmarshal(data, &obj)
		return obj.Text
	}

	cache.Version = feedCacheVersion

	for uri, twter := range cache.Twters {
		if twter.URI == "" {
			twter.URI = twter.URL
			twter.URL = ""
		}
		cache.Twters[uri] = twter
	}

	for uri, cached := range cache.Feeds {
		twts := make(types.Twts, len(cached.GetTwts()))
		for i, twt := range cached.GetTwts() {
			twter := types.Twter{
				Nick:      twt.Twter().Nick,
				URI:       twt.Twter().URL,
				Avatar:    twt.Twter().Avatar,
				Tagline:   twt.Twter().Tagline,
				Following: twt.Twter().Following,
				Followers: twt.Twter().Followers,
				Follow:    twt.Twter().Follow,
			}
			twts[i] = types.MakeTwt(twter, twt.Created(), getLiteralTextFromTwt(twt))
		}
		cache.Feeds[uri] = cached
	}

	cache.Refresh()

	if err := cache.Store(conf); err != nil {
		log.WithError(err).Errorf("error migrating old cache")
		return cleanupCorruptCache()
	}
	log.Infof("Successfully migrated old cache to v%d", cache.Version)

	return cache, nil
}

// LoadCache ...
func LoadCache(conf *Config) (*Cache, error) {
	cache := NewCache(conf)

	fn := filepath.Join(conf.Data, feedCacheFile)
	f, err := os.Open(fn)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Error("error loading cache, cache file found but unreadable")
			return nil, err
		}
		return NewCache(conf), nil
	}
	defer f.Close()

	dec := gob.NewDecoder(f)

	cleanupCorruptCache := func() (*Cache, error) {
		// Remove invalid cache file.
		os.Remove(fn)
		return NewCache(conf), nil
	}

	if err := dec.Decode(&cache.Version); err != nil {
		log.WithError(err).Error("error decoding cache.Version, removing corrupt file")
		return cleanupCorruptCache()
	}

	if cache.Version != feedCacheVersion {
		log.Warnf(
			"cache.Version %d does not match %d, will try to load old cache v%d instead...",
			cache.Version, feedCacheVersion, (feedCacheVersion - 1),
		)
		cache, err := FromOldCache(conf)
		if err != nil {
			log.WithError(err).Error("error loading old cache, removing corrupt file")
			return cleanupCorruptCache()
		}
		return cache, nil
	}

	if err := dec.Decode(&cache.Peers); err != nil {
		log.WithError(err).Error("error decoding cache.Peers, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Feeds); err != nil {
		log.WithError(err).Error("error decoding cache.Feeds, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Followers); err != nil {
		log.WithError(err).Warn("error decoding cache.Followers, removing corrupt file")
		return cleanupCorruptCache()
	}

	if err := dec.Decode(&cache.Twters); err != nil {
		log.WithError(err).Warn("error decoding cache.Twters, removing corrupt file")
		return cleanupCorruptCache()
	}

	log.Infof("Cache version %d", cache.Version)

	return cache, nil
}

// Store ...
func (cache *Cache) Store(conf *Config) error {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	fn := filepath.Join(conf.Data, feedCacheFile)
	f, err := os.OpenFile(fn, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.WithError(err).Error("error opening cache file for writing")
		return err
	}
	defer f.Close()

	enc := gob.NewEncoder(f)

	if err := enc.Encode(cache.Version); err != nil {
		log.WithError(err).Error("error encoding cache.Version")
		return err
	}

	if err := enc.Encode(cache.Peers); err != nil {
		log.WithError(err).Error("error encoding cache.Peers")
		return err
	}

	if err := enc.Encode(cache.Feeds); err != nil {
		log.WithError(err).Error("error encoding cache.Feeds")
		return err
	}

	if err := enc.Encode(cache.Followers); err != nil {
		log.WithError(err).Error("error encoding cache.Followers")
		return err
	}

	if err := enc.Encode(cache.Twters); err != nil {
		log.WithError(err).Error("error encoding cache.Twters")
		return err
	}

	return nil
}

func MergeFollowers(old, new types.Followers) types.Followers {
	var res types.Followers

	oldSet := make(map[string]*types.Follower)
	for _, o := range old {
		oldSet[o.URL] = o
		res = append(res, o)
	}

	for _, n := range new {
		if o, ok := oldSet[n.URL]; ok {
			o.LastFetchedAt = n.LastFetchedAt
		} else {
			res = append(res, n)
		}
	}

	return res
}

// DetectClientFromRequest ...
func (cache *Cache) DetectClientFromRequest(req *http.Request, profile types.Profile) error {
	ua, err := ParseUserAgent(req.UserAgent())
	if err != nil {
		return nil
	}

	// Detect Pod (if User-Agent is a pod) and update peering

	if ua.IsPod() {
		if err := cache.DetectPodFromUserAgent(ua); err != nil {
			log.WithError(err).Error("error detecting pod")
			return err
		}
	}

	// Update Followers cache

	newFollowers := ua.Followers(cache.conf)
	cache.mu.RLock()
	currentFollowers := cache.getFollowers(profile)
	cache.mu.RUnlock()
	mergedFollowers := MergeFollowers(currentFollowers, newFollowers)

	cache.mu.Lock()
	cache.Followers[profile.Username] = mergedFollowers
	cache.mu.Unlock()

	return nil
}

// DetectClientFromResponse ...
func (cache *Cache) DetectClientFromResponse(res *http.Response) error {
	poweredBy := res.Header.Get("Powered-By")
	if poweredBy == "" {
		return nil
	}

	ua, err := ParseUserAgent(poweredBy)
	if err != nil {
		log.WithError(err).Warnf("error parsing Powered-By header '%s'", poweredBy)
		return nil
	}

	if err := cache.DetectPodFromUserAgent(ua); err != nil {
		log.WithError(err).Error("error detecting pod")
	}

	return nil
}

// DetectPodFromUserAgent ...
func (cache *Cache) DetectPodFromUserAgent(ua TwtxtUserAgent) error {
	if !ua.IsPod() {
		return nil
	}

	if !cache.conf.Debug && !ua.IsPublicURL() {
		return nil
	}

	podBaseURL := ua.PodBaseURL()
	if podBaseURL == "" {
		return nil
	}

	cache.mu.RLock()
	oldPeer, hasSeen := cache.Peers[podBaseURL]
	cache.mu.RUnlock()

	if hasSeen && !oldPeer.ShouldRefresh() {
		// This might in fact race if another goroutine would have fetched the
		// pod info and updated the cache between our check above and the
		// update here. However, since we're only setting a timestamp when
		// we've last seen the peering pod, this should not be a problem at
		// all. We just override it a fraction of a second later. Doesn't harm
		// anything.
		cache.mu.Lock()
		oldPeer.LastSeen = time.Now()
		cache.mu.Unlock()
		return nil
	}

	// Set an empty &Peer{} to avoid multiple concurrent calls from making
	// multiple callbacks to peering pods unncessarily for Multi-User pods and
	// guard against race from other goroutine doing the same thing.
	cache.mu.Lock()
	oldPeer, hasSeen = cache.Peers[podBaseURL]
	if hasSeen && !oldPeer.ShouldRefresh() {
		cache.mu.Unlock()
		return nil
	}
	cache.Peers[podBaseURL] = &Peer{}
	cache.mu.Unlock()

	resetDummyPeer := func() {
		cache.mu.Lock()
		if oldPeer.IsZero() {
			delete(cache.Peers, podBaseURL)
		} else {
			cache.Peers[podBaseURL] = oldPeer
		}
		cache.mu.Unlock()
	}

	headers := make(http.Header)
	headers.Set("Accept", "application/json")

	res, err := Request(cache.conf, http.MethodGet, podBaseURL+"/info", headers)
	if err != nil {
		resetDummyPeer()
		log.WithError(err).Errorf("error making /info request to pod running at %s", podBaseURL)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		resetDummyPeer()
		log.Errorf("HTTP %s response for /info of pod running at %s", res.Status, podBaseURL)
		return fmt.Errorf("non-success HTTP %s response for %s/info", res.Status, podBaseURL)
	}

	if ctype := res.Header.Get("Content-Type"); ctype != "" {
		mediaType, _, err := mime.ParseMediaType(ctype)
		if err != nil {
			resetDummyPeer()
			log.WithError(err).Errorf("error parsing content type header '%s' for /info of pod running at %s", ctype, podBaseURL)
			return err
		}
		if mediaType != "application/json" {
			resetDummyPeer()
			log.Errorf("non-JSON response '%s' for /info of pod running at %s", ctype, podBaseURL)
			return fmt.Errorf("non-JSON response content type '%s' for %s/info", ctype, podBaseURL)
		}
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		resetDummyPeer()
		log.WithError(err).Errorf("error reading response body for /info of pod running at %s", podBaseURL)
		return err
	}

	var peer Peer

	if err := json.Unmarshal(data, &peer); err != nil {
		resetDummyPeer()
		log.WithError(err).Errorf("error decoding response body for /info of pod running at %s", podBaseURL)
		return err
	}
	peer.URI = podBaseURL
	peer.LastSeen = time.Now()
	peer.LastUpdated = time.Now()

	cache.mu.Lock()
	cache.Peers[podBaseURL] = &peer
	cache.mu.Unlock()

	return nil
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

			twter := cache.GetTwter(feed.URL)
			cachedFeed := cache.GetOrSetCachedFeed(feed.URL)

			if twter == nil {
				twter = &types.Twter{Nick: feed.Nick}
				if isLocalURL(feed.URL) {
					twter.URI = URLForUser(conf.BaseURL, feed.Nick)
					twter.Avatar = URLForAvatar(conf.BaseURL, feed.Nick, "")
				} else {
					twter.URI = feed.URL
					avatar := GetExternalAvatar(conf, *twter)
					if avatar != "" {
						twter.Avatar = URLForExternalAvatar(conf, feed.URL)
					}
				}
				cache.SetTwter(feed.URL, twter)
			}

			// Handle Feed Refresh
			// Supports two methods of refresh:
			// 1) A refresh interval (suggested refresh interval by feed author), e.g:
			//    # refresh = 1h
			// 2) An exponential back-off based on a weighted moving average of a feed's update frequency (TBD)
			if !cache.ShouldRefreshFeed(feed.URL) {
				twtsch <- nil
				return
			}

			// Update LastFetched time
			cachedFeed.SetLastFetched()

			// Handle Gopher feeds
			// TODO: Refactor this into some kind of sensible interface
			if strings.HasPrefix(feed.URL, "gopher://") {
				res, err := RequestGopher(conf, feed.URL)
				if err != nil {
					cachedFeed.SetError(err)
					log.WithError(err).Errorf("error fetching feed %s", feed)
					twtsch <- nil
					return
				}

				limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}

				tf, err := types.ParseFile(limitedReader, twter)
				if err != nil {
					cachedFeed.SetError(err)
					log.WithError(err).Errorf("error parsing feed %s", feed)
					twtsch <- nil
					return
				}
				if !isLocalURL(twter.Avatar) {
					_ = GetExternalAvatar(conf, *twter)
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

			if cachedFeed.GetLastModified() != "" {
				headers.Set("If-Modified-Since", cachedFeed.GetLastModified())
			}

			res, err := Request(conf, http.MethodGet, feed.URL, headers)
			if err != nil {
				cachedFeed.SetError(err)
				log.WithError(err).Errorf("error fetching feed %s", feed)
				twtsch <- nil
				return
			}
			defer res.Body.Close()

			actualURL := res.Request.URL.String()
			if actualURL == "" {
				log.WithField("feed", feed).Warnf("%s trying to redirect to an empty url", feed)
				twtsch <- nil
				return
			}

			if actualURL != feed.URL {
				log.WithError(err).Warnf("feed %s has moved to %s", feed, actualURL)
				cache.mu.Lock()
				cache.Feeds[actualURL] = cachedFeed
				cache.mu.Unlock()
				feed.URL = actualURL
			}

			cache.DetectClientFromResponse(res)

			var twts types.Twts

			switch res.StatusCode {
			case http.StatusOK: // 200
				limitedReader := &io.LimitedReader{R: res.Body, N: conf.MaxFetchLimit}

				tf, err := types.ParseFile(limitedReader, twter)
				if err != nil {
					cachedFeed.SetError(err)
					log.WithError(err).Errorf("error parsing feed %s", feed)
					twtsch <- nil
					return
				}
				if !isLocalURL(twter.Avatar) {
					_ = GetExternalAvatar(conf, *twter)
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
				twts = cachedFeed.GetTwts()
				cachedFeed.UpdateMovingAverage()
			case 401, 402, 403, 404, 407, 410, 451:
				// These are permanent 4xx errors and considered a dead feed
				cachedFeed.SetError(types.ErrDeadFeed{Reason: res.Status})
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
	return len(cache.List.GetTwts())
}

func GetPeersForCached(cached *Cached, peers map[string]*Peer) Peers {
	var matches Peers

	for _, twt := range cached.GetTwts() {
		twterURL := NormalizeURL(twt.Twter().URI)
		for uri, peer := range peers {
			if strings.HasPrefix(twterURL, NormalizeURL(uri)) {
				matches = append(matches, peer)
			}
		}
	}

	return matches
}

func RandomSubsetOfPeers(peers Peers, pct float64) Peers {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(peers), func(i, j int) { peers[i], peers[j] = peers[j], peers[i] })
	return peers[:int(math.Ceil(float64(len(peers))*pct))]
}

// Converge ...
func (cache *Cache) Converge(archive Archiver) {
	stime := time.Now()
	defer func() {
		metrics.Gauge(
			"cache",
			"last_convergence_seconds",
		).Set(
			float64(time.Since(stime) / 1e9),
		)
	}()

	// Missing Root Twts
	// Missing Twt Hash -> List of Peer(s)
	missingRootTwts := make(map[string][]*Peer)
	cache.mu.RLock()
	for subject, cached := range cache.Views {
		if !strings.HasPrefix(subject, "subject:") {
			continue
		}

		hash := ExtractHashFromSubject(subject)
		if _, inCache := cache.Map[hash]; inCache || archive.Has(hash) {
			continue
		}

		peers := GetPeersForCached(cached, cache.Peers)
		if len(peers) == 0 {
			peers = RandomSubsetOfPeers(cache.getPeers(), 0.6)
		}
		missingRootTwts[hash] = peers
	}
	cache.mu.RUnlock()

	metrics.Counter("cache", "missing_twts").Add(float64(len(missingRootTwts)))

	for hash, peers := range missingRootTwts {
		var missingTwt types.Twt
		for _, peer := range peers {
			if twt, err := peer.GetTwt(cache.conf, hash); err == nil {
				missingTwt = twt
				break
			}
		}
		if missingTwt != nil {
			cache.InjectFeed(missingTwt.Twter().URI, missingTwt)
			GetExternalAvatar(cache.conf, missingTwt.Twter())
		}
	}

	cache.Refresh()
}

// Refresh ...
func (cache *Cache) Refresh() {
	var allTwts types.Twts

	cache.mu.RLock()
	for _, cached := range cache.Feeds {
		allTwts = append(allTwts, cached.GetTwts()...)
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

		if isLocalURL(twt.Twter().URI) {
			localTwts = append(localTwts, twt)
		}

		if filterOutFeedsAndBots(twt) {
			discoverTwts = append(discoverTwts, twt)
		}
	}

	tags := GroupTwtsBy(allTwts, GroupByTag)
	subjects := GroupTwtsBy(allTwts, GroupBySubject)

	// XXX: I _think_ this is a bit of a hack.
	// Insert at the top of all subject views the original Twt (if any)
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
	cache.List = NewCachedTwts(allTwts, "")
	cache.Map = twtMap
	cache.Views = map[string]*Cached{
		localViewKey:    NewCachedTwts(localTwts, ""),
		discoverViewKey: NewCachedTwts(discoverTwts, ""),
	}
	for k, v := range tags {
		cache.Views["tag:"+k] = NewCachedTwts(v, "")
	}
	for k, v := range subjects {
		cache.Views["subject:"+k] = NewCachedTwts(v, "")
	}
	for k, peer := range cache.Peers {
		if (peer.LastSeen.Sub(peer.LastUpdated)) > (podInfoUpdateTTL/2) || time.Since(peer.LastUpdated) > podInfoUpdateTTL {
			delete(cache.Peers, k)
		}
	}
	cache.mu.Unlock()
}

// InjectFeed ...
func (cache *Cache) InjectFeed(url string, twt types.Twt) {
	cache.mu.RLock()
	cached, ok := cache.Feeds[url]
	cache.mu.RUnlock()

	if !ok {
		cache.mu.Lock()
		cache.Feeds[url] = NewCachedTwts(types.Twts{twt}, time.Now().Format(http.TimeFormat))
		cache.mu.Unlock()
	} else {
		cached.Inject(url, twt)
	}
}

// ShouldRefreshFeed ...
func (cache *Cache) ShouldRefreshFeed(url string) bool {
	cache.mu.RLock()
	cachedFeed, isCachedFeed := cache.Feeds[url]
	cache.mu.RUnlock()

	if !isCachedFeed {
		return true
	}

	// Skip feeds considered to be dead.
	if cachedFeed.IsDead() {
		return false
	}

	// Always refresh feeds on the same pod.
	if IsLocalURLFactory(cache.conf)(url) {
		return true
	}

	twter := cache.GetTwter(url)
	if twter == nil {
		return true
	}

	refresh := twter.Metadata.Get("refresh")
	if refresh != "" {
		if n, err := strconv.Atoi(refresh); err == nil {
			return int(time.Since(cachedFeed.GetLastFetched()).Seconds()) >= n
		}
	}

	// TODO: Implement exponential back-off using weighted moving average of a feed's update frequency
	if cache.conf.Features.IsEnabled(FeatureMovingAverageFeedRefresh) {
		movingAverage := cachedFeed.GetMovingAverage()
		log.Infof("Applying moving average refresh for feed %s: %0.2f", url)
		boundedMovingAverage := math.Max(60, math.Min(1800, movingAverage))
		return time.Since(cachedFeed.GetLastFetched()).Seconds() >= boundedMovingAverage
	}

	return true
}

// UpdateFeed ...
func (cache *Cache) UpdateFeed(url, lastmodified string, twts types.Twts) {
	cache.mu.RLock()
	cached, ok := cache.Feeds[url]
	cache.mu.RUnlock()

	if !ok {
		cache.mu.Lock()
		cache.Feeds[url] = NewCachedTwts(twts, lastmodified)
		cache.mu.Unlock()
	} else {
		cached.Update(url, lastmodified, twts)
	}
}

func (cache *Cache) getFollowers(profile types.Profile) types.Followers {
	followers := cache.Followers[profile.Username]
	sort.Sort(followers)
	return followers
}

// GetFollowers ...
// XXX: Returns a map[string]string of nick -> url for API compat
func (cache *Cache) GetFollowers(profile types.Profile) map[string]string {
	followers := make(map[string]string)

	cache.mu.RLock()
	defer cache.mu.RUnlock()

	for _, follower := range cache.getFollowers(profile) {
		followers[follower.Nick] = follower.URL
	}

	return followers
}

func (cache *Cache) FollowedBy(user *User, uri string) bool {
	cache.mu.RLock()
	followers := cache.Followers[user.Username]
	cache.mu.RUnlock()

	followersByURL := make(map[string]bool)
	for _, follower := range followers {
		followersByURL[follower.URL] = true
	}

	return followersByURL[uri]
}

func (cache *Cache) getPeers() (peers Peers) {
	for k, peer := range cache.Peers {
		if k == "" || peer.IsZero() {
			continue
		}
		peers = append(peers, peer)
	}

	sort.Sort(peers)

	return
}

// GetPeers ...
func (cache *Cache) GetPeers() (peers Peers) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return cache.getPeers()
}

// GetAll ...
func (cache *Cache) GetAll(refresh bool) types.Twts {
	cache.mu.RLock()
	cached := cache.List
	cache.mu.RUnlock()

	if cached != nil && !refresh {
		return cached.GetTwts()
	}

	cache.Refresh()

	cache.mu.RLock()
	cached = cache.List
	cache.mu.RUnlock()

	return cached.GetTwts()
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
		return cached.GetTwts()
	}

	twts := cache.FilterBy(FilterByMentionFactory(u))

	cache.mu.Lock()
	cache.Views[key] = NewCachedTwts(twts, "")
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

// GetOrSetCachedFeed ...
func (cache *Cache) GetOrSetCachedFeed(url string) *Cached {
	cache.mu.RLock()
	cached, ok := cache.Feeds[url]
	cache.mu.RUnlock()

	if !ok {
		cached = NewCached()

		cache.mu.Lock()
		cache.Feeds[url] = cached
		cache.mu.Unlock()
	}

	return cached
}

// GetByView ...
func (cache *Cache) GetByView(key string) types.Twts {
	cache.mu.RLock()
	cached, ok := cache.Views[key]
	cache.mu.RUnlock()

	if ok {
		return cached.GetTwts()
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
		return cached.GetTwts()
	}

	var twts types.Twts

	for feed := range u.Sources() {
		twts = append(twts, cache.GetByURL(feed.URL)...)
	}
	twts = FilterTwts(u, twts)
	sort.Sort(twts)

	cache.mu.Lock()
	cache.Views[key] = NewCachedTwts(twts, "")
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
		return cached.GetTwts()
	}

	twts := FilterTwts(u, cache.GetByView(view))
	sort.Sort(twts)

	cache.mu.Lock()
	cache.Views[key] = NewCachedTwts(twts, "")
	cache.mu.Unlock()

	return twts
}

// GetByURL ...
func (cache *Cache) GetByURL(url string) types.Twts {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if cached, ok := cache.Feeds[url]; ok {
		return cached.GetTwts()
	}
	return types.Twts{}
}

// GetTwter ...
func (cache *Cache) GetTwter(uri string) *types.Twter {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.Twters[uri]
}

// SetTwter ...
func (cache *Cache) SetTwter(uri string, twter *types.Twter) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.Twters[uri] = twter
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

// PruneFollowers ...
func (cache *Cache) PruneFollowers(olderThan time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	for user, followers := range cache.Followers {
		sort.Sort(followers)
		for i, follower := range followers {
			if time.Since(follower.LastFetchedAt) < olderThan {
				followers = followers[i:]
				cache.Followers[user] = followers
				break
			}
		}
	}
}
