package internal

import (
	"net/url"
	"regexp"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
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

	// DefaultLogo is the default logo (SVG)
	DefaultLogo = `<svg aria-hidden="true" width="196.26" viewBox="100 50 294.389 108.746" height="70.739" xmlns="http://www.w3.org/2000/svg"><g transform="translate(1.302 -35.908)"><path fill="currentCOlor" stroke="null" d="M178.103 112.703c-7.338-7.366-17.095-11.423-27.472-11.423-10.378 0-20.135 4.057-27.473 11.423-6.965 6.992-10.957 16.17-11.345 25.993a1.23 1.23 0 0 0-.018.476c-.01.369-.016.738-.016 1.108 0 10.418 4.041 20.211 11.38 27.578a39.217 39.217 0 0 0 6.546 5.296 118.789 118.789 0 0 1-3.974-1.632c-4.94-2.1-8.84-3.76-14.312-2.468a1.222 1.222 0 0 0 .557 2.378c4.697-1.11 8.098.337 12.805 2.34 5.77 2.454 12.951 5.508 25.85 5.508 10.377 0 20.134-4.056 27.472-11.422 7.338-7.367 11.38-17.16 11.38-27.578 0-10.417-4.042-20.21-11.38-27.577zm.683 50.77-31.174 13.24a36.296 36.296 0 0 1-6.456-1.12l41.321-17.549a36.468 36.468 0 0 1-3.69 5.429zm-4.402 4.522a36.015 36.015 0 0 1-20.475 8.695zm9.721-13.273a1.21 1.21 0 0 0-.207.066l-46.282 19.656a35.902 35.902 0 0 1-4.352-2.013l52.447-22.274a36.187 36.187 0 0 1-1.606 4.565zm2.292-7.508-55.717 23.662a36.484 36.484 0 0 1-3.303-2.459l59.564-25.296a36.852 36.852 0 0 1-.544 4.093zm-67.155 11.63 66.521-28.25c.316 1.16.574 2.34.774 3.533l-65.309 27.736a36.41 36.41 0 0 1-1.986-3.018zm-2.61-5.404 34.466-14.638.012-.005 32.602-13.845a36.113 36.113 0 0 1 1.326 3.298l-66.97 28.442a36.16 36.16 0 0 1-1.436-3.252zm.286-27.036 16.984 17.05-4.15 1.762a1.214 1.214 0 0 0-.323-.588l-13.941-13.994c.39-1.44.867-2.853 1.43-4.23zm3.124-5.982 20.244 20.32-3.966 1.684-18.33-18.4a36.282 36.282 0 0 1 2.052-3.604zm7.476 25.742-3.965 1.684-9.299-9.334c.08-1.715.28-3.408.592-5.07zm-13.267-4.199 6.884 6.91-5.3 2.251a36.692 36.692 0 0 1-1.584-9.16zm11.504-28.382a36.775 36.775 0 0 1 4.192-3.401v7.61zm6.626-4.954a35.903 35.903 0 0 1 4.055-2.034v17.71l-4.055-4.071zm6.489-2.964a35.975 35.975 0 0 1 4.055-1.125v26.278l-4.055-4.07zm6.488-1.563a36.665 36.665 0 0 1 4.056-.357v33.12l-.327.14-3.729-3.744v-29.159zm30.011 21.74-4.055 1.723v-17.383a36.739 36.739 0 0 1 4.055 3.273zm-6.488 2.756-4.056 1.723v-23.726c1.391.59 2.745 1.27 4.056 2.034zm-6.49 2.756-4.055 1.722v-28.535c1.377.296 2.73.671 4.056 1.124zm-6.488 2.756-4.056 1.722v-32.087c1.367.045 2.72.164 4.056.357zm26.711-11.344-4.811 2.043v-8.868a36.567 36.567 0 0 1 4.811 6.825zm-58.58-7.456 22.635 22.721-3.965 1.684-21.246-21.327a36.935 36.935 0 0 1 2.576-3.078zm-1.242 48.54 64.08-27.213c.117 1.203.177 2.419.177 3.643l-.002.144-61.673 26.192c-.166-.16-.332-.321-.496-.485a37.253 37.253 0 0 1-2.086-2.28z"/><text font-family="Helvetica" xml:space="preserve" text-anchor="middle" text-rendering="geometricPrecision" transform="matrix(.7694 0 0 1 86.303 52.164)" font-size="32" x="220.051" y="85" fill="currentColor" stroke="null">{{ .PodName }}</text><text xml:space="preserve" font-family="'Noto Sans JP'" stroke-width="0" fill="currentColor" stroke="null" font-size="22" y="167.245" x="198.372">a Yarn.social pod</text></g></svg>`

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
	// to control when to actually fetch and update feeds. This accepts `time.Duration`
	// as parsed by `time.ParseDuration()`.
	DefaultFetchInterval = "5m"

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
	DefaultMaxFetchLimit = 1 << 21 // ~2MB (or more than enough for a year)

	// DefaultAPISessionTime is the server's default session time for API tokens
	DefaultAPISessionTime = 240 * time.Hour // 10 days

	// DefaultAPISigningKey is the default API JWT signing key for tokens
	DefaultAPISigningKey = InvalidConfigValue
)

var (
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
)

func NewConfig() *Config {
	return &Config{
		Version: version,
		Debug:   DefaultDebug,

		Name:              DefaultName,
		Logo:              DefaultLogo,
		Description:       DefaultMetaDescription,
		Store:             DefaultStore,
		Theme:             DefaultTheme,
		BaseURL:           DefaultBaseURL,
		AdminUser:         DefaultAdminUser,
		FeedSources:       DefaultFeedSources,
		RegisterMessage:   DefaultRegisterMessage,
		CookieSecret:      DefaultCookieSecret,
		TwtPrompts:        DefaultTwtPrompts,
		TwtsPerPage:       DefaultTwtsPerPage,
		MaxTwtLength:      DefaultMaxTwtLength,
		OpenProfiles:      DefaultOpenProfiles,
		OpenRegistrations: DefaultOpenRegistrations,
		DisableGzip:       DefaultDisableGzip,
		DisableLogger:     DefaultDisableLogger,
		DisableFfmpeg:     DefaultDisableFfmpeg,
		Features:          NewFeatureFlags(),
		SessionExpiry:     DefaultSessionExpiry,
		MagicLinkSecret:   DefaultMagicLinkSecret,
		SMTPHost:          DefaultSMTPHost,
		SMTPPort:          DefaultSMTPPort,
		SMTPUser:          DefaultSMTPUser,
		SMTPPass:          DefaultSMTPPass,
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
		d, err := time.ParseDuration(fetchInterval)
		if err != nil {
			log.WithError(err).Errorf("error parsing fetch interval %s", fetchInterval)
			return err
		}
		cfg.FetchInterval = d
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
