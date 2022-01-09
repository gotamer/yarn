package internal

import (
	_ "embed"
	"net/url"
	"regexp"
	"runtime"
	"time"
)

const (
	// InvalidConfigValue is the constant value for invalid config values
	// which must be changed for production configurations before successful
	// startup
	InvalidConfigValue = "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE"

	// DefaultDebug is the default debug mode
	DefaultDebug = false

	// DefaultData is the default data directory for storage
	DefaultData = "./data"

	// DefaultStore is the default data store used for accounts, sessions, etc
	DefaultStore = "bitcask://yarn.db"

	// DefaultBaseURL is the default Base URL for the app used to construct feed URLs
	DefaultBaseURL = "http://0.0.0.0:8000"

	// DefaultAdminUser is the default username to grant admin privileges to
	DefaultAdminUser = "admin"

	// DefaultAdminName is the default name of the admin user used in support requests
	DefaultAdminName = "Administrator"

	// DefaultAdminEmail is the default email of the admin user used in support requests
	DefaultAdminEmail = "support@yarn.social"

	// DefaultName is the default instance name
	DefaultName = "yarn.local"

	// DefaultMetaxxx are the default set of <meta> tags used on non-specific views
	DefaultMetaTitle       = ""
	DefaultMetaAuthor      = "Yarn.social"
	DefaultMetaKeywords    = "twtxt, twt, yarn, blog, micro-blog, microblogging, social, media, decentralised, pod"
	DefaultMetaDescription = "🧶 Yarn.social is a Self-Hosted, Twitter™-like Decentralised Microblogging social media platform. No ads, no tracking, your content, your data!"

	// DefaultTheme is the default theme to use for templates and static assets
	// (en empty value means to use the builtin default theme)
	DefaultTheme = ""

	// DefaultLang is the default language to use ('en' or 'zh-cn')
	DefaultLang = "auto"

	// DefaultOpenRegistrations is the default for open user registrations
	DefaultOpenRegistrations = false

	// DefaultDisableGzip is the default for disabling Gzip compression
	DefaultDisableGzip = false

	// DefaultDisableLogger is the default for disabling the Logger (access logs)
	DefaultDisableLogger = false

	// DefaultDisableMedia is the default for disabling Media support
	DefaultDisableMedia = false

	// DefaultDisableFfmpeg is the default for disabling ffmpeg support
	DefaultDisableFfmpeg = false

	// DefaultRegisterMessage is the default message displayed when  registrations are disabled
	DefaultRegisterMessage = ""

	// DefaultCookieSecret is the server's default cookie secret
	DefaultCookieSecret = InvalidConfigValue

	// DefaultTwtsPerPage is the server's default twts per page to display
	DefaultTwtsPerPage = 50

	// DefaultMaxTwtLength is the default maximum length of posts permitted
	DefaultMaxTwtLength = 288

	// DefaultMaxCacheTTL is the default maximum cache ttl of twts in memory
	DefaultMaxCacheTTL = time.Hour * 24 * 10 // 10 days 28 days 28 days 28 days

	// DefaultFetchInterval is the default interval used by the global feed cache
	// to control when to actually fetch and update feeds.
	DefaultFetchInterval = "@every 5m"

	// DefaultMaxCacheItems is the default maximum cache items (per feed source)
	// of twts in memory
	DefaultMaxCacheItems = DefaultTwtsPerPage * 3 // We get bored after paging thorughh > 3 pages :D

	// DefaultOpenProfiles is the default for whether or not to have open user profiles
	DefaultOpenProfiles = false

	// DefaultMaxUploadSize is the default maximum upload size permitted
	DefaultMaxUploadSize = 1 << 24 // ~16MB (enough for high-res photos)

	// DefaultSessionCacheTTL is the server's default session cache ttl
	DefaultSessionCacheTTL = 1 * time.Hour

	// DefaultSessionExpiry is the server's default session expiry time
	DefaultSessionExpiry = 240 * time.Hour // 10 days

	// DefaultTranscoderTimeout is the default vodeo transcoding timeout
	DefaultTranscoderTimeout = 10 * time.Minute // 10mins

	// DefaultMagicLinkSecret is the jwt magic link secret
	DefaultMagicLinkSecret = InvalidConfigValue

	// Default Messaging settings
	DefaultSMTPBind = "0.0.0.0:8025"
	DefaultPOP3Bind = "0.0.0.0:8110"

	// Default SMTP configuration
	DefaultSMTPHost = "smtp.gmail.com"
	DefaultSMTPPort = 587
	DefaultSMTPUser = InvalidConfigValue
	DefaultSMTPPass = InvalidConfigValue
	DefaultSMTPFrom = InvalidConfigValue

	// DefaultMaxFetchLimit is the maximum fetch fetch limit in bytes
	DefaultMaxFetchLimit = 1 << 20 // ~1MB (or more than enough for months)

	// DefaultAPISessionTime is the server's default session time for API tokens
	DefaultAPISessionTime = 240 * time.Hour // 10 days

	// DefaultAPISigningKey is the default API JWT signing key for tokens
	DefaultAPISigningKey = InvalidConfigValue

	// MinimumCacheFetchInterval is the smallest allowable cache fetch interval for
	// production pods, an attempt to configure a pod with a smaller value than this
	// results in a configuration validation error.
	MinimumCacheFetchInterval = 59 * time.Second

	// DefaultMediaResolution is the default resolution used to downscale media (iamges)
	// (the original is also preserved and accessible via adding the query string ?full=1)
	DefaultMediaResolution = 850 // 850px width (maintaining aspect ratio)

	// DefaultAvatarResolution is the default resolution used to downscale avatars (profiles)
	DefaultAvatarResolution = 360 // 360px width (maintaining aspect ratio)
)

var (
	// DefaultLogo is the default logo (SVG)
	//go:embed logo.svg
	DefaultLogo string

	// DefaultFeedSources is the default list of external feed sources
	DefaultFeedSources = []string{
		"https://feeds.twtxt.net/we-are-feeds.txt",
	}

	// DefaultTwtPrompts are the set of default prompts  for twt text(s)
	DefaultTwtPrompts = []string{
		`What's on your mind? 🤔`,
		`Share something insightful! 💡`,
		`Good day to you! 👌 What's new? 🥳`,
		`Did something cool lately? 🤔 Share it! 🤗`,
		`Hi! 👋 Don't forget to post a Twt today! 😉`,
		`Let's have a Yarn! ✋`,
	}

	// DefaultWhitelistedImages is the default list of image domains
	// to whitelist for external images to display them inline
	DefaultWhitelistedImages = []string{
		`imgur\.com`,
		`giphy\.com`,
		`imgs\.xkcd\.com`,
		`tube\.mills\.io`,
		`reactiongifs\.com`,
		`githubusercontent\.com`,
	}

	// DefaultBlacklistedFeeds is the default list of feed uris thar are
	// blacklisted and prohibuted from being fetched by the global feed cache
	DefaultBlacklistedFeeds = []string{}

	// DefaultMaxCacheFetchers is the default maximun number of fetchers used
	// by the global feed cache during update cycles. This controls how quickly
	// feeds are updated in each feed cache cycle. The default is the number of
	// available CPUs on the system.
	DefaultMaxCacheFetchers = runtime.NumCPU()

	// DefaultDisplayDatesInTimezone is the default timezone date and times are display in at the Pod level for
	// anonymous or unauthenticated users or users who have not changed their timezone rpefernece.
	DefaultDisplayDatesInTimezone = "UTC"

	// DefaultDisplayTimePreference is the default Pod level time representation (12hr or 24h) overridable by Users.
	DefaultDisplayTimePreference = "12h"

	// DefaultOpenLinksInPreference is the default Pod level behaviour for opening external links (overridable by Users).
	DefaultOpenLinksInPreference = "newwindow"

	// DisplayImagesPreference is the default Pod-level image display behaviour
	// (inline or gallery) for displaying images (overridable by Users).
	DefaultDisplayImagesPreference = "inline"
)

func NewConfig() *Config {
	return &Config{
		Version: version,
		Debug:   DefaultDebug,

		Name:                    DefaultName,
		Logo:                    DefaultLogo,
		Description:             DefaultMetaDescription,
		Store:                   DefaultStore,
		Theme:                   DefaultTheme,
		BaseURL:                 DefaultBaseURL,
		AdminUser:               DefaultAdminUser,
		FeedSources:             DefaultFeedSources,
		RegisterMessage:         DefaultRegisterMessage,
		CookieSecret:            DefaultCookieSecret,
		TwtPrompts:              DefaultTwtPrompts,
		TwtsPerPage:             DefaultTwtsPerPage,
		MaxTwtLength:            DefaultMaxTwtLength,
		AvatarResolution:        DefaultAvatarResolution,
		MediaResolution:         DefaultMediaResolution,
		OpenProfiles:            DefaultOpenProfiles,
		OpenRegistrations:       DefaultOpenRegistrations,
		DisableGzip:             DefaultDisableGzip,
		DisableLogger:           DefaultDisableLogger,
		DisableFfmpeg:           DefaultDisableFfmpeg,
		DisableMedia:            DefaultDisableMedia,
		Features:                NewFeatureFlags(),
		DisplayDatesInTimezone:  DefaultDisplayDatesInTimezone,
		DisplayTimePreference:   DefaultDisplayTimePreference,
		OpenLinksInPreference:   DefaultOpenLinksInPreference,
		DisplayImagesPreference: DefaultDisplayImagesPreference,
		SessionExpiry:           DefaultSessionExpiry,
		MagicLinkSecret:         DefaultMagicLinkSecret,
		SMTPHost:                DefaultSMTPHost,
		SMTPPort:                DefaultSMTPPort,
		SMTPUser:                DefaultSMTPUser,
		SMTPPass:                DefaultSMTPPass,
	}
}

// Option is a function that takes a config struct and modifies it
type Option func(*Config) error

// WithDebug sets the debug mode lfag
func WithDebug(debug bool) Option {
	return func(cfg *Config) error {
		cfg.Debug = debug
		return nil
	}
}

// WithData sets the data directory to use for storage
func WithData(data string) Option {
	return func(cfg *Config) error {
		cfg.Data = data
		return nil
	}
}

// WithStore sets the store to use for accounts, sessions, etc.
func WithStore(store string) Option {
	return func(cfg *Config) error {
		cfg.Store = store
		return nil
	}
}

// WithBaseURL sets the Base URL used for constructing feed URLs
func WithBaseURL(baseURL string) Option {
	return func(cfg *Config) error {
		u, err := url.Parse(baseURL)
		if err != nil {
			return err
		}
		cfg.BaseURL = baseURL
		cfg.baseURL = u
		return nil
	}
}

// WithAdminUser sets the Admin user used for granting special features to
func WithAdminUser(adminUser string) Option {
	return func(cfg *Config) error {
		cfg.AdminUser = adminUser
		return nil
	}
}

// WithAdminName sets the Admin name used to identify the pod operator
func WithAdminName(adminName string) Option {
	return func(cfg *Config) error {
		cfg.AdminName = adminName
		return nil
	}
}

// WithAdminEmail sets the Admin email used to contact the pod operator
func WithAdminEmail(adminEmail string) Option {
	return func(cfg *Config) error {
		cfg.AdminEmail = adminEmail
		return nil
	}
}

// WithFeedSources sets the feed sources  to use for external feeds
func WithFeedSources(feedSources []string) Option {
	return func(cfg *Config) error {
		cfg.FeedSources = feedSources
		return nil
	}
}

// WithName sets the instance's name
func WithName(name string) Option {
	return func(cfg *Config) error {
		cfg.Name = name
		return nil
	}
}

// WithDescription sets the instance's description
func WithDescription(description string) Option {
	return func(cfg *Config) error {
		cfg.Description = description
		return nil
	}
}

// WithTheme sets the theme to use for templates and static asssets
func WithTheme(theme string) Option {
	return func(cfg *Config) error {
		cfg.Theme = theme
		return nil
	}
}

// WithOpenRegistrations sets the open registrations flag
func WithOpenRegistrations(openRegistrations bool) Option {
	return func(cfg *Config) error {
		cfg.OpenRegistrations = openRegistrations
		return nil
	}
}

// WithDisableGzip sets the disable Gzip flag
func WithDisableGzip(disableGzip bool) Option {
	return func(cfg *Config) error {
		cfg.DisableGzip = disableGzip
		return nil
	}
}

// WithDisableLogger sets the disable Logger flag
func WithDisableLogger(disableLogger bool) Option {
	return func(cfg *Config) error {
		cfg.DisableLogger = disableLogger
		return nil
	}
}

// WithDisableMedia sets the disable Media flag
func WithDisableMedia(disablemedia bool) Option {
	return func(cfg *Config) error {
		cfg.DisableMedia = disablemedia
		return nil
	}
}

// WithDisableFfmpeg sets the disable ffmpeg flag
func WithDisableFfmpeg(disableFfmpeg bool) Option {
	return func(cfg *Config) error {
		cfg.DisableFfmpeg = disableFfmpeg
		return nil
	}
}

// WithCookieSecret sets the server's cookie secret
func WithCookieSecret(secret string) Option {
	return func(cfg *Config) error {
		cfg.CookieSecret = secret
		return nil
	}
}

// WithTwtsPerPage sets the server's twts per page
func WithTwtsPerPage(twtsPerPage int) Option {
	return func(cfg *Config) error {
		cfg.TwtsPerPage = twtsPerPage
		return nil
	}
}

// WithMaxTwtLength sets the maximum length of posts permitted on the server
func WithMaxTwtLength(maxTwtLength int) Option {
	return func(cfg *Config) error {
		cfg.MaxTwtLength = maxTwtLength
		return nil
	}
}

// WithMaxCacheTTL sets the maximum cache ttl of twts in memory
func WithMaxCacheTTL(maxCacheTTL time.Duration) Option {
	return func(cfg *Config) error {
		cfg.MaxCacheTTL = maxCacheTTL
		return nil
	}
}

// WithFetchInterval sets the cache fetch interval
// Accepts a string as parsed by `time.ParseDuration`
func WithFetchInterval(fetchInterval string) Option {
	return func(cfg *Config) error {
		cfg.FetchInterval = fetchInterval
		return nil
	}
}

// WithMaxCacheFetchers sets the maximum number of fetchers for the feed cache
func WithMaxCacheFetchers(maxCacheFetchers int) Option {
	return func(cfg *Config) error {
		cfg.MaxCacheFetchers = maxCacheFetchers
		return nil
	}
}

// WithMaxCacheItems sets the maximum cache items (per feed source) of twts in memory
func WithMaxCacheItems(maxCacheItems int) Option {
	return func(cfg *Config) error {
		cfg.MaxCacheItems = maxCacheItems
		return nil
	}
}

// WithOpenProfiles sets whether or not to have open user profiles
func WithOpenProfiles(openProfiles bool) Option {
	return func(cfg *Config) error {
		cfg.OpenProfiles = openProfiles
		return nil
	}
}

// WithMaxUploadSize sets the maximum upload size permitted by the server
func WithMaxUploadSize(maxUploadSize int64) Option {
	return func(cfg *Config) error {
		cfg.MaxUploadSize = maxUploadSize
		return nil
	}
}

// WithSessionCacheTTL sets the server's session cache ttl
func WithSessionCacheTTL(cacheTTL time.Duration) Option {
	return func(cfg *Config) error {
		cfg.SessionCacheTTL = cacheTTL
		return nil
	}
}

// WithSessionExpiry sets the server's session expiry time
func WithSessionExpiry(expiry time.Duration) Option {
	return func(cfg *Config) error {
		cfg.SessionExpiry = expiry
		return nil
	}
}

// WithTranscoderTimeout sets the video transcoding timeout
func WithTranscoderTimeout(timeout time.Duration) Option {
	return func(cfg *Config) error {
		cfg.TranscoderTimeout = timeout
		return nil
	}
}

// WithMagicLinkSecret sets the MagicLinkSecert used to create password reset tokens
func WithMagicLinkSecret(secret string) Option {
	return func(cfg *Config) error {
		cfg.MagicLinkSecret = secret
		return nil
	}
}

// WithSMTPHost sets the SMTPHost to use for sending email
func WithSMTPHost(host string) Option {
	return func(cfg *Config) error {
		cfg.SMTPHost = host
		return nil
	}
}

// WithSMTPPort sets the SMTPPort to use for sending email
func WithSMTPPort(port int) Option {
	return func(cfg *Config) error {
		cfg.SMTPPort = port
		return nil
	}
}

// WithSMTPUser sets the SMTPUser to use for sending email
func WithSMTPUser(user string) Option {
	return func(cfg *Config) error {
		cfg.SMTPUser = user
		return nil
	}
}

// WithSMTPPass sets the SMTPPass to use for sending email
func WithSMTPPass(pass string) Option {
	return func(cfg *Config) error {
		cfg.SMTPPass = pass
		return nil
	}
}

// WithSMTPFrom sets the SMTPFrom address to use for sending email
func WithSMTPFrom(from string) Option {
	return func(cfg *Config) error {
		cfg.SMTPFrom = from
		return nil
	}
}

// WithMaxFetchLimit sets the maximum feed fetch limit in bytes
func WithMaxFetchLimit(limit int64) Option {
	return func(cfg *Config) error {
		cfg.MaxFetchLimit = limit
		return nil
	}
}

// WithAPISessionTime sets the API session time for tokens
func WithAPISessionTime(duration time.Duration) Option {
	return func(cfg *Config) error {
		cfg.APISessionTime = duration
		return nil
	}
}

// WithAPISigningKey sets the API JWT signing key for tokens
func WithAPISigningKey(key string) Option {
	return func(cfg *Config) error {
		cfg.APISigningKey = key
		return nil
	}
}

// WithWhitelistedImages sets the list of image domains whitelisted
// and permitted for external iamges to display inline
func WithWhitelistedImages(whitelistedImages []string) Option {
	return func(cfg *Config) error {
		cfg.WhitelistedImages = whitelistedImages
		for _, whitelistedImage := range whitelistedImages {
			if whitelistedImage == "" {
				continue
			}
			re, err := regexp.Compile(whitelistedImage)
			if err != nil {
				return err
			}
			cfg.whitelistedImages = append(cfg.whitelistedImages, re)
		}
		return nil
	}
}

// WithBlacklistedFeeds sets the list of feed uris blacklisted
// and prohibited from being fetched by the global feed cache
func WithBlacklistedFeeds(blacklistedFeeds []string) Option {
	return func(cfg *Config) error {
		cfg.BlacklistedFeeds = blacklistedFeeds
		for _, blacklistedFeed := range blacklistedFeeds {
			if blacklistedFeed == "" {
				continue
			}
			re, err := regexp.Compile(blacklistedFeed)
			if err != nil {
				return err
			}
			cfg.blacklistedFeeds = append(cfg.blacklistedFeeds, re)
		}
		return nil
	}
}
