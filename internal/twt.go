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

func DeleteTwt(conf *Config, feed string, twt types.Twt) error {
	fn := filepath.Join(conf.Data, feedsDir, feed)
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Error("error opening feed")
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		log.WithError(err).Error("error getting feed stat")
		return err
	}

	pr, err := types.ReadPreambleFeed(f, stat.Size())
	if err != nil {
		log.WithError(err).Error("error reading feed")
		return err
	}

	preample := pr.Preamble()

	tf, err := types.ParseFile(f, types.Twter{})
	if err != nil {
		log.WithError(err).Errorf("error processing feed %s", fn)
		return err
	}

	return nil
}

func DeleteLastTwt(conf *Config, feed string) error {
	_, n, err := GetLastTwt(conf, feed)
	if err != nil {
		return err
	}

	fn := filepath.Join(conf.Data, feedsDir, feed)
	f, err := os.OpenFile(fn, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	return f.Truncate(int64(n))
}

func GetLastTwt(conf *Config, feed string) (twt types.Twt, offset int, err error) {
	twt = types.NilTwt

	fn := filepath.Join(filepath.Join(conf.Data, feedsDir, feed))
	if !FileExists(fn) {
		return
	}

	var data []byte
	data, offset, err = read_file_last_line.ReadLastLine(fn)
	if err != nil {
		return
	}

	twt, err = types.ParseLine(string(data), user.Twter())

	return
}

func AppendSpecial(conf *Config, db Store, specialUsername, text string, args ...interface{}) (types.Twt, error) {
	user := &User{Username: specialUsername}
	user.Following = make(map[string]string)
	return AppendTwt(conf, db, user, text, args)
}

func AppendTwt(conf *Config, db Store, user *User, text string, args ...interface{}) (types.Twt, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return types.NilTwt, fmt.Errorf("cowardly refusing to twt empty text, or only spaces")
	}

	p := filepath.Join(conf.Data, feedsDir)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating feeds directory")
		return types.NilTwt, err
	}

	fn := filepath.Join(p, user.Username)

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

	twt := types.MakeTwt(user.Twter(), now, strings.TrimSpace(text))

	twt.ExpandMentions(conf, NewFeedLookup(conf, db, user))
	if _, err = fmt.Fprintf(f, "%+l\n", twt); err != nil {
		return types.NilTwt, err
	}

	return twt, nil
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
		URL:  URLForUser(conf.BaseURL, name),
	}
	fn := filepath.Join(p, name)
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening feed: %s", fn)
		return nil, err
	}
	t, err := types.ParseFile(f, twter)
	if err != nil {
		log.WithError(err).Errorf("error processing feed %s", fn)
		return nil, err
	}
	twts = append(twts, t.Twts()...)
	f.Close()

	return twts, nil
}
