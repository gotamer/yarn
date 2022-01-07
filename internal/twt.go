// -*- tab-width: 4; -*-

package internal

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	read_file_last_line "git.mills.io/prologic/read-file-last-line"
	log "github.com/sirupsen/logrus"

	"git.mills.io/yarnsocial/yarn/types"
)

const (
	feedsDir = "feeds"
)

func DeleteLastTwt(conf *Config, user *User) error {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return err
	}

	fn := filepath.Join(p, user.Username)

	_, n, err := GetLastTwt(conf, user)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Truncate(int64(n))
}

type AppendTwtFunc func(user *User, feed *Feed, text string, args ...interface{}) (types.Twt, error)

func AppendTwtFactory(conf *Config, db Store) AppendTwtFunc {
	isAdminUser := IsAdminUserFactory(conf)
	canPostAsFeed := func(user *User, feed *Feed) bool {
		if user.OwnsFeed(feed.Name) {
			return true
		}
		if IsSpecialFeed(feed.Name) && isAdminUser(user) {
			return true
		}
		return false
	}

	return func(user *User, feed *Feed, text string, args ...interface{}) (types.Twt, error) {
		text = strings.TrimSpace(text)
		if text == "" {
			return types.NilTwt, fmt.Errorf("cowardly refusing to twt empty text, or only spaces")
		}

		p := filepath.Join(conf.Data, feedsDir)
		if err := os.MkdirAll(p, 0755); err != nil {
			log.WithError(err).Error("error creating feeds directory")
			return types.NilTwt, err
		}

		if feed != nil && !canPostAsFeed(user, feed) {
			log.Warnf("unauthorized attempt to post to feed %s from user %s", feed, user)
			return types.NilTwt, fmt.Errorf("unauthorized attempt to post to feed %s from user %s", feed, user)
		}

		var fn string

		if feed == nil {
			fn = filepath.Join(p, user.Username)
		} else {
			fn = filepath.Join(p, feed.Name)
		}

		f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return types.NilTwt, err
		}
		defer f.Close()

		// Support replacing/editing an existing Twt whilst preserving Created Timestamp
		now := time.Now()
		if len(args) == 1 {
			if t, ok := args[0].(time.Time); ok {
				now = t
			}
		}

		var twter types.Twter

		if feed == nil {
			twter = user.Twter(conf)
		} else {
			twter = feed.Twter(conf)
		}

		// XXX: This is a bit convoluted @xuu can we improve this somehow?
		tmpTwt := types.MakeTwt(twter, now, strings.TrimSpace(text))
		tmpTwt.ExpandMentions(conf, NewFeedLookup(conf, db, user))
		newText := tmpTwt.FormatText(types.LiteralFmt, conf)
		twt := types.MakeTwt(twter, now, newText)

		if _, err = fmt.Fprintf(f, "%+l\n", twt); err != nil {
			return types.NilTwt, err
		}

		return twt, nil
	}
}

func FeedExists(conf *Config, username string) bool {
	fn := filepath.Join(conf.Data, feedsDir, NormalizeUsername(username))
	if _, err := os.Stat(fn); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

func GetLastTwt(conf *Config, user *User) (twt types.Twt, offset int, err error) {
	twt = types.NilTwt

	p := filepath.Join(conf.Data, feedsDir)
	if err = os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return
	}

	fn := filepath.Join(p, user.Username)
	if !FileExists(fn) {
		return
	}

	var data []byte
	data, offset, err = read_file_last_line.ReadLastLine(fn)
	if err != nil {
		return
	}

	twter := user.Twter(conf)
	twt, err = types.ParseLine(string(data), &twter)

	return
}

func GetAllFeeds(conf *Config) ([]string, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	files, err := ioutil.ReadDir(p)
	if err != nil {
		log.WithError(err).Error("error reading feeds directory")
		return nil, err
	}

	fns := []string{}
	for _, fileInfo := range files {
		fn := filepath.Base(fileInfo.Name())
		// feeds with an extension are rotated/archived feeds
		// e.g: prologic.1 prologic.2
		if filepath.Ext(fn) != "" {
			continue
		}
		fns = append(fns, fn)
	}
	return fns, nil
}

func GetFeedCount(conf *Config, name string) (int, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return 0, err
	}

	fn := filepath.Join(p, name)

	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Error("error opening feed file")
		return 0, err
	}
	defer f.Close()

	return LineCount(f)
}

func GetAllTwts(conf *Config, name string) (types.Twts, error) {
	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return nil, err
	}

	var twts types.Twts

	twter := types.Twter{
		Nick: name,
		URI:  URLForUser(conf.BaseURL, name),
	}
	fn := filepath.Join(p, name)
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening feed: %s", fn)
		return nil, err
	}
	defer f.Close()
	t, err := types.ParseFile(f, &twter)
	if err != nil {
		log.WithError(err).Errorf("error processing feed %s", fn)
		return nil, err
	}
	twts = append(twts, t.Twts()...)
	f.Close()

	return twts, nil
}
