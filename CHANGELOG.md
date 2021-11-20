
<a name="0.8.0"></a>
## [0.8.0](https://git.mills.io/yarnsocial/yarn/compare/0.7.4...0.8.0) (2021-11-21)

### Bug Fixes

* @prologic: Fix loading pod settings with invalid features (Fixes #556)
* @prologic: Fix IsAdmin contexxt variable
* @prologic: Fix a potential panic in NewContext()
* @prologic: Fix bug with --disable-ffmpeg that also disable incorrectly uploading of photos/pictures
* @prologic: Fix fd socket leak in webmention handling
* @prologic: Fix the margin of the Yarn count badge
* @prologic: Fix a few bugs with twt context rendering (aparently you can't nest a tags :O)
* @james: Fix twt context to use proper Markdown rendering + HTML sanitizer (#532)
* @prologic: Fix css for show_twt_context so ellipsis works
* @prologic: Fix display of Root conv button for unauthenticated users

### Features

* @prologic: Add graceful AJAX support for Follow/UnFollow in Following and Followers views (Fixes #547)
* @prologic: Add support for Service Workers to be hosted at /js/ (/static/js/ in themes)
* @prologic: Add MagicLinkAuth, StripTwtSubjectHashes and ShowTwtContext as promoted features
* @prologic: Add graceful AJAX for Follow/UnFollow and Mute/UnMute for externalProfile template too
* @james: Add a bunch of missed defer f.Close() -- none of which I believe could lea file descriptors though from open files (#531)
* @fastidious: Add a class to the twt navigation (#528)
* @prologic: Add getRootTwt template function
* @prologic: Add experimental Twt Context feature enabled with show_twt_context
* @prologic: Add support for displaying list of features with ./yarnd --enable-feature list

### Updates

* @prologic: Update CHANGELOG for 0.8.0
* @prologic: Update CHANGELOG template and config
* @fastidious: Update 'internal/theme/static/css/99-yarn.css' (#534)
* @fastidious: Update 'internal/theme/static/css/99-yarn.css' (#533)
* @prologic: Update default pod logo as per abb9c01


<a name="0.7.4"></a>
## [0.7.4](https://git.mills.io/yarnsocial/yarn/compare/0.7.3...0.7.4) (2021-11-15)

### Bug Fixes

* @prologic: Fix dupe Twt bug in User views by returning a copy of the view's slice
* @prologic: Fix broken logic for archived root twts

### Updates

* @prologic: Update CHANGELOG for 0.7.4


<a name="0.7.3"></a>
## [0.7.3](https://git.mills.io/yarnsocial/yarn/compare/0.7.2...0.7.3) (2021-11-15)

### Bug Fixes

* @prologic: Fix cache bug causing dupe Root Twts in conversation views (Caching is hard 😅)
* @prologic: Fix conversation forking
* @prologic: Fix twt-hash link to lower-right of Twt cards to always be linked
* @prologic: Fix bug disabling features in the Manage Pod UI
* @prologic: Fix CI
* @prologic: Fix UI/UX behaviour of the Bookmark button

### Features

* @prologic: Add experimental StripConvSubjectHashes (strip_conv_subject_hashes) feature
* @prologic: Add (re-add) MergeStore daily job
* @prologic: Add conversation length badges and fix Edit/Delete/Reply/Conv buttons (can't use ids here)

### Updates

* @prologic: Update CHANGELOG for 0.7.3
* @ullarah: Update 'internal/theme/templates/settings.html' (#521)
* @prologic: Update tools/check-versions.sh with new pod: txt.quisquiliae.com


<a name="0.7.2"></a>
## [0.7.2](https://git.mills.io/yarnsocial/yarn/compare/0.7.1...0.7.2) (2021-11-13)

### Bug Fixes

* @prologic: Fix Docker image to just work without any arguments and drop chown of default data volume

### Updates

* @prologic: Update CHANGELOG for 0.7.2


<a name="0.7.1"></a>
## [0.7.1](https://git.mills.io/yarnsocial/yarn/compare/0.7.0...0.7.1) (2021-11-13)

### Bug Fixes

* @prologic: Fix signal handling using su-exec and ownership of default data volume /data
* @prologic: Fix example Docker Compose for Docker Swarm to match Docker image entrypoint chagne
* @lyse: Fix password reset token by properly checking counters (#519)
* @prologic: Fix password reset token by not decrementing it twice in the token cache
* @prologic: Fix expiryAt timestamp for magic_link auth and password reset tokens
* @prologic: Fix example Docker Compose for Docker Swarm

### Features

* @prologic: Add an improved Docker image that supports PUID/PGID env vars to run yarnd as different users (e.g: to support Synolgy)

### Updates

* @prologic: Update CHANGELOG for 0.7.1


<a name="0.7.0"></a>
## [0.7.0](https://git.mills.io/yarnsocial/yarn/compare/0.6.2...0.7.0) (2021-11-13)

### Bug Fixes

* @prologic: Fix bug in setting tokens for passwrod reset and magiclink auth
* @prologic: Fix minor bug in TTLCache.get()
* @prologic: Fix cached tokens to be deleted after use
* @prologic: Fix token expiry check on password reset form
* @prologic: Fix password reset and magiclink auth tokens to only be usable once with a default TTL of 30m
* @prologic: Fix edit/reply/fork buttons ot use ID selectors
* @prologic: Fix /custom static route for prod builds
* @prologic: Fix incorrect cache header used in conversation view
* @prologic: Fix FeatureFlags' JSON marshalling/unmarshalling
* @prologic: Fix Cached.UpdateFeed() to not overwrite a cached feed with an empty set of twts
* @prologic: Fix misspelled translation key for ErrTokenExpired
* @prologic: Fix autocomplete for Login via EMail's username field
* @prologic: Fix potential nil pointer bug in PermalinkHandler()
* @prologic: Fix bug that logs pod owner out when deleting users
* @prologic: Fix the session cookie name once and for all 🥳
* @prologic: Fix Pod Base URL automatically if it's missing a Scheme, log a warning and assume https://
* @prologic: Fix caching bug with editing and deleting last twt
* @prologic: Fix default MediaResolution to match default themes page width of 850px (Fixes #508)
* @prologic: Fix Cache.FetchTwts() to Normalize Feed.URL(s) before doing anything
* @prologic: Fix Session Cookie name to by config.LocalURL().Hostname() so the Pod name can be more free form
* @prologic: Fix NormalizeURL() to strip URL Fragments from Feed URIs
* @prologic: Fix short time ago formats (Fixes #509)
* @prologic: Fix title entries of Atom/XML feeds

### Features

* @prologic: Add improved UX for Follow/Unfollow and Mute/Unmute with graceful JS fallback
* @prologic: Add support for Pod-level (User overridable) Timezone, Time and external link preferences
* @prologic: Add support for /custom/*filepath arbitrary static file serving from a theme with any directory/file structure
* @prologic: Add missing lang msg for MsgMagicLinkAuthEmailSent
* @prologic: Add support for Login via Email with feature magic_link_auth
* @prologic: Add isFeatureEnabled() func to templates
* @prologic: Add support for custom pages (Fixes #393)
* @prologic: Add s simple JS confirm to logout to avoid accidental logout
* @prologic: Add a PruneUsers job that runs once per week with an email list of candidate users to delete for the Pod Owner based on some heuristics
* @prologic: Add new pod yarn.meff.me to manually track versions to help keep the ecosystem up-to-date as we grow :D
* @prologic: Add basic validation of the Pod's Base URL to ensure it is TLS enabled in non-debug (production) mode
* @movq: Add pagination extension spec (#494)

### Updates

* @prologic: Update CHANGELOG for 0.7.0
* @prologic: Update deps
* @prologic: Update contributing guidelines


<a name="0.6.2"></a>
## [0.6.2](https://git.mills.io/yarnsocial/yarn/compare/0.6.1...0.6.2) (2021-11-08)

### Bug Fixes

* @prologic: Fix consistency of pointers on toolbar buttons (Fixes #505)
* @prologic: Fix expression in User Settings template for which openLinksInPreference is checked 🤦‍♂️
* @prologic: Fix typo in User Settings handler for persisting OpenLinksInPreference 🤦‍♂️

### Features

* @james: Add RotateFeeds job (#504)
* @fastidious: Add article target (#506)
* @prologic: Add per-handler latency metrics

### Updates

* @prologic: Update CHANGELOG for 0.6.2


<a name="0.6.1"></a>
## [0.6.1](https://git.mills.io/yarnsocial/yarn/compare/0.6.0...0.6.1) (2021-11-06)

### Bug Fixes

* @prologic: Fix a nil pointer exception handling User OpenLinksInPreference :/

### Updates

* @prologic: Update CHANGELOG for 0.6.1


<a name="0.6.0"></a>
## [0.6.0](https://git.mills.io/yarnsocial/yarn/compare/0.5.0...0.6.0) (2021-11-06)

### Bug Fixes

* @prologic: Fix and improve the yarnc timeline to support Markdown rendering and fix multi-line twts
* @prologic: Fix missing parser.HardLineBreak extension
* @prologic: Fix consistency of Pod URL for the yarnc command-line client to match the mobile app just taking a Pod URL
* @lyse: Fix broken tests (#500)
* @prologic: Fix typo in recovery email template
* @prologic: Fix bug persisting BlacklistedFeeds in Pod override Settings (ooops)
* @prologic: Fix validation/initialization bugs with pod override settings and handling of whitelisted_images and blacklisted_feeds
* @prologic: Fix data races with feature flags
* @prologic: Fix marshaling/unmarshaling of feature flags
* @prologic: Fix some weird Markdown parsing and HTML formatting behaviour by turning off some extensions/options
* @prologic: Fix bug with serializing FeatureFlags causing pod settings overrides not to load
* @prologic: Fix typo (case) in managePod template
* @prologic: Fix a panic bug in Cache v7 :/
* @prologic: Fix preflight check for Go1.17
* @prologic: Fix performance of computing timeline view updated_at timestamps
* @prologic: Fix updating UpdateAt timestamps across all views
* @prologic: Fix the builtin theme to use title instead of data-tooltip
* @prologic: Fix Markdown html rendering and re-enable CommonFlags
* @prologic: Fix performance regression posting as user that e9222f5 broke
* @lyse: Fix metadata parsing in comments (#484)
* @james: Fix TwtxtHanadler() to support If-Modified-Since and Range requests (#490)

### Features

* @prologic: Add a Open links in to user settings page
* @prologic: Add a user setting OpenLinksInPreference to control behaviour of opening external links
* @prologic: Add highlights to mentions for the yarnc command-line client's timeline sub-command
* @prologic: Add vim support to yarnc post to use an editor to write the twt
* @prologic: Add parser.NoEmptyLineBeforeBlock to Markdown parsing extensions
* @prologic: Add support for user preferred time format (12h/24h)
* @prologic: Add support for blacklisting feeds or entire pods, domains, etc
* @prologic: Add background tasks for Post handlers and External feeds to decrease latency
* @prologic: Add custom user views that take into account user muted feeds
* @prologic: Add a --disable-logger flag for those that already do access logs at the ingress level
* @prologic: Add a local cache view
* @prologic: Add a new view for Discover
* @prologic: Add a hacky pre-rendering hook and move timeline tiemstamp updates there for hopefully better perf
* @prologic: Add a --max-cache-fetchers configuration option to control the number of fetchers the feed cache uses (defaults to number of cpus)
* @prologic: Add a custom version of humanize.Time with shorter output
* @prologic: Add .Profile.LastPostedAt to be used by template authors
* @prologic: Add support for displaying last updated times for the main three timeline views and proivde as template context (updated_at)
* @prologic: Add support for displaying last updated times for the main three timeline views and proivde as template context
* @prologic: Add support for displaying external feed's following feeds
* @prologic: Add immediate on-pod feed/persona updates
* @james: Add retryableStore to automatically recover from database failure in some cases (#474)
* @prologic: Add discover_all_posts and shorter_permalink_title as permanent feature changes
* @prologic: Add fork button to search results
* @james: Add a much impmroved SearchHandler() and new search template with similar UX to Conversation/Yarn view (#493)

### Updates

* @prologic: Update CHANGELOG for 0.6.0


<a name="0.5.0"></a>
## [0.5.0](https://git.mills.io/yarnsocial/yarn/compare/0.4.1...0.5.0) (2021-10-29)

### Bug Fixes

* @prologic: Fix unnecessary warning on non-existent settings.yaml on startup (Fixes #435)
* @prologic: Fix italics formatting toolbar butotn to use _foo_ instead of *foo*
* @prologic: Fix the name of the default database, app specific JS and CSS files
* @prologic: Fix loading custom templates and static assets with -t/--theme
* @prologic: Fix mising AvatarHash for the API SettingsEndpoint()
* @prologic: Fix missing translation on Conversation View (Yarn) title

### Features

* @prologic: Add Feeds to Profile for the Mobile App (only for the user viewing their own profile)
* @james: Add support for ignoring twts in the future until they become relevant (#491)
* @prologic: Add tw.lohn.in to tools/check-versions.sh
* @prologic: Add automatic rename of old store path for pod owners that use defaults to smooth over change in 034009e
* @prologic: Add first-class support for themes to more easily customize templates and static assets
* @prologic: Add support for displaying bookmarks in a timeline view instead of just a list of hashes
* @prologic: Add two additional lines of preamble text promoting Yarn.social (shameful plug)
* @prologic: Add support for also disabling media in general (--disable-media)
* @prologic: Add -m 5 (5s timeout) to tools/check-pod-versions.sh

### Updates

* @prologic: Update CHANGELOG for 0.5.0


<a name="0.4.1"></a>
## [0.4.1](https://git.mills.io/yarnsocial/yarn/compare/0.4.0...0.4.1) (2021-10-25)

### Updates

* @prologic: Update CHANGELOG for 0.4.1
* @prologic: Update the default favicon
* @prologic: Update release script to also push the git commit


<a name="0.4.0"></a>
## [0.4.0](https://git.mills.io/yarnsocial/yarn/compare/0.3.0...0.4.0) (2021-10-24)

### Bug Fixes

* @prologic: Fix abbr of URL
* @fastidious: Fix padding on code blocks (#476)

### Features

* @prologic: Add support for auto-refreshing avatars using a cache-busting technique
* @prologic: Add a fragment, a blake2b base32 encoded hash of user and feed avatars to avatar metddata
* @prologic: Add list of available features if invalid feature supplied to --enable-feature

### Updates

* @prologic: Update CHANGELOG for 0.4.0


<a name="0.3.0"></a>
## [0.3.0](https://git.mills.io/yarnsocial/yarn/compare/0.2.0...0.3.0) (2021-10-24)

### Bug Fixes

* @prologic: Fix nav layout and remove unncessary container-fluid class
* @fastidious: Fix excessive padding, minor CSS tweaks and use a centered layout (#473)
* @prologic: Fix bug with User.FollowAndValidate() to use heuristics to set an alias for the feed if none given
* @prologic: Fix typo in ad-hoc check-versions.sh tool
* @prologic: Fix performance regression of loading timeline
* @prologic: Fix CI
* @fastidious: Fix the Web interface UI/UX to be a bit more compact and cleaner (#468)
* @prologic: Fix CI
* @prologic: Fix badge color to match primary color
* @prologic: Fix video poster generation for short (<3s) videos using thumbnail filter
* @prologic: Fix User.Fork() behaviour
* @prologic: Fix an edge case with the lextwt feed parser with bad url/avatar metadata missing the scheme
* @prologic: Fix bug where external avatars were not generated for some misbehaving feeds
* @prologic: Fix the rendering of the Twter in permalink titles
* @prologic: Fix overriding alrady cached/discovered Avatars from feed advertised Avatar in feed preamble metadata
* @prologic: Fix Reply hints to just the Twter
* @prologic: Fix and remove unnecessary footer on every Markdown page
* @prologic: Fix tests
* @prologic: Fix external feed avatar handling so avatars are always cached and served from the pod
* @prologic: Fix up the rendering of the default pages
* @prologic: Fix the yarnc stats sub-command to display the right information on feeds
* @prologic: Fix User.Filter() to return an empty slice if all twts are filtered
* @prologic: Fix handling for external avatars so the local pod has a cached copy and services all external avatars (regression)
* @prologic: Fix external profile view to correctly point to the external profile view for the header feed  name and cleanup some of hte code
* @prologic: Fix consistency of profileLinks partial and drop use of TwtURL from Profile model
* @prologic: Fix internal links to open in same page

### Documentation

* @prologic: Document todo for removing fragment on mentions from the API as Goryon may not need it now

### Features

* @prologic: Add arrakis.netbros.com to ad-hoc tools/check-versions.sh
* @james: Add Follow to Settings page and fix template reloading in debug mode (#454)
* @prologic: Add conversation length badges
* @prologic: Add shell script to check versions of known pods
* @prologic: Add support for templating the logo with the pod name and update the default yarnd logo
* @lyse: Add test case for weird bug with adi's twt (#463)

### Updates

* @prologic: Update CHANGELOG for 0.3.0
* @fastidious: Update 'internal/static/css/01-pico.css'
* @fastidious: Update 'internal/static/css/01-pico.css'
* @fastidious: Update 'internal/static/css/01-pico.css'
* @fastidious: Update 'internal/static/css/99-twtxt.css'
* @fastidious: Update 'internal/static/css/01-pico.css'
* @fastidious: Update 'internal/langs/active.en.toml'
* @fastidious: Update 'internal/templates/base.html'
* @fastidious: Update 'internal/static/css/01-pico.css'
* @fastidious: Update 'internal/static/css/99-twtxt.css'
* @fastidious: Update 'internal/static/css/99-twtxt.css'
* @prologic: Update deps
* @prologic: Update some of the default options
* @eldersnake: Update 'internal/templates/page.html'
* @eldersnake: Update 'internal/langs/active.en.toml'


<a name="0.2.0"></a>
## [0.2.0](https://git.mills.io/yarnsocial/yarn/compare/0.1.0...0.2.0) (2021-10-17)

### Bug Fixes

* @prologic: Fix GoReleaser config
* @prologic: Fix bug in profileLinks partial
* @prologic: Fix external profile view to not display following/followers counts if zero
* @prologic: Fix NFollowing count based on no. of # follow = found
* @prologic: Fix Conversation view to hide Root button when the conversation has no more roots
* @prologic: Fix display of Fork button
* @prologic: Fix typo
* @prologic: Fix layout of local feeds
* @prologic: Fix Feeds view when uneven no. of feeds
* @prologic: Fix Feeds view
* @prologic: Fix behaviour of when Fork/Conversation buttons are displayed contextually and cleanup UX
* @prologic: Fix behaviour of prompt for when the Timeline is empty
* @lyse: Fix Python twt hash reference implementation (#459)
* @prologic: Fix serving videos behind Cloudflare for Safari and Mobile Safari (redo of 35241bc) by adding a --disable-gzip flag
* @prologic: Fix broken test
* @prologic: Fix mentions lookup and expansion based on the new (by default) nick@domain format with fallback to local user and custom aliases
* @prologic: Fix shortel_permalink_title maxPermalinkTitle to 144
* @prologic: Fix feature for shorter_permalink_title
* @prologic: Fix serving videos for Mobile Safari / Safari through Cloudflare by disabling Gzip compression :/
* @prologic: Fix related projects
* @prologic: Fix up the README's links and branding
* @prologic: Fix follow handlers and data models to cope with following multiple feeds with the same nick or alias
* @prologic: Fix typo in Conversations sub-heading view
* @james: Fix showing Register button when registrations are disabled (#452)
* @james: Fix inconsistent button colors using only contrast class sparingly and only for irréversible actions like deleting your account (#453)
* @prologic: Fix tests
* @prologic: Fix duplicate twtst in FilterOutFeedsAndBotsFactory
* @prologic: Fix missing view context initialization missed in two other templates
* @prologic: Fix templtes view context var that wasn't initialized
* @prologic: Fix logic of handling local user/feeds vs. external feeds in /api/v1/fetch-twts endpoint
* @prologic: Fix fetching twts from external profiles
* @prologic: Fix order of twts in conversations (reverse order)
* @prologic: Fix Twts.Swap() implementation
* @prologic: Fix sorting of twts to be consistent (sorted by created, then hash)
* @prologic: Fix typo in error handling for updating white list domains
* @prologic: Fix preflight to handle Go 1.17
* @prologic: Fix loading settings and re-applying WhitelistedDomains on startup
* @prologic: Fix lextwt to cope with bad url in metadata
* @prologic: Fix FeedLookup case sensitivity bug
* @prologic: Fix CI
* @prologic: Fix preflight to account for null GOPATH and GOBIN
* @prologic: Fix performance of processing WebMentions by using a TaskFunc
* @prologic: Fix Makefile
* @prologic: Fix typo in +RegisterFormEmailSummary
* @prologic: Fix error handling in reset user
* @prologic: Fix image name for Docker Hub
* @prologic: Fix Drone CI config to build docker images in paralell and fix installation of deps
* @prologic: Fix Drone CI for building and pushing Docker images using plugins/kaniko plugin
* @james: Fix writeBtn (blog post editor) behavior (#433)
* @prologic: Fix version string of server binary
* @prologic: Fix Drone CI config and add make deps back
* @prologic: Fix Drone CI and Docker image tags and versioning
* @prologic: Fix Drone CI config
* @prologic: Fix server version strings
* @prologic: Fix typo
* @prologic: Fix Dockerfile.dev with missing langs soruces
* @prologic: Fix some more dependneices of things that moved to git.mills.io
* @prologic: Fix docker-compose
* @prologic: Fix .gitignore
* @prologic: Fix stale issue workflow
* @prologic: Fix stale issues workflow
* @prologic: Fix change in case of translation files
* @prologic: Fix User-Agent detection by tightening up the regex used (#418)
* @prologic: Fix lang translations to use embed and io/fs to load translations
* @prologic: Fix whitespace
* @prologic: Fix display of multi-line private messages
* @prologic: Fix Emails removeal migration and remove it from the templates
* @prologic: Fix tests and refactor invalid feed error (#412)
* @prologic: Fix bug in URLForConvFactory to also check for OP Hash validity in the archive too
* @prologic: Fix bug in lastTwt.Created() call -- missed in refactor from concrete types to interfaces
* @prologic: Fix formatTwtText template factory function
* @prologic: Fix User-Agent matching for iPhone OS 14.x that can't handle WebP correctly (even though it claims it can)
* @prologic: Fix typo around custom pod Settings struct with missing quote for pod_logo
* @prologic: Fix custom pod logo support (#358)
* @github: Fix public following disclosure (#379)
* @prologic: Fix an off-by-one error in types.SplitTwts() (#377)
* @prologic: Fix Uhuntu build (#376)
* @prologic: Fix option for -P/--parser
* @prologic: Fix fallback to config theme when user has no theme set
* @prologic: Fix display of published time of blog posts (#366)
* @prologic: Fix User Profile URL routes (#360)
* @prologic: Fix typo
* @prologic: Fix typo
* @github: Fix typos (#353)
* @prologic: Fix a bug for iOS 14.3 Mobile Safari not rendering WebP correctly (work-around)
* @prologic: Fix missing closing a (Fixes #335)
* @prologic: Fix working simplified docker-compose.yml to get up and running quickly
* @prologic: Fix a JS bug clikcing on the Write button in the toolbar to start writing a Twt Blog Post
* @prologic: Fix internal events from non-Public IPs (#332)
* @prologic: Fix Referer redirect behaviour (#331)
* @prologic: Fix deleting last twt (missing csrf token)
* @prologic: Fix goreleaser configs
* @prologic: Fix bug in UnparseTwtFactory() causing erroneous d to be appended to domains
* @github: Fix footer layout regression (#322)
* @prologic: Fix messaging for when feeds may possibly exceed conf.MaxFetchLimit
* @prologic: Fix the way cache_limited counter/log works
* @prologic: Fix CI
* @github: Fix HTML validation errors (#321)
* @vaniot: Fix text overflow p element
* @prologic: Fix a JS bug on load
* @github: Fix tag expansion (#314)
* @vaniot: Fix user able visit login and register page after login (#313)
* @prologic: Fix edge case with Subject parsing
* @prologic: Fix developer experience so UI/UX developers can modify templates and static assets without reloading or rebuilding
* @markwylde: Fix header wrapping on domain (#306)
* @prologic: Fix action URL for managing pods
* @prologic: Fix Docker image build
* @prologic: Fix Docker image push
* @prologic: Fix Docker Image badges (don't yet have a jointwt Docke rHub org)
* @prologic: Fix Docker image push
* @prologic: Fix conversations for Twt Blog Posts (#287)
* @prologic: Fix blog post validation
* @prologic: Fix bad data with missing Followers
* @prologic: Fix spelling errors on register form
* @prologic: Fix bitcask dep
* @prologic: Fix a data race with session handling
* @prologic: Fix refreshing user's own feed and re-warming cache on a user deleting their last Twt (#272)
* @prologic: Fix blog posting with empty titles (#271)
* @prologic: Fix image Makefile target that 15f25c1 broke and add PUBLISH=1 variable so we publish new images again
* @prologic: Fix order of twts displayed in twt timeline (reverse order)
* @prologic: Fix data privacy by removing all user email addresses and never storing emails (#269)
* @prologic: Fix image target to not push the image built immediately (CI does this)
* @prologic: Fix LICENSE file
* @prologic: Fix image resizing for media (avatars was already working)
* @prologic: Fix bug with AddFollower()
* @andriputra.job: Fix timeline style on mobile devices with very long feed names (#248)
* @prologic: Fix userAgentRegex bug with greedy matching
* @prologic: Fix UA detection and relax regex even more
* @prologic: Fix logic in archiver makePath
* @prologic: Fix bug in archiver to prevent use of invalid twt hashes
* @prologic: Fix concurrent map read/write bug with feed cache
* @prologic: Fix Follows/FollowedBy/Muted for external profiles
* @prologic: Fix followers view for external profiles
* @prologic: Fix bad copy/paste
* @prologic: Fix Docker build
* @prologic: Fix title of twt permalinks
* @prologic: Fix generate Make target to run rice to embed the assets __after__ minification/bundling
* @prologic: Fix size of RSS Icon on External Profile view
* @prologic: Fix avatar for external profiles when no avatar found
* @prologic: Fix size of icss-rss icon for externals feeds without an avatar
* @prologic: Fix color of multimedia upload buttons
* @prologic: Fix bug in /settings view
* @prologic: Fix more UI/UX things on the Web App with better CSS
* @prologic: Fix typo
* @prologic: Fix spacing around icons for Post/Publish button (#233)
* @prologic: Fix potential session cookie bug where Path was not explicitly set
* @prologic: Fix name of details settings tab for Pod Management
* @prologic: Fix name of details settings tab for Pod Management
* @prologic: Fix /report view to work anonymously without a user account
* @prologic: Fix conversation bug caused by user filter (mute user feature)
* @prologic: Fix a bug with User.Filter()
* @prologic: Fix wording in Mute / Report User text
* @prologic: Fix grammar on /register view
* @prologic: Fix link to /abuse page
* @prologic: Fix JS error
* @prologic: Fix og:description meta tag
* @prologic: Fix a few bugs with OpenGraph tag generation
* @prologic: Fix URLForExternalProfile links and typo (#224)
* @prologic: Fix subject bug causing conversations to fork (#217)
* @prologic: Fix JS bug with persisting title/text fields
* @prologic: Fix API Signing Key (#212)
* @prologic: Fix video upload quality by disabling rescaling (#205)
* @dooven: Fix structtag for config (#203)
* @prologic: Fix old missing twts missing from the archive from local users and feeds
* @prologic: Fix older Twt Blog posts whoose Twts has been archived (and missing? bug?)
* @prologic: Fix MediaHandler to return a Media Not Found for non-existent media rather than a 500 Internal Server Error
* @prologic: Fix build (#198)
* @prologic: Fix responsive video size on mobile (#195)
* @prologic: Fix video aspect ratio and scaling (#191)
* @prologic: Fix Range Requests for /media/* (#190)
* @prologic: Fix video display to use block style inside paragraphs (#189)
* @prologic: Fix missing names to feeds
* @prologic: Fix links to external feeds
* @prologic: Fix ExpandTags() function so links to Github issue comments work and add unit tess (#176)
* @prologic: Fix PublishBlogHandler(o and fix line endings so rendering happens correctly
* @prologic: Fix incorrect locking used in GetByPrefix()
* @prologic: Fix blogs cache bug
* @prologic: Fix logic for when to attempt to send webmentions for mentions
* @prologic: Fix duplicate @mentions in reply (Fixes #167)
* @prologic: Fix concurrency bug in cache.GetByPrefix()
* @prologic: Fix content-type and set charset=utf-8
* @prologic: Fix wrapping behaviour and remove warp=hard
* @prologic: Fix blogs cache to be concurrenct safe too
* @prologic: Fix global feed cache concurrency bug
* @prologic: Fix all invalid uses of footer inside hgroup and make all hgroups consistently use h2/h3
* @prologic: Fix regex patterns for valid username and feed names and mention syntax
* @prologic: Fix and restore full subjet in twt replies (#163)
* @prologic: Fix the pager (properly)
* @prologic: Fix bug with pager (temporary fix)
* @prologic: Fix nil pointer in map assignment bug
* @prologic: Fix cli build nad refactor username/password prompt handling
* @prologic: Fix the CSS/JS bundled assets with new minify tool (with bug fixes)
* @prologic: Fix bug in /settings with incorrect call to .Profile()
* @prologic: Fix date/time shown on blog posts (remove time)
* @prologic: Fix metric naming consistency feed_sources -> cache_sources (#155)
* @prologic: Fix duplicate tags and mentions (#154)
* @prologic: Fix content-negotiation for image/webp
* @prologic: Fix ExpandMentions()
* @prologic: Fix dns issues in container and force Go to use cgo resolver
* @prologic: Fix CI
* @prologic: Fix feed cache bug not storing Last-Modified and thereby not respecting caching
* @prologic: Fix inconsistency in Syndication and Profile views accessing feed fiels directly instead of by the cache
* @prologic: Fix CI
* @prologic: Fix Dockerfile with missing webmention sources
* @prologic: Fix webmention handling to be more robust and error proof
* @prologic: Fix AvatarHandler that was incorrectly encoding the auto-generated as image/webp when image/png was asked for
* @prologic: Fix bug in /mentions and dedupe twts displayed
* @prologic: Fix bug causing permalink(s) on fresh new twts to 404 when linked to but not in the local pod's cache
* @prologic: Fix /mentions view logic (ooops)
* @prologic: Fix bug in register user on fresh pods with no directory structure in place yet
* @prologic: Fix typo
* @prologic: Fix bug
* @prologic: Fix ordering of twts post cache ttl and archival
* @prologic: Fix an index out of bounds bugs
* @prologic: Fix the logic for Max Cache Items (whoops)
* @prologic: Fix bug in ParseFile() which _may_ cause all local and user twts to be considered old
* @prologic: Fix UX perf of posting and perform async webmentions instead
* @prologic: Fix bugs with followers/folowing view post external feed integration
* @prologic: Fix session leadkage by not calling SetSession() on newly created sessions (only when we actually store session data)
* @prologic: Fix leaking sessions and clean them up on startup and only persist sessions to store for logged in users with an account
* @prologic: Fix external avatar, don't set a Twter.Avatar is none was foudn
* @prologic: Fix build
* @prologic: Fix bug with caching Twter.URL for external feeds
* @prologic: Fix isLocal check in proile template
* @prologic: Fix more bus
* @prologic: Fix Twtter caching in feed cache cycles
* @prologic: Fix Twtxt URL of externalProfile view
* @prologic: Fix several bugs to do with external profiles (I'm tired :/)
* @prologic: Fix typo
* @prologic: Fix bug with DownloadImage()
* @prologic: Fix computed external avatar URIs
* @prologic: Fix external avatar negogiation
* @prologic: Fix old user avatars (proper)
* @prologic: Fix profile avatars
* @prologic: Fix profile bug
* @prologic: Fix Dockerfile (again)
* @prologic: Fix Dockerfile adding missing minify tool
* @prologic: Fix Edit on Github page link
* @prologic: Fix bug in #hashtag searching showing duplicates twts in results
* @prologic: Fix the global feed_cache_size count
* @prologic: Fix profile view and make Reply action available on profile views too
* @prologic: Fix typo
* @prologic: Fix bug in theme so null theme defaults to auto
* @prologic: Fix feed_cache_size metric bug (missed counting already-cached items)
* @prologic: Fix bug in profile view re ShowConfig for profileLinks
* @prologic: Fix another memory optimization and remove another cache.GetAll() call
* @prologic: Fix media upload image resize bug
* @prologic: Fix caching bug with /avatar endpoint
* @prologic: Fix similar bug to #124 for editing last twt
* @prologic: Fix bug in feed cache for local twts
* @prologic: Fix bug in feed cache dealing with empty feeds or null urls
* @prologic: Fix media uploads to only resize images if image > UploadOptions.ResizeW
* @prologic: Fix session storage leakage and delete/expunge old sessions
* @prologic: Fix memory allocation bloat on retrieving local twts from cache
* @prologic: Fix the glitchy theme-switcher that causes unwanted flicker effects (by removing it!)
* @hosseinzeinalii: Fix populating textarea when reply button is clicked for more than one time. (#124)
* @prologic: Fix repository name in Docker GHA workflow
* @prologic: Fix database fragmentation and merge on startup
* @prologic: Fix local twts lookup  to be O(n) where n is the no. of source urls (not total twts in cache)
* @prologic: Fix image handling and auto-orient images according to their EXIF orientation tag
* @prologic: Fix panic in PageHandler() -- not safe to reuse the markdown parser, etc

### Documentation

* @github: Document Twt Subject Extension (#309)

### Features

* @prologic: Add support for detecting and disabling ffmpeg support for audio/video with --disable-ffmpeg as an optional configuration flag
* @prologic: Add support for fetching, storing and serving remote external feed description and following/followers counts via the API
* @prologic: Add support for fetching, storing and displaying remote feed description and following/followers counts
* @prologic: Add Root to link back to root conversations in Conversation view
* @prologic: Add Disable Gzip server setting on startup output
* @prologic: Add build-dev-site Drone CI job to build docs (dev.twtxt.net) site
* @lyse: Add Spec for Metadata Extension (#451)
* @prologic: Add shorter_permalink_title feature
* @prologic: Add yarnc --help to README
* @prologic: Add the same nick@domain and then nick_xx behaviour for keeping track of followers too
* @prologic: Add Tools section to Useer Settings and Bookmarklet that can be added to browser bookmark bars
* @prologic: Add some prompts to new users with an empty timeline
* @prologic: Add a human/crawler friendly version of /version
* @prologic: Add a SoftwareConfig struct and /version endpoint to more easily identify pods
* @prologic: Add JSON marshallers/unmarshallers to FeatureFlags
* @prologic: Add FeatureDiscoverALlPosts to the API end /api/v1/discover endpoint too
* @prologic: Add optional feature (--enable-feature discover_posts_all)
* @prologic: Add following and followers counts to default feed preamble metadata
* @james: Add a hacky temporary solution to squelch flip-flopping followers (#450)
* @prologic: Add a ~/:nick/post/ route for blog posts
* @prologic: Add support for Muting/Unmuting external pods (cross-pod)
* @prologic: Add a POSIX Shell script to run preflight checks as part of the default Makefile target
* @prologic: Add handlers for ~/:nick
* @prologic: Add --json flag to yarnc timeline command for seeing and processing the raw JSON from the API
* @prologic: Add WhitelistedDomains support to Manage Pod settings
* @prologic: Add content-negogiation for /twt/:hash handler
* @prologic: Add CI for dev.twtxt.net
* @prologic: Add native support for Gopher (gopher://) feeds
* @prologic: Add support for forking conversations in the Web UI
* @prologic: Add Drone CI step to build and push to prologic/yarnd (Docker Hub)
* @prologic: Add a 2nd docker image publihs step
* @prologic: Add separate docker image publish step
* @prologic: Add Reset User feature to Pod Management interface
* @prologic: Add a blake2b_base32 cli tool
* @prologic: Add i18n to deps and generate active.*.toml
* @prologic: Add missing lang (i18n) files to Docker image builds
* @prologic: Add preamble to top of all twtxt feeds (#384)
* @prologic: Add check for Bad Request in CLI and fix PEBKAC problem of entering a bad Pod API Base URL :D
* @prologic: Add support for bookmarks (#372)
* @prologic: Add support for draft blog posts and fix deletion (#367)
* @d.vladimyr: Add Node.js reference implementation of Twt Hash algorithm (#365)
* @prologic: Add @twtxt FOLLOW events for feeds too
* @dgy: Add information about ffmpeg dependency. Remove repeated FreeBSD section. Fix a formatting error or two
* @prologic: Add missing CI step for tools deps
* @prologic: Add local Drone CI config
* @prologic: Add support for expanding Twtxt mentions and tags in Twt Blog posts
* @prologic: Add support for bookmarklet(s)
* @prologic: Add Content-Length to TwtxtHandler for /user/:user/twtxt.txt URIs so Pods know what the size of feeds are
* @prologic: Add feed_limited counter to measure no. of feeds affected by conf.MaxFetchLimit
* @prologic: Add SameSite session cookie policy and CSRF Verification to prevent Cross-Site-Sciprint (XSS)
* @prologic: Add msgs.Inc() and msgs.Dec() for messages
* @prologic: Add Pod config handlers and API endpoints (#297)
* @prologic: Add rice-embed.go so the go get works
* @prologic: Add net/http/pprof to debug mode
* @prologic: Add improved async media upload endpoint for the API (backwards compatible) (#274)
* @prologic: Add front matter to pages
* @prologic: Add Atom link for Pod's timeline (aka Discover) on the bottom of every page
* @prologic: Add ValidateFeeds job to cleanup bad feeds on twtxt.net
* @prologic: Add timeline command to twt (command-line client)
* @prologic: Add GET support for /api/v1/settings endpoint to retrieve user settings/object (#256)
* @prologic: Add /api/v1/settings API Endpoint for updating user settings (#252)
* @prologic: Add better support for UA on fetches with token callback (#249)
* @prologic: Add Follows/FollowedBy attributes to user profiles
* @prologic: Add footnote parsing for Twt Blogs
* @prologic: Add HardLineBreak and NoEmptyLineBeforeBlock to Markdown parser options
* @prologic: Add protection against brute force login attempts at user accounts (#241)
* @prologic: Add FilterTwts to conversation view and treat /conv URIs a bit like /twt (permalink) ones
* @prologic: Add a RealIP middleware to capture the real ip of clients when we're behind a proxy
* @prologic: Add better page titles to improve SEO
* @prologic: Add /api/v1/conv ConversationEndpoint() to API
* @prologic: Add DEBUG=1 capability to Makefile for debug builds with real-time static (css/js/img) asset modifications
* @prologic: Add missing FilterTwts() calls to API
* @prologic: Add Muted property to Profile objects in ProfileResposne for the API (#235)
* @prologic: Add improved CSS for the timeline view
* @prologic: Add feed validation to Follow requests from Web App and API to avoid invalid feeds (#228)
* @prologic: Add /mute and /unmute API endpoints (#230)
* @prologic: Add /support and /report API endpoints (#231)
* @prologic: Add switch on /register view to encourage new users to read the community guidelines and agree to them (EULA)
* @prologic: Add support for Pod Owners to manage users (add/delete) (#227)
* @prologic: Add a fast-path to User.Filter() when .muted is of length 0
* @prologic: Add support for muting/unmuting intolerable users (#226)
* @prologic: Add text to /register view that links to the /abuse page
* @prologic: Add blank Abuse Policy (to be filled out)
* @prologic: Add OpenGraph Meta tags to Twt Permalinks (#225)
* @prologic: Add support for uploading audio media (#222)
* @dooven: Add MarkdownText field to Twt (#223)
* @prologic: Add Tags and Subject as dynamic fields to Twt.MarshalJSON() for API clients (#219)
* @shahxeb.malik: Add support for managing a pod's configuration as a pod owner (#170)
* @prologic: Add Hash to Twt and Slug to Twter outputs when Marshalled as JSON for the API (#206)
* @prologic: Add archive_dupe counter
* @70777954+gioperazzo: Add support for persisting Title and Text in Twts and Twt Blogs to local storage (#193)
* @prologic: Add support for auto-generating poster thumbnails for uploaded videos
* @prologic: Add playsinline as allowed attr on video elements
* @prologic: Add ffmeg to Dockerfile runtime image  and remove debug loggging (#188)
* @prologic: Add support for older video media (locally) (#187)
* @prologic: Add support for video uploads and hosting (#186)
* @prologic: Add blogs, archived and cache stats and cache_blogs to metrics
* @prologic: Add BlogPost PublishedAt and Modified timestamps
* @prologic: Add support for editing twt blog posts (#171)
* @prologic: Add missing mu.Lock() for loading older twts cache
* @prologic: Add support for newlines in Twts (short-form) by using Unicode LS (\u2028) to encode them (#166)
* @prologic: Add Timeline navbar item and cleanup Navbar
* @prologic: Add support for editing/deleting your last Twt in a Conversation view
* @prologic: Add Ctrl+Enter to Post a new Twt
* @prologic: Add link to Blog on Twts associated with a Twt Blog Post
* @prologic: Add formatDateTime template function to shorten the human display of Twt posted date/time shorter when it wasn't that long ago
* @prologic: Add a border around avatar images
* @prologic: Add support for style attr on some HTML elements as an option for some users
* @prologic: Add integrated support for conversations (#161)
* @prologic: Add a O(1) lookup for a types.Twt by hash in the global feed cache (cahced on demand)
* @prologic: Add /blogs view to display all twt blog posts for an author (#160)
* @aaadonai: Add unit tests for types.Twt.Subject() and fix the regex (#157)
* @prologic: Add unit tests for types.Feed (#156)
* @prologic: Add a note about having to be logged in to comment on blog posts
* @prologic: Add better UI/UX around the blog view with comments in reverse order and some nice headings
* @prologic: Add support for long-form posts (blog posts) with integrated twt(s). (#152)
* @prologic: Add remote mention syntax @user@domain (#151)
* @prologic: Add twtd_server_sessions metric
* @5233036+danielcooperxyz: Add Token model and view for managing API Tokens (#135)
* @prologic: Add vendored version of github.com/prologic/webmention since it has detracted slightly from standard webmention handling
* @prologic: Add a Remember Me to /login view
* @prologic: Add a new SessionStore that is a caching/persistent store
* @prologic: Add MergeStoreJob to merge the store once per day
* @prologic: Add local twts to mentions view in addition to feeds the user follows
* @prologic: Add a DiskArchver that implements a Cache TTL (#138)
* @prologic: Add more candidates to GetExxternalAvatar()  and rearrange preferences
* @prologic: Add a db.Merge() call after startupJobs() to free memory on database compaction possibly after some cleanup jobs
* @prologic: Add a GenerateAvatar() func and use github.com/nullrocks/identicon for identicons
* @prologic: Add loading=lazy attr to all images to lazy load images on supported browsers
* @prologic: Add post form to external profile view
* @prologic: Add improved external feed integration
* @prologic: Add .gitkeep files for data directory layout
* @prologic: Add missing HEAD handler for /externa/:slug/avatar
* @prologic: Add support for WebP image format (#130)
* @prologic: Add tooling to combined and minify all static css/js assets into a single file and request (boosting load times)
* @prologic: Add external avatar to /external view
* @prologic: Add support for fetching, caching and displaying external avatars
* @prologic: Add /robots.txt view
* @prologic: Add post form on profile view and puts profile links in a 2nd column
* @prologic: Add user preferences for theme in /settings view
* @prologic: Add target=_blank to Config/Twtxt/Atom Profile Links
* @prologic: Add link to user config on /settings view
* @prologic: Add server_info metric with version information
* @prologic: Add Grafana Dashbaord
* @prologic: Add /user/:nick/config.yaml handler (Closes #36)
* @prologic: Add debug-only memory profiler to debug some memory bloat
* @prologic: Add a Uploading to the Upload button tooltip when its processing the media
* @prologic: Add a paper-plance icon to the Post button
* @prologic: Add profile links to /settings view to be consistent with the profile view

### Updates

* @prologic: Update CHANGELOG for 0.2.0
* @prologic: Update the footer of the base template to remove James Mills (the original author/creator) and instead link to Yarn.social as the primary branding
* @prologic: Update to Bitcask v0.3.14
* @prologic: Update bitcask to v0.3.13
* @prologic: Update README
* @prologic: Update README a little bit and add Drone CI badge
* @prologic: Update docker image publication
* @prologic: Update deps
* @prologic: Update about.md
* @prologic: Update stale workflow
* @prologic: Update privacy.md
* @prologic: Update icon used for bookmarking Twts (Closes Viglino/iconicss#21)
* @prologic: Update production example docker + traefik deployment
* @prologic: Update README.md
* @prologic: Update README.md
* @prologic: Update deps
* @prologic: Update README
* @prologic: Update external feed source URI(s)
* @prologic: Update Go module path to github.com/jointwt/twtxt and remove committed .min.* and commit rice-embed.go insted
* @prologic: Update links to Related Projects
* @prologic: Update README.md
* @prologic: Update bundled CSS
* @prologic: Update abuse.md
* @prologic: Update about.md
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update Grafana Dashboard
* @prologic: Update README.md
* @gabrielfemi799: Update AUTHORS (#131)
* @prologic: Update Grafana Dashboards and make all panels transparent
* @prologic: Update Grafana Dashbaord with fixed Go Memory panel
* @prologic: Update Grafana Dashbaord
* @prologic: Update README.md
* @prologic: Update README.md


<a name="0.1.0"></a>
## [0.1.0](https://git.mills.io/yarnsocial/yarn/compare/0.0.12...0.1.0) (2020-08-19)

### Bug Fixes

* @prologic: Fix paging on discover and profile views
* @prologic: Fix missing links in about page
* @prologic: Fix Dockerfile with missing new pages
* @prologic: Fix horizontal scroll / overflow on mobile devices
* @prologic: Fix Atom feed and populate Summary with text/html and title with text/plain
* @prologic: Fix UX of hashes and shorten them to 11 (by default) characters which is roughly 88 bits of entropy or basically never likely to collide :D
* @prologic: Fix UX of relative time display and use humanize.Time
* @prologic: Fix /settings to be a 2-column layout since we don't have that many settings
* @prologic: Fix superfluous paragraphs in twt formatting
* @prologic: Fix the email templates to be consistent
* @prologic: Fix the UX of the password reset view
* @prologic: Fix formatting of Support Request email and indent/quote Subject/Message
* @prologic: Fix the workding around password reset emails
* @prologic: Fix Reply-To for support emails
* @prologic: Fix email to send text/plain instead of text/html
* @prologic: Fix wrong template for SendSupportRequestEmail()
* @prologic: Fix Docker GHA workflow
* @prologic: Fix docker image versioning
* @prologic: Fix Docker image
* @prologic: Fix long option name for open registrations
* @prologic: Fix bug in /lookup handler
* @prologic: Fix /lookup to only regturn following and local feeds
* @prologic: Fix /lookup handler behaviour
* @prologic: Fix UI/UX of relative twt post time
* @prologic: Fix UI/UX of date/time of twts
* @prologic: Fix Content-Type on HEAD /twt/:hash
* @prologic: Fix a bunch of IE11+ related JS bugs
* @prologic: Fix Follow/Unfollow actuions on /following view
* @prologic: Fix feed_cache_last_processing_time_seconds unit
* @prologic: Fix bug with /lookup handler and perform case insensitive looksup
* @prologic: Fix and tidy up the /settings view with followers/following now moved to their own views
* @prologic: Fix missing space on /followers
* @prologic: Fix user experience with editing your last Twt and preserve the original timestamp
* @prologic: Fix Atom URL for individual Twts (Fixes #117)
* @prologic: Fix bad name of PNG (typod extension)
* @prologic: Fix hash collisions of twts by including the source twtxt URI as well
* @prologic: Fix and add some missing icons
* @prologic: Fix bug in new permalink handling
* @prologic: Fix other missing uploadoptions

### Features

* @prologic: Add post partial to permalink view for authenticated users so Reply works
* @prologic: Add WebMentions and basic IndieWeb µFormats v2 support (h-card, h-entry) (#122)
* @prologic: Add missing spinner icon
* @prologic: Add tzdata package to runtime docker image
* @prologic: Add user setting to display dates/times in timezone of choice
* @prologic: Add Content-Typre to HEAD /twt/:hash handler
* @prologic: Add HEAD handler for /twt/:hash handler
* @prologic: Add link to twt.social in footer
* @prologic: Add feed_cache_last_processing_time_seconds metric
* @prologic: Add /metrics endpoint for monitoring
* @dooven: Add external feed (#118)
* @prologic: Add link to user's profile from settings
* @prologic: Add Follow/Unfollow actions for the authenticated user on /followers view
* @prologic: Add /following view with defaults for new to true and tidy up followers view
* @prologic: Add Twtxt and Atom links to Profile view
* @prologic: Add a note about off-Github contributions to README
* @prologic: Add PNG version of twtxt.net logo
* @prologic: Add support for configurable img whitelist (#113)
* @prologic: Add permalink support for individual local/external twts (#112)
* @dooven: Add etags for default avatar (#111)
* @prologic: Add text/plain alternate rel link to user profiles
* @prologic: Add docs for Homebrew formulare

### Updates

* @prologic: Update CHANGELOG for 0.1.0
* @prologic: Update CHANGELOG for 0.0.13
* @prologic: Update README.md
* @dooven: Update README gif (#121)
* @prologic: Update /feeds view and simplify the actions and remove own feeds from local feeds as they  apprea in my feeds already
* @prologic: Update the /feeds view with My Feeds and improve some of the wording
* @sstefin: Update README.md (#116)
* @prologic: Update README.md
* @prologic: Update logo
* @prologic: Update README.md


<a name="0.0.12"></a>
## [0.0.12](https://git.mills.io/yarnsocial/yarn/compare/0.0.11...0.0.12) (2020-08-10)

### Bug Fixes

* @prologic: Fix duplicate build ids for goreleaser config
* @prologic: Fix and simplify goreleaser config
* @prologic: Fix avatar upload handler to resize (disproportionally?) to 60x60
* @prologic: Fix config file loading for CLI
* @prologic: Fix install Makefile target
* @prologic: Fix server Makefile target
* @prologic: Fix index out of range bug in API for bad clients that don't pass a Token in Headers
* @prologic: Fix z-index of the top navbar
* @prologic: Fix logic of count of global followers and following for stats feed bot
* @prologic: Fix the style of the media upload button and create placeholde rbuttons for other fomratting
* @prologic: Fix the mediaUpload form entirely by moving it outside the twtForm so it works on IE
* @prologic: Fix bug pollyfilling the mediaUpload input into the uploadMedia form
* @prologic: Fix another bug with IE for the uploadMedia capabilities
* @prologic: Fix script tags inside body
* @prologic: Fix JS compatibility for Internet Explorer (Fixes #96)
* @prologic: Fix bad copy/paste in APIv1 spec docs
* @prologic: Fix error handling for APIv1 /api/v1/follow
* @prologic: Fix the route for the APIv1 /api/v1/discover endpoint
* @prologic: Fix error handling of API's isAuthorized() middleware
* @prologic: Fix updating feed cache on APIv1 /api/v1/post endpoint
* @prologic: Fix typo in /follow endpoint
* @prologic: Fix the alignment if the icnos a bit
* @prologic: Fix bug loading last twt from timeline and discover
* @prologic: Fix delete last tweet behaviour
* @prologic: Fix replies on profile views
* @prologic: Fix techstack document name
* @prologic: Fix Dockerfile image versioning finally
* @prologic: Fix wrong handler called for /mentions
* @prologic: Fix mentions parsing/matching
* @prologic: Fix binary verisoning
* @prologic: Fix Dockerfile image and move other sub-packages to the internal namespace too
* @prologic: Fix typo in profile template

### Documentation

* @prologic: Document Bitcask's usage in teh Tech Stack

### Features

* @prologic: Add Homebrew tap to goreleaser config
* @prologic: Add version string to twtd startup
* @prologic: Add a basic CLI client with login and post commands (#108)
* @dooven: Add hashtag search (#104)
* @prologic: Add FOLLOWERS:%d and FOLLOWING:%d to daily stats feed
* @prologic: Add section to /help on whot you need to create an account
* @prologic: Add backend handler /lookup for type-ahead / auot-complete @mention lookups from the UI
* @prologic: Add tooltip for toolbar buttons
* @prologic: Add &nbsp; between toolbar sections
* @prologic: Add strikethrough and fixed-width formatting buttons on the toolabr
* @prologic: Add other formatting uttons
* @prologic: Add support for syndication formats (RSS, Atom, JSON Feed) (#95)
* @prologic: Add Pull Request template
* @prologic: Add Contributor Code of Conduct
* @prologic: Add Github Downloads README badge
* @prologic: Add Docker Hub README badges
* @prologic: Add docs for the APIv1 /api/v1/post and /api/v1/follow endpoints
* @prologic: Add configuration open to have open user profiles (default: false)
* @prologic: Add basic e2e integration test framework (just a simple binary)
* @prologic: Add some more error logging
* @prologic: Add support for editing and deleting your last Twt (#88)
* @prologic: Add Contributing section to README
* @prologic: Add a CNAME (dev.twtxt.net) for developer docs
* @prologic: Add some basic developer docs
* @dooven: Add feature to allow users to manage their subFeeds (#80)
* @prologic: Add basic mentions view and handler
* @prologic: Add Docker image CI (#82)
* @prologic: Add MaxUploadSizd to server startup logs
* @prologic: Add reuseable template partials so we can reuse the post form, posts and pager

### Updates

* @prologic: Update CHANGELOG for 0.0.12
* @prologic: Update CHANGELOG for 0.0.12
* @prologic: Update CHANGELOG for 0.0.12
* @prologic: Update CHANGELOG for 0.0.12
* @prologic: Update /about page
* @prologic: Update issue templates
* @prologic: Update README.md
* @prologic: Update APIv1 spec docs, s/Methods/Method/g as all endpoints accept a single-method, if some accept different methods they will be a different endpoint


<a name="0.0.11"></a>
## [0.0.11](https://git.mills.io/yarnsocial/yarn/compare/0.0.10...0.0.11) (2020-08-02)

### Bug Fixes

* @prologic: Fix size of external feed icons
* @prologic: Fix alignment of Twts a bit better (align the actions and Twt post time)
* @prologic: Fix alignment of uploaded media to be display: block; aligned
* @prologic: Fix postas functionality post Media Upload (Missing form= attr)
* @prologic: Fix downscale resolution of media
* @prologic: Fix bug with appending new media URI to text input
* @prologic: Fix misuse of pronoun in postas dropdown field
* @prologic: Fix sourcer links in README
* @prologic: Fix bad error handling in /settings endpoint for missing avatar_file (Fixes #63)
* @prologic: Fix potential vulnerability and limit fetches to a configurable limit
* @prologic: Fix accidental double posting
* @prologic: Fix /settings handler to limit request body
* @dooven: Fix followers page (#53)
* @prologic: Fix wording on settings re showing followers publicly
* @prologic: Fix bug that incorrectly redirects to the / when you're posting from /discover
* @prologic: Fix profile template and profile type to show followers correctly with correct link
* @prologic: Fix Profile.Type setting when calling .Profile() on models
* @prologic: Fix a few misisng trimSuffix calls in some tempaltes
* @prologic: Fix sessino persistence and increase default session timeout to 10days (#49)
* @prologic: Fix session unmarshalling caused by 150690c
* @prologic: Fix the mess that is User/Feed URL vs. TwtURL (#47)
* @prologic: Fix user registration to disallow existing users and feeds
* @prologic: Fix the specialUsernames feeds for the adminuser properly on twtxt.net
* @prologic: Fix remainder of feeds on twtxt.net (we lost the contents of news oh well)
* @prologic: Fix new feed entities on twtxt.net
* @prologic: Fix all logging in background jobs  to only output warnings
* @prologic: Fix and tidy up dependencies

### Features

* @prologic: Add /api/v1/follow endpoint
* @prologic: Add /api/v1/discover endpoint
* @prologic: Add /api/v1/timeline endpoint and content negogiation for general NotFound handler
* @prologic: Add a basic APIv1 set of endpoints (#66)
* @shahxeb.malik: Add Media Upload Support (#69)
* @dooven: Add Etag in AvatarHandler (#67)
* @prologic: Add meta tags to base template
* @xdev4420: Add improved mobile friendly top navbar
* @prologic: Add logging for SMTP configuration on startup
* @prologic: Add configuration options for SMTP From addresss used
* @prologic: Add fixPossibleFeedFollowers migration for twtxt.net
* @prologic: Add avatar upload to /settings (#61)
* @prologic: Add update email to /settings (Fixees #55
* @prologic: Add Password Reset feature (#51)
* @prologic: Add list of local (sub)Feeds to the /feeds view for better discovery of user created feeds
* @prologic: Add Feed model with feed profiles
* @davencasia: Add link to followers
* @prologic: Add random tweet prompts for a nice variance on the text placeholder
* @prologic: Add user Avatars to the User Profile view as well
* @prologic: Add Identicons and support for Profile Avatars (#43)
* @davencasia: Add a flag that allows users to set if the public can see who follows them

### Updates

* @prologic: Update CHANGELOG for 0.0.11
* @prologic: Update README.md
* @prologic: Update README
* @prologic: Update and improve handling to include conventional (re ...) (#68)
* @prologic: Update pager wording
* @prologic: Update pager wording  (It's Twts)
* @prologic: Update CHANGELOG for 0.0.11
* @prologic: Update default list of external feeds and add we-are-twtxt
* @prologic: Update feed sources, refactor and improve the UI/UX by displaying feed sources by source instead of lumped together


<a name="0.0.10"></a>
## [0.0.10](https://git.mills.io/yarnsocial/yarn/compare/0.0.9...0.0.10) (2020-07-28)

### Bug Fixes

* @prologic: Fix bug in ExpandMentions
* @prologic: Fix incorrect log call
* @prologic: Fix server shutdown and signal handling to listen for SIGTERM and SIGINT
* @prologic: Fix twtxt.net missing user feeds for prologic (home_datacenter) wtf?!
* @prologic: Fix missing db.SetUser for fixUserURLs
* @prologic: Fix another bug in Profile template
* @prologic: Fix more bugs with User Profile view
* @prologic: Fix User Profile Latest Tweets
* @prologic: Fix build and remove unused vars in FixUserAccounts
* @prologic: Fix User URL and TwtURLs on twtxt.net  and reset them
* @prologic: Fix Context.IsLocal bug
* @prologic: Fix bug in User.Is function
* @prologic: Fix /settings view (again)
* @prologic: Fix build error (oops silly me)
* @prologic: Fix bug in /settings vieew
* @prologic: Fix missing feeds for @rob and @kt84  that went missing from their accounts :/
* @prologic: Fix UI/UX bug in text input with erroneous spaces
* @prologic: Fix adminUser account on twtxt.net
* @prologic: Fix user feeds on twtxt.net
* @prologic: Fix bug with feed creation (case sensitivity)
* @prologic: Fix Follow/Unfollow local events post v0.9.0 release re URL/TwtURL changes
* @prologic: Fix numerous bugs post v0.8.0 release (sorry) due to complications  with User Profile URL vs. Feed URL (TwtURL)
* @prologic: Fix Tweeter.URL on /discover
* @prologic: Fix syntax error (oops)
* @prologic: Fix adminUser feeds on twtxt.net
* @prologic: Fix link to user profiles in user settings followers/following
* @prologic: Fix Tagline in User Settings so you users can see what they have entered (if it was set)
* @prologic: Fix User.Following URIs post v0.9.0 break in URIs

### Features

* @prologic: Add fixAdminUser function to FixUserAccountsJob to add specialUser feeds to the configured AdminUser
* @prologic: Add SyncStore job to sync data to disk every 1m to prevent accidental data loss
* @prologic: Add logging when server is shutdown and store is synced/closed
* @prologic: Add local @mention automatic linking for local users and local feeds without a user having to follow  them first

### Updates

* @prologic: Update CHANGELOG for 0.0.10
* @prologic: Update README.md
* @prologic: Update README.md
* @prologic: Update README.md
* @prologic: Update startup to merge data store
* @prologic: Update deps
* @prologic: Update the FixUserAccounts job and remove all fixes, but leave  the job (we might breka more things)
* @prologic: Update FixUserAccounts job and remov most of the migration code now that twtxt.net data is fixed
* @prologic: Update FixUserAccounts job schedule to @hourly and remove adminUser.Feeds hack
* @prologic: Update  FixUserAccountsJob to eif User URL(s)
* @prologic: Update FixUserAccounts job back to 1h schedule


<a name="0.0.9"></a>
## [0.0.9](https://git.mills.io/yarnsocial/yarn/compare/0.0.8...0.0.9) (2020-07-26)

### Features

* @prologic: Add user profile pages and **BREAKS** existing local user feed URIs (#27)

### Updates

* @prologic: Update CHANGELOG for 0.0.9


<a name="0.0.8"></a>
## [0.0.8](https://git.mills.io/yarnsocial/yarn/compare/0.0.7...0.0.8) (2020-07-26)

### Bug Fixes

* @prologic: Fix the custom release-notes for goreleaser (finally)
* @prologic: Fix the gorelease custom release notes by skipping the gorelease changelog generation
* @prologic: Fix the release process, remove git-chglog use before running gorelease
* @prologic: Fix instructions on how to build from source (Fixes #30)
* @prologic: Fix PR blocks and remove autoAssign workflow that fails with Resource not accessible by integration
* @prologic: Fix releasee process to generate release-notes via git-chglog which are better than goreleaser's ones
* @prologic: Fix goarch in gorelease config (uggh)
* @prologic: Fix gorelease config (3rd time's the charm)
* @prologic: Fix gorelease config properly (this time)
* @prologic: Fix release tools and remove unused shell script
* @prologic: Fix goreleaser config
* @prologic: Fix binary release tools and process
* @prologic: Fix name of twtxt Docker Swarm Stackfile
* @prologic: Fix docker stack usage in README (Fixes #34)
* @prologic: Fix typo in feeds template
* @prologic: Fix error handling for user registrationg and return 404 Feed Not Found for non-existent users/feeds
* @prologic: Fix build error (oops)
* @prologic: Fix set of reserved vs. special usernames
* @prologic: Fix unconstrained no. of user feeds to prevent abuse
* @prologic: Fix error message when trying to register an account with a previously deleted user (to preserve feeds)
* @prologic: Fix potential abuse of unconstrained username lengths
* @prologic: Fix and remove  some useless debugging
* @prologic: Fix documentation on configuration options and warn about user registration being disabled (Fixes #29)
* @prologic: Fix the annoying greeting workflow and remove it (it's kind of annoying)

### Features

* @flavien: Add default configuration env values to docker-compose (#39)
* @prologic: Add git-chglog to release process to update the ongoing CHANGELOG too
* @prologic: Add feed creation event to the twtxt special feed
* @prologic: Add FixUserAccounts job logic to touch feed files for users that have never posted
* @prologic: Add automated internal special feed
* @flavien: Add documentation on using docker-compose (#26)
* @prologic: Add new feature for creating sub-feeds / personas allowing users to create topic-based feeds and poast as those topics
* @prologic: Add a section to the help page on formatting posts

### Updates

* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8
* @prologic: Update CHANGELOG for 0.0.8


<a name="0.0.7"></a>
## [0.0.7](https://git.mills.io/yarnsocial/yarn/compare/0.0.6...0.0.7) (2020-07-25)

### Bug Fixes

* @prologic: Fix .gitignore for ./data/sources
* @prologic: Fix bug updating followee Followers
* @prologic: Fix poor spacing between posts on larger devices (Fixes #28)
* @prologic: Fix and remove accidently commited data/sources file (data is meant to be empty)
* @prologic: Fix bug with Follow/Unfollow and updating Followers, missed using NormalizeUsername()
* @prologic: Fix updating Followers on Follow/Unfollow for local users too
* @prologic: Fix potential nil map bug
* @prologic: Fix user accounts and populate local Followers
* @prologic: Fix the settings Followers Follow/Unfollow state
* @prologic: Fix build system to permit passing VERSION and COMMIT via --build-arg for docker build
* @prologic: Fix the CI builds to actually build the daemon (#21)

### Features

* @prologic: Add a convenient UI/UX [Reply] feature on posts
* @prologic: Add twtxt special feed updates for Follow/Unfollow events from the local instance
* @prologic: Add confirmation on account deletion in case of accidental clicks
* @prologic: Add support for faster Docker builds by refactoring the Dockerfile (#20)
* @prologic: Add Docker Swarmmode Stackfile for production deployments based on https://twtxt.net/ (#22)
* @prologic: Add local (non-production) docker-compose.yml for reference and Docker-based development (#25)

### Updates

* @prologic: Update NewFixUserAccountsJob to 1h schedule


<a name="0.0.6"></a>
## [0.0.6](https://git.mills.io/yarnsocial/yarn/compare/0.0.5...0.0.6) (2020-07-23)

### Bug Fixes

* @prologic: Fix formatting in FormatRequest
* @prologic: Fix the session timeout (which was never set0
* @prologic: Fix some embarassing typos :)
* @prologic: Fix error handling for parsing feeds and feed sources

### Features

* @prologic: Add bad feed dtection and log possible bad feeds (no action taken yet)
* @prologic: Add new feature to detect new followers of feeds on twtxt.net from twtxt User-Agent strings
* @prologic: Add twtxt to reserve usernames
* @prologic: Add an improved /about page and add a /help page to help new users

### Updates

* @prologic: Update README and remove references to the non-existent CLI (this will come later)
* @prologic: Update default job interval of UpdateFeedSourcesJob


<a name="0.0.5"></a>
## [0.0.5](https://git.mills.io/yarnsocial/yarn/compare/0.0.4...0.0.5) (2020-07-21)

### Bug Fixes

* @prologic: Fix UI/UX handling around bad logins
* @prologic: Fix the follow self feature properly with more consistency
* @prologic: Fix firefox UI/UX issue by upgrading to PicoCSS v1.0.3 (#17)

### Features

* @prologic: Add /feeds view with configurable feed sources for discoverability of other sources of feeds
* @prologic: Add username validation to prevent more potential bad data
* @prologic: Add configurable theme (site-wide) and persist user-defined (vai cookies) theme selection (#16)
* @prologic: Add configurable maximum tweet length and cleanup tweets before they are stored to replace new lines, etc


<a name="0.0.4"></a>
## [0.0.4](https://git.mills.io/yarnsocial/yarn/compare/0.0.3...0.0.4) (2020-07-21)

### Bug Fixes

* @prologic: Fix links opening in new window with target=_blank
* @mindboosternoori: Fix typo on support page (#5)
* @prologic: Fix app versioning and add to base template so we can tell which version of twtxt is running
* @prologic: Fix bug in TwtfileHandler with case sensitivity of nick param

### Features

* @prologic: Add delete account support
* @prologic: Add better layout of tweets so they stand out more
* @prologic: Add support for Markdown formatting (#10)
* @prologic: Add pagination support (#9)
* @prologic: Add Follow/Unfollow to /discover view that understands the state of who a user follows or doesn't (#8)

### Updates

* @prologic: Update README.md
* @prologic: Update README.md


<a name="0.0.3"></a>
## [0.0.3](https://git.mills.io/yarnsocial/yarn/compare/0.0.2...0.0.3) (2020-07-19)

### Bug Fixes

* @prologic: Fix bug with NormalizeURL() incorrectly translating https:// to http://
* @prologic: Fix deps and cleanup unused ones
* @prologic: Fix the layout of thee /settings view
* @prologic: Fix a critical bug whereby users could re-register the same username and override someone else's account :/
* @prologic: Fix username case sensitivity and normalize by forcing all usersnames to be lowercase and whitespace stripped
* @prologic: Fix useability issue where some UI/UX would toggle between authenticated and unauthentiated state causing confusion

### Features

* @prologic: Add support for configuring flags from the environment via the same long option names
* @prologic: Add options to configure session cookie secret and expiry
* @prologic: Add Contributing guideline and TODO
* @prologic: Add additional logic to fix null/blank user account URL(s) to the FixUserAccountJob as well
* @prologic: Add FixUserAccountsJob to fix existing broken accounts that might already exist
* @prologic: Add new /discover view for convenience access to the global instance's timeline  with easy to use Follow links for discoverability


<a name="0.0.2"></a>
## [0.0.2](https://git.mills.io/yarnsocial/yarn/compare/0.0.1...0.0.2) (2020-07-19)

### Bug Fixes

* @prologic: Fix the  follow self behaviour to actually work
* @prologic: Fix defaults to use the same  ones in twtxt's options
* @prologic: Fix  URL() of User objects
* @prologic: Fix import and hard-code no. of tweets to display

### Features

* @prologic: Add feature whereby new registered users follow themselves by default
* @prologic: Add support, stargazers and contributing info to READmE
* @prologic: Add enhanced server capability for graceful/clean shutdowns
* @prologic: Add /import feature to import multiple feeds at once (#1)

### Updates

* @prologic: Update feed update frequency to 5m by default


<a name="0.0.1"></a>
## 0.0.1 (2020-07-18)

### Bug Fixes

* @prologic: Fix release tool
* @prologic: Fix screenshots
* @prologic: Fix broken links and incorrect text that hasn't happened yet
* @prologic: Fix /login CTA link on /register page
* @prologic: Fix /register CTA link on /login page
* @prologic: Fix parsing store uri
* @prologic: Fix bug ensuring feedsDir exists
* @prologic: Fix Dockerfile

### Features

* @prologic: Add theme-switcher and remove erroneous prism.js

### Updates

* @prologic: Update README.md

