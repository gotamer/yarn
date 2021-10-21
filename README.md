# yarn

[![Build Status](https://ci.mills.io/api/badges/yarnsocial/yarn/status.svg)](https://ci.mills.io/yarnsocial/yarn)

📕 yarn is a Self-Hosted, Twitter™-like Decentralised micro-Blogging platform. No ads, no tracking, your content, your data!

- `yarnd` is the [Yarn.social](https://yarn.social) pod backend server
- `yarnc` is the command-line client to `yarnd` API and command-line Twtxt client

See [Yarn.social] for more deatils

## Installation

### Pre-built Binaries

__NOTE:__ Please don't use the pre-built binaries until [Issue #250](https://git.mills.io/yarnsocial/yarn/issues/250) is resolved.
          Please build from source or use the [Docker Images](https://hub.docker.com/jointwt).
          Thank you. 🙇‍♂️

As a first point, please try to use one of the pre-built binaries that are
available on the [Releases](https://git.mills.io/yarnsocial/yarn/releases) page.

### Using Homebrew

We provide [Homebrew](https://brew.sh) formulae for macOS users for both the
command-line client (`yarn`) as well as the server (`yarnd`).

```console
brew tap yarnsocial/yarn https://git.mills.io/yarnsocial/homebrew-yarn.git
brew install yarn
```

Run the server:

```console
yarnd
```

Run the command-line client:

```console
yarn
```

### Building from source

This is an option if you are familiar with [Go](https://golang.org) development.

1. Clone this repository (_this is important_)

```console
git clone https://git.mills.io/yarnsocial/yarn.git
```

2. Install required dependencies (_this is important_)

Linux, macOS:

```console
make deps
```
Note that in order to get the media upload functions to work, you need to
install ffmpeg and its associated `-dev` packages. Consult your distribution's package
repository for availability and names.

FreeBSD:

- Install `gmake`
- Install `pkgconf` that brings `pkg-config`
`gmake deps`

3. Build the binaries

Linux, macOS:

```console
make
```

FreeBSD:

```console
gmake
```


## Usage

### Command-line Client

1. Login to  your [Yarn.social](https://yarn.social) pod:

```#!console
$ ./yarn login
INFO[0000] Using config file: /Users/prologic/.twt.yaml
Username:
```

2. Viewing your timeline

```#!console
$ ./yarn timeline
INFO[0000] Using config file: /Users/prologic/.twt.yaml
> prologic (50 minutes ago)
Hey @rosaelefanten 👋 Nice to see you have a Twtxt feed! Saw your [Tweet](https://twitter.com/koehr_in/status/1326914925348982784?s=20) (_or at least I assume it was yours?_). Never heard of `aria2c` till now! 🤣 TIL

> dilbert (2 hours ago)
Angry Techn Writers ‣ https://dilbert.com/strip/2020-11-14
```

3. Making a Twt (_post_):

```#!console
$ ./yarn post
INFO[0000] Using config file: /Users/prologic/.twt.yaml
Testing `yarn` the command-line client
INFO[0015] posting twt...
INFO[0016] post successful
```

For additional help on using the `yarnc` command-line client:

```#!console
$ yarnc help
This is the command-line client for Yarn.social pods running
yarnd. This tool allows a user to interact with a pod to view their timeline,
following feeds, make posts and managing their account.

Usage:
  yarnc [command]

Available Commands:
  completion  generate the autocompletion script for the specified shell
  help        Help about any command
  login       Login and euthenticate to a Yarn.social pod
  post        Post a new twt to a Yarn.social pod
  stats       Parses and performs statistical analytis on a Twtxt feed given a URL or local file
  timeline    Display your timeline

Flags:
  -c, --config string   set a custom config file (default "/Users/prologic/.yarnc.yml")
  -d, --debug           Enable debug logging
  -h, --help            help for yarnc
  -p, --parser string   Set active parse engine [lextwt, retwt] (default "lextwt")
  -t, --token string    yarnd API token to use to authenticate to endpoints (default "$YARNC_TOKEN")
  -u, --uri string      yarnd API endpoint URI to connect to (default "http://localhost:8000/api/v1/")

Use "yarnc [command] --help" for more information about a command.
```

### Deploy with Docker Compose

Run the compose configuration:

```console
docker-compose up -d
```

Then visit: http://localhost:8000/

### Web App

Run yarnd:

```console
yarnd -R
```

__NOTE:__ Registrations are disabled by default so hence the `-R` flag above.

Then visit: http://localhost:8000/

You can configure other options by specifying them on the command-line:

```console
$ ./yarnd --help
Usage of ./yarnd:
  -E, --admin-email string            default admin user email (default "support@yarn.social")
  -N, --admin-name string             default admin user name (default "Administrator")
  -A, --admin-user string             default admin user to use (default "admin")
      --api-session-time duration     timeout for api tokens to expire (default 240h0m0s)
      --api-signing-key string        secret to use for signing api tokens (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
  -u, --base-url string               base url to use (default "http://0.0.0.0:8000")
  -b, --bind string                   [int]:<port> to bind to (default "0.0.0.0:8000")
      --cookie-secret string          cookie secret to use secure sessions (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
  -d, --data string                   data directory (default "./data")
  -D, --debug                         enable debug logging
  -m, --description string            set the pod's description (default "🧶 Yarn.social is a Self-Hosted, Twitter™-like Decentralised microBlogging platform. No ads, no tracking, your content, your data!")
      --enable-feature feature        enable the named feature
      --feed-sources strings          external feed sources for discovery of other feeds (default [https://feeds.twtxt.net/we-are-feeds.txt])
      --fetch-interval string         cache fetch interval (how often to update feeds) in cron syntax (https://pkg.go.dev/github.com/robfig/cron) (default "@every 5m")
  -l, --lang string                   set the default language (default "auto")
      --magiclink-secret string       magiclink secret to use for password reset tokens (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
  -I, --max-cache-items int           maximum cache items (per feed source) of cached twts in memory (default 150)
  -C, --max-cache-ttl duration        maximum cache ttl (time-to-live) of cached twts in memory (default 240h0m0s)
  -F, --max-fetch-limit int           maximum feed fetch limit in bytes (default 2097152)
  -L, --max-twt-length int            maximum length of posts (default 288)
  -U, --max-upload-size int           maximum upload size of media (default 16777216)
  -n, --name string                   set the pod's name (default "twtxt.net")
  -O, --open-profiles                 whether or not to have open user profiles
  -R, --open-registrations            whether or not to have open user registgration
  -P, --parser string                 set active parsing engine (default "lextwt")
      --pop3-bind string              POP3 interface and port to bind to (default "0.0.0.0:8110")
      --session-cache-ttl duration    time-to-live for cached sessions (default 1h0m0s)
      --session-expiry duration       timeout for sessions to expire (default 240h0m0s)
      --smtp-bind string              SMTP interface and port to bind to (default "0.0.0.0:8025")
      --smtp-from string              SMTP From to use for email sending (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
      --smtp-host string              SMTP Host to use for email sending (default "smtp.gmail.com")
      --smtp-pass string              SMTP Pass to use for email sending (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
      --smtp-port int                 SMTP Port to use for email sending (default 587)
      --smtp-user string              SMTP User to use for email sending (default "INVALID CONFIG VALUE - PLEASE CHANGE THIS VALUE")
  -s, --store string                  store to use (default "bitcask://twtxt.db")
  -t, --theme string                  set the default theme (default "auto")
      --transcoder-timeout duration   timeout for the video transcoder (default 10m0s)
  -T, --twts-per-page int             maximum twts per page to display (default 50)
  -v, --version                       display version information
      --whitelist-domain strings      whitelist of external domains to permit for display of inline images (default [imgur\.com,giphy\.com,imgs\.xkcd\.com,tube\.mills\.io,reactiongifs\.com,githubusercontent\.com])
pflag: help requested
```

## Configuring your Pod

At a bare minimum you should set the following options:

- `-d /path/to/data`
- `-s bitcask:///path/to/data/twtxt.db` (_we will likely simplify/default this_)
- `-R` to enable open registrations.
- `-O` to enable open profiles.

Most other configuration values _should_ be done via environment variables.

It is _recommended_ you pick an account you want to use to "administer" the
pod with and set the following environment values:

- `ADMIN_USER=username`
- `ADMIN_EMAIL=email`

In order to configure email settings for password recovery and the `/support`
and `/abuse` endpoints, you should set appropriate `SMTP_` values.

It is **highly** recommended you also set the following values to secure your Pod:

- `API_SIGNING_KEY`
- `COOKIE_SECRET`
- `MAGICLINK_SECRET`

These values _should_ be generated with a secure random number generator and
be of length `64` characters long. You can use the following shell snippet
to generate secrets for your pod for the above values:

```console
$ cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 64 | head -n 1
```

**DO NOT** publish or share these values. **BE SURE** to only set them as env vars.

## Production Deployments

### Docker Swarm

You can deploy `yarnd` to a [Docker Swarm](https://docs.docker.com/engine/swarm/)
cluster by utilising the provided `yarn.yaml` Docker Stack. This also depends on
and uses the [Traefik](https://docs.traefik.io/) ingress load balancer so you must
also have that configured and running in your cluster appropriately.

```console
docker stack deploy -c yarn.yml
```

## Contributing

Interested in contributing to this project? You are welcome! Here are some ways
you can contribute:

- [File an Issue](https://git.mills.io/yarnsocial/yarn/issues/new) -- For a bug,
  or interesting idea you have for a new feature or just general questions.
- Submit a Pull-Request or two! We welcome all PR(s) that improve the project!

Please see the [Contributing Guidelines](/CONTRIBUTING.md) and checkout the
[Developer Documentation](https://dev.twtxt.net) or over at [/docs](/docs).

## Contributors

Thank you to all those that have contributed to this project, battle-tested it, used it in their own projects or products, fixed bugs, improved performance and even fix tiny typos in documentation! Thank you and keep contributing!

You can find an [AUTHORS](/AUTHORS) file where we keep a list of contributors to the project. If you contribute a PR please consider adding your name there.

## Related Projects

- [Yarn.social](https://git.mills.io/yarnsocial/yarn.social) -- [Yarn.social](https://yarn.social) landing page
- [Yarns](https://git.mills.io/yarnsocial/yarns) -- The [Yarn.social](https://yarn.social) search engine hosted at [search.twtxt.net](https://search.twtxt.net)
- [Goryon](https://git.mills.io/yarnsocial/goryon) -- Our Flutter iOS and Android Mobile App
- [rss2twtxt](https://git.mills.io/yarnsocial/rss2twtxt) -- RSS/Atom/Twitter to [Twtxt](https://twtxt.readthedocs.org) aggregator service hosted at [feeds.twtxt.net](https://feeds.twtxt.net)

## License

`yarn` is licensed under the terms of the [MIT License](/LICENSE)
