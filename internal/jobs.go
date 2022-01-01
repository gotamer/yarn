package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
)

type Job interface {
	fmt.Stringer
	Run()
}

// JobSpec ...
type JobSpec struct {
	Schedule string
	Factory  JobFactory
}

func NewJobSpec(schedule string, factory JobFactory) JobSpec {
	return JobSpec{schedule, factory}
}

var (
	Jobs        map[string]JobSpec
	StartupJobs map[string]JobSpec
)

func InitJobs(conf *Config) {
	Jobs = map[string]JobSpec{
		"SyncStore":         NewJobSpec("@every 1m", NewSyncStoreJob),
		"UpdateFeeds":       NewJobSpec(conf.FetchInterval, NewUpdateFeedsJob),
		"UpdateFeedSources": NewJobSpec("@every 15m", NewUpdateFeedSourcesJob),

		"ActiveUsers":       NewJobSpec("@hourly", NewActiveUsersJob),
		"DeleteOldSessions": NewJobSpec("@hourly", NewDeleteOldSessionsJob),

		"Stats":          NewJobSpec("@daily", NewStatsJob),
		"RotateFeeds":    NewJobSpec("0 0 1 * * 0", NewRotateFeedsJob),
		"PruneFollowers": NewJobSpec("0 0 2 * * 0", NewPruneFollowersJob),
		"PruneUsers":     NewJobSpec("0 0 3 * * 0", NewPruneUsersJob),

		"CreateAdminFeeds":     NewJobSpec("", NewCreateAdminFeedsJob),
		"CreateAutomatedFeeds": NewJobSpec("", NewCreateAutomatedFeedsJob),
	}

	StartupJobs = map[string]JobSpec{
		"RotateFeeds":          Jobs["RotateFeeds"],
		"UpdateFeeds":          Jobs["UpdateFeeds"],
		"UpdateFeedSources":    Jobs["UpdateFeedSources"],
		"CreateAdminFeeds":     Jobs["CreateAdminFeeds"],
		"CreateAutomatedFeeds": Jobs["CreateAutomatedFeeds"],
		"DeleteOldSessions":    Jobs["DeleteOldSessions"],
	}

}

type JobFactory func(conf *Config, cache *Cache, archive Archiver, store Store) Job

type SyncStoreJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewSyncStoreJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &SyncStoreJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *SyncStoreJob) String() string { return "SyncStore" }

func (job *SyncStoreJob) Run() {
	if err := job.db.Sync(); err != nil {
		log.WithError(err).Warn("error sycning store")
	}
	log.Info("synced store")
}

type StatsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewStatsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &StatsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *StatsJob) String() string { return "Stats" }

func (job *StatsJob) Run() {
	var (
		followers []string
		following []string
	)

	log.Infof("updating stats")

	adminUser, err := job.db.GetUser(job.conf.AdminUser)
	if err != nil {
		log.WithError(err).Warnf("error loading user object for AdminUser")
		return
	}

	archiveSize, err := job.archive.Count()
	if err != nil {
		log.WithError(err).Warn("unable to get archive size")
		return
	}

	feeds, err := job.db.GetAllFeeds()
	if err != nil {
		log.WithError(err).Warn("unable to get all feeds from database")
		return
	}

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	for _, feed := range feeds {
		followers = append(followers, MapStrings(StringValues(feed.Followers), NormalizeURL)...)
	}

	for _, user := range users {
		followers = append(followers, MapStrings(StringValues(user.Followers), NormalizeURL)...)
		following = append(following, MapStrings(StringValues(user.Following), NormalizeURL)...)
	}

	followers = UniqStrings(followers)
	following = UniqStrings(following)

	var twts int

	allFeeds, err := GetAllFeeds(job.conf)
	if err != nil {
		log.WithError(err).Warn("unable to get all local feeds")
		return
	}
	for _, feed := range allFeeds {
		count, err := GetFeedCount(job.conf, feed)
		if err != nil {
			log.WithError(err).Warnf("error getting feed count for %s", feed)
			return
		}
		twts += count
	}

	text := fmt.Sprintf(
		"🧮 USERS:%d FEEDS:%d TWTS:%d ARCHIVED:%d CACHE:%d FOLLOWERS:%d FOLLOWING:%d",
		len(users), len(feeds), twts, archiveSize, job.cache.TwtCount(), len(followers), len(following),
	)

	if _, err := AppendTwt(job.conf, job.db, adminUser, statsBot, text); err != nil {
		log.WithError(err).Warn("error updating stats feed")
	}
}

type UpdateFeedsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewUpdateFeedsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &UpdateFeedsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *UpdateFeedsJob) String() string { return "UpdateFeeds" }

func (job *UpdateFeedsJob) Run() {
	feeds, err := job.db.GetAllFeeds()
	if err != nil {
		log.WithError(err).Warn("unable to get all feeds from database")
		return
	}

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	log.Infof("updating feeds for %d users and  %d feeds", len(users), len(feeds))

	sources := make(types.Feeds)
	publicFollowers := make(map[types.Feed][]string)

	// Ensure all specialUsername feeds are in the cache
	for _, username := range specialUsernames {
		sources[types.Feed{Nick: username, URL: URLForUser(job.conf.BaseURL, username)}] = true
	}

	// Ensure all twtxtBots feeds are in the cache
	for _, bot := range automatedFeeds {
		sources[types.Feed{Nick: bot, URL: URLForUser(job.conf.BaseURL, bot)}] = true
	}

	for _, feed := range feeds {
		// Ensure we fetch the feed's own posts in the cache
		sources[types.Feed{Nick: feed.Name, URL: feed.URL}] = true
	}

	for _, user := range users {
		for feed := range user.Sources() {
			sources[feed] = true
			if user.IsFollowingPubliclyVisible {
				publicFollowers[feed] = append(publicFollowers[feed], user.Username)
			}
		}
	}

	log.Infof("updating %d sources", len(sources))
	job.cache.FetchFeeds(job.conf, job.archive, sources, publicFollowers)

	log.Infof("converging cache with %d potential peers", len(job.cache.GetPeers()))
	job.cache.Converge(job.archive)

	log.Info("syncing feed cache")
	if err := job.cache.Store(job.conf); err != nil {
		log.WithError(err).Warn("error saving feed cache")
		return
	}

	log.Info("synced feed cache")

}

type UpdateFeedSourcesJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewUpdateFeedSourcesJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &UpdateFeedSourcesJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *UpdateFeedSourcesJob) String() string { return "UpdateFeedSources" }

func (job *UpdateFeedSourcesJob) Run() {
	log.Infof("updating %d feed sources", len(job.conf.FeedSources))

	feedsources := FetchFeedSources(job.conf, job.conf.FeedSources)

	log.Infof("fetched %d feed sources", len(feedsources.Sources))

	if err := SaveFeedSources(feedsources, job.conf.Data); err != nil {
		log.WithError(err).Warn("error saving feed sources")
	} else {
		log.Info("updated feed sources")
	}
}

type CreateAdminFeedsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewCreateAdminFeedsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &CreateAdminFeedsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *CreateAdminFeedsJob) String() string { return "CreateAdminFeeds" }

func (job *CreateAdminFeedsJob) Run() {
	log.Infof("creating feeds for admin user: %s", job.conf.AdminUser)

	if !job.db.HasUser(job.conf.AdminUser) {
		log.Warnf("no admin user account matching %s", job.conf.AdminUser)
		return
	}

	adminUser, err := job.db.GetUser(job.conf.AdminUser)
	if err != nil {
		log.WithError(err).Warnf("error loading user object for AdminUser")
		return
	}

	for _, feed := range specialUsernames {
		if !job.db.HasFeed(feed) {
			if err := CreateFeed(job.conf, job.db, adminUser, feed, true); err != nil {
				log.WithError(err).Warnf("error creating new feed %s for adminUser", feed)
			}
		}
	}

	if err := job.db.SetUser(adminUser.Username, adminUser); err != nil {
		log.WithError(err).Warn("error saving user object for AdminUser")
	}

}

type CreateAutomatedFeedsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewCreateAutomatedFeedsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &CreateAutomatedFeedsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *CreateAutomatedFeedsJob) String() string { return "CreateAutomatedFeeds" }

func (job *CreateAutomatedFeedsJob) Run() {
	log.Infof("creating automated feeds ...")

	// Create automated feeds
	for _, feed := range automatedFeeds {
		if !job.db.HasFeed(feed) {
			if err := CreateFeed(job.conf, job.db, nil, feed, true); err != nil {
				log.WithError(err).Warnf("error creating new feed %s", feed)
			}
		}
	}
}

type ActiveUsersJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewActiveUsersJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &ActiveUsersJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *ActiveUsersJob) String() string { return "ActiveUsers" }

func (job *ActiveUsersJob) Run() {
	log.Info("updating active user stats")

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	dau := 0
	mau := 0
	for _, user := range users {
		if time.Since(user.LastSeenAt) <= (24 * time.Hour) {
			dau++
		}
		if time.Since(user.LastSeenAt) <= (28 * 24 * time.Hour) {
			mau++
		}
	}

	metrics.Gauge("server", "dau").Set(float64(dau))
	metrics.Gauge("server", "mau").Set(float64(mau))
}

type DeleteOldSessionsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewDeleteOldSessionsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &DeleteOldSessionsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *DeleteOldSessionsJob) String() string { return "DeleteOldSessions" }

func (job *DeleteOldSessionsJob) Run() {
	log.Info("deleting old sessions")

	sessions, err := job.db.GetAllSessions()
	if err != nil {
		log.WithError(err).Error("error loading seessions")
		return
	}

	for _, session := range sessions {
		if session.Expired() {
			log.Infof("deleting expired session %s", session.ID)
			if err := job.db.DelSession(session.ID); err != nil {
				log.WithError(err).Error("error deleting session object")
			}
		}
	}
}

type RotateFeedsJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewRotateFeedsJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &RotateFeedsJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *RotateFeedsJob) String() string { return "RotateFeeds" }

func (job *RotateFeedsJob) Run() {
	feeds, err := GetAllFeeds(job.conf)
	if err != nil {
		log.WithError(err).Warn("unable to get all local feeds")
		return
	}

	for _, feed := range feeds {
		fn := filepath.Join(job.conf.Data, feedsDir, feed)
		stat, err := os.Stat(fn)
		if err != nil {
			log.WithError(err).Error("error getting feed size")
			continue
		}

		if stat.Size() > job.conf.MaxFetchLimit {
			log.Infof("rotating %s with size %s > %s", feed, humanize.Bytes(uint64(stat.Size())), humanize.Bytes(uint64(job.conf.MaxFetchLimit)))

			if err := RotateFeed(job.conf, feed); err != nil {
				log.WithError(err).Error("error rotating feed")
			} else {
				log.Infof("rotated feed %s", feed)
			}
		}
	}
}

type PruneFollowersJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewPruneFollowersJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &PruneFollowersJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *PruneFollowersJob) String() string { return "PruneFollowers" }

func (job *PruneFollowersJob) Run() {
	job.cache.PruneFollowers(90 * 24 * time.Hour)
}

type PruneUsersJob struct {
	conf    *Config
	cache   *Cache
	archive Archiver
	db      Store
}

func NewPruneUsersJob(conf *Config, cache *Cache, archive Archiver, db Store) Job {
	return &PruneUsersJob{conf: conf, cache: cache, archive: archive, db: db}
}

func (job *PruneUsersJob) String() string { return "PruneUsers" }

func (job *PruneUsersJob) Run() {
	candidateForDeletion := func(u *User) int {
		score := 1000

		if count, err := GetFeedCount(job.conf, u.Username); err == nil {
			if count == 0 {
				score *= 2
			}
		}

		if lastTwt, _, err := GetLastTwt(job.conf, u); err == nil {
			daysSinceLastTwt := int(time.Since(lastTwt.Created()).Hours() / 24)
			score += (daysSinceLastTwt % 100) * 10
		} else {
			score += 990
		}

		if len(u.Following) == 1 {
			score += 1
		}

		if u.Tagline == "" {
			score += 1
		}

		if u.AvatarHash == "" {
			score += 1
		}

		return score
	}

	users, err := job.db.GetAllUsers()
	if err != nil {
		log.WithError(err).Warn("unable to get all users from database")
		return
	}

	var candidates CandidatesByScore
	for _, user := range users {
		score := candidateForDeletion(user)
		if score > 1200 {
			log.Infof("user %s is a candidate for deletoin with score of %d", user.Username, score)
			candidates = append(candidates, DeletionCandidate{Username: user.Username, Score: score})
		}
	}
	sort.Sort(sort.Reverse(candidates))

	if len(candidates) > 10 {
		if err := SendCandidatesForDeletionEmail(job.conf, candidates[:10]); err != nil {
			log.WithError(err).Error("error sending candidates for deletion email")
		}
	} else {
		if err := SendCandidatesForDeletionEmail(job.conf, candidates); err != nil {
			log.WithError(err).Error("error sending candidates for deletion email")
		}
	}
}
