package internal

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"math"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	text_template "text/template"
	"time"

	// Blank import so we can handle image/jpeg
	_ "image/gif"
	_ "image/jpeg"
	"image/png"

	"git.mills.io/prologic/go-gopher"
	"git.mills.io/yarnsocial/yarn"
	"git.mills.io/yarnsocial/yarn/types"
	"git.mills.io/yarnsocial/yarn/types/lextwt"
	"github.com/PuerkitoBio/goquery"
	"github.com/audiolion/ipip"
	"github.com/disintegration/gift"
	"github.com/disintegration/imageorient"
	"github.com/dustin/go-humanize"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/goware/urlx"
	"github.com/h2non/filetype"
	shortuuid "github.com/lithammer/shortuuid/v3"
	"github.com/microcosm-cc/bluemonday"
	"github.com/nullrocks/identicon"
	sync "github.com/sasha-s/go-deadlock"
	log "github.com/sirupsen/logrus"
	"github.com/writeas/slug"
	"golang.org/x/crypto/blake2b"
)

const (
	avatarsDir  = "avatars"
	externalDir = "external"
	mediaDir    = "media"

	newsSpecialUser    = "news"
	helpSpecialUser    = "help"
	supportSpecialUser = "support"

	me       = "me"
	twtxtBot = "twtxt"
	statsBot = "stats"

	maxUsernameLength   = 15 // avg 6 chars / 2 syllables per name commonly
	maxFeedNameLength   = 25 // avg 4.7 chars per word in English so ~5 words
	maxTwtContextLength = 140

	DayAgo   = time.Hour * 24
	WeekAgo  = DayAgo * 7
	MonthAgo = DayAgo * 30
	YearAgo  = MonthAgo * 12
)

// TwtTextFormat represents the format of which the twt text gets formatted to
type TwtTextFormat int

const (
	// MarkdownFmt to use markdown format
	MarkdownFmt TwtTextFormat = iota
	// HTMLFmt to use HTML format
	HTMLFmt
	// TextFmt to use for og:description
	TextFmt
)

var (
	specialUsernames = []string{
		newsSpecialUser,
		helpSpecialUser,
		supportSpecialUser,
	}
	reservedUsernames = []string{
		me,
		statsBot,
		twtxtBot,
	}
	automatedFeeds = []string{
		statsBot,
		twtxtBot,
	}
	specialFeeds = append(
		append([]string{}, specialUsernames...),
		automatedFeeds...)

	validFeedName     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	validUsername     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]+$`)
	singleUserUARegex = regexp.MustCompile(`(.+) \(\+(https?://\S+/\S+); @(\S+)\)`)
	multiUserUARegex  = regexp.MustCompile(`(.+) \(~(https?://\S+\/\S+); contact=(https?://\S+)\)`)
	yarndUserUARegex  = regexp.MustCompile(`(.+) \(Pod: (\S+) Support: (https?://\S+)\)`)

	ErrInvalidFeedName  = errors.New("error: invalid feed name")
	ErrBadRequest       = errors.New("error: request failed with non-200 response")
	ErrFeedNameTooLong  = errors.New("error: feed name is too long")
	ErrInvalidUsername  = errors.New("error: invalid username")
	ErrUsernameTooLong  = errors.New("error: username is too long")
	ErrInvalidUserAgent = errors.New("error: invalid twtxt user agent")
	ErrReservedUsername = errors.New("error: username is reserved")
	ErrInvalidImage     = errors.New("error: invalid image")
	ErrInvalidAudio     = errors.New("error: invalid audio")
	ErrInvalidVideo     = errors.New("error: invalid video")
	ErrInvalidVideoSize = errors.New("error: invalid video size")
)

func GenerateRandomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func DecodeHash(hash string) ([]byte, error) {
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	return encoding.DecodeString(strings.ToUpper(hash))
}

func FastHash(data []byte) string {
	sum := blake2b.Sum256(data)

	// Base32 is URL-safe, unlike Base64, and shorter than hex.
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	hash := strings.ToLower(encoding.EncodeToString(sum[:]))

	return hash
}

func FastHashString(s string) string {
	return FastHash([]byte(s))
}

func FastHashFile(fn string) (string, error) {
	data, err := os.ReadFile(fn)
	if err != nil {
		return "", err
	}
	return FastHash(data), nil
}

func IntPow(x, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}

func Slugify(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("Slugify(): error parsing uri: %s", uri)
		return ""
	}

	return slug.Make(fmt.Sprintf("%s/%s", u.Hostname(), u.Path))
}

func GenerateAvatar(conf *Config, domainNick string) (image.Image, error) {
	ig, err := identicon.New(conf.Name, 7, 4)
	if err != nil {
		log.WithError(err).Error("error creating identicon generator")
		return nil, err
	}

	ii, err := ig.Draw(domainNick)
	if err != nil {
		log.WithError(err).Errorf("error generating external avatar for %s", domainNick)
		return nil, err
	}

	return ii.Image(conf.AvatarResolution), nil
}

func ReplaceExt(fn, newExt string) string {
	oldExt := filepath.Ext(fn)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(fn, oldExt), newExt)
}

func HasExternalAvatarChanged(conf *Config, twter types.Twter) bool {
	uri := NormalizeURL(twter.URI)
	slug := Slugify(uri)
	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.cbf", slug))

	log := log.WithField("uri", uri)

	// If the %s.cbf doesn't yet exist, then assume the external avatar has changed
	if !FileExists(fn) {
		return true
	}

	// If the twter.Avatar uri is empty but a %s.cbf exists then assume the avatar has not changed
	if twter.Avatar == "" {
		return false
	}

	// If the twter.Avatar uri cannot be parsed but a %s.cbf exists then assume the avatar has not changed
	u, err := url.Parse(twter.Avatar)
	if err != nil {
		log.WithError(err).Warnf("error parsing avatar url for %s", twter.Avatar)
		return false
	}

	// if we cannot read the %s.cbf file assume the avatar has not changed
	data, err := os.ReadFile(fn)
	if err != nil {
		log.WithError(err).Warnf("error reading avatar cbf for %s", slug)
		return false
	}

	// compare the cbf(s)
	return string(data) != FastHashString(u.String())
}

func GetExternalAvatar(conf *Config, twter types.Twter) {
	uri := NormalizeURL(twter.URI)
	slug := Slugify(uri)
	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.png", slug))

	log := log.WithField("uri", uri)

	//
	// Use an already cached Avatar (unless there's a new one!)
	//

	if FileExists(fn) && !HasExternalAvatarChanged(conf, twter) {
		return
	}

	// Use the Avatar advertised in the feed
	if twter.Avatar != "" {
		u, err := url.Parse(twter.Avatar)
		if err != nil {
			log.WithError(err).Errorf("error parsing avatar url %s", twter.Avatar)
			return
		}

		opts := &ImageOptions{Resize: true, Width: conf.AvatarResolution, Height: conf.AvatarResolution}
		if _, err := DownloadImage(conf, u.String(), externalDir, slug, opts); err != nil {
			log.WithError(err).Errorf("error downloading external avatar: %s", u)
			return
		}
		if err := os.WriteFile(ReplaceExt(fn, ".cbf"), []byte(FastHashString(u.String())), 0644); err != nil {
			log.WithError(err).Warnf("error writing avatar cbf for %s", slug)
		}
		return
	}
}

func RequestGopher(conf *Config, uri string) (*gopher.Response, error) {
	res, err := gopher.Get(uri)
	if err != nil {
		log.WithError(err).Errorf("%s: client.Do fail: %s", uri, err)
		return nil, err
	}

	if res.Type != gopher.FILE {
		return nil, fmt.Errorf("unexpected type %s (expected FILE)", res.Type)
	}

	return res, nil
}

func Request(conf *Config, method, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.WithError(err).Errorf("%s: http.NewRequest fail: %s", url, err)
		return nil, err
	}

	if headers == nil {
		headers = make(http.Header)
	}

	// Set a default User-Agent (if none set)
	if headers.Get("User-Agent") == "" {
		headers.Set(
			"User-Agent",
			fmt.Sprintf(
				"yarnd/%s (Pod: %s Support: %s)",
				yarn.FullVersion(), conf.Name, URLForPage(conf.BaseURL, "support"),
			),
		)
	}

	req.Header = headers

	client := http.Client{
		Timeout: conf.RequestTimeout(),
	}

	res, err := client.Do(req)
	if err != nil {
		log.WithError(err).Errorf("%s: client.Do fail: %s", url, err)
		return nil, err
	}

	return res, nil
}

func LineCount(r io.Reader) (int, error) {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := r.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// CmdExists ...
func CmdExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// RunCmd ...
func RunCmd(timeout time.Duration, command string, args ...string) error {
	var (
		ctx    context.Context
		cancel context.CancelFunc
	)

	if timeout > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if ws, ok := exitError.Sys().(syscall.WaitStatus); ok && ws.Signal() == syscall.SIGKILL {
				err = &ErrCommandKilled{Err: err, Signal: ws.Signal()}
			} else {
				err = &ErrCommandFailed{Err: err, Status: exitError.ExitCode()}
			}
		}

		log.
			WithError(err).
			WithField("out", string(out)).
			Errorf("error running command")

		return err
	}

	return nil
}

// RenderLogo ...
func RenderLogo(logo string, podName string) (template.HTML, error) {
	t := text_template.Must(text_template.New("logo").Parse(logo))
	buf := bytes.NewBuffer([]byte{})
	err := t.Execute(buf, map[string]string{"PodName": podName})
	if err != nil {
		return "", err
	}

	return template.HTML(buf.String()), nil
}

func IsLocalURLFactory(conf *Config) func(url string) bool {
	return func(url string) bool {
		if NormalizeURL(url) == "" {
			return false
		}
		return strings.HasPrefix(NormalizeURL(url), NormalizeURL(conf.BaseURL))
	}
}

func GetUserFromURL(conf *Config, db Store, url string) (*User, error) {
	if !strings.HasPrefix(url, conf.BaseURL) {
		return nil, fmt.Errorf("error: %s does not match our base url of %s", url, conf.BaseURL)
	}

	userURL := UserURL(url)
	username := filepath.Base(userURL)

	return db.GetUser(username)
}

func WebMention(target, source string) error {
	targetURL, err := url.Parse(target)
	if err != nil {
		log.WithError(err).Error("error parsing target url")
		return err
	}
	sourceURL, err := url.Parse(source)
	if err != nil {
		log.WithError(err).Error("error parsing source url")
		return err
	}
	webmentions.SendNotification(targetURL, sourceURL)
	return nil
}

func StringKeys(kv map[string]string) []string {
	var res []string
	for k := range kv {
		res = append(res, k)
	}
	return res
}

func StringValues(kv map[string]string) []string {
	var res []string
	for _, v := range kv {
		res = append(res, v)
	}
	return res
}

func MapStrings(xs []string, f func(s string) string) []string {
	var res []string
	for _, x := range xs {
		res = append(res, f(x))
	}
	return res
}

func HasString(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func UniqStrings(xs []string) []string {
	set := make(map[string]bool)
	for _, x := range xs {
		if _, ok := set[x]; !ok {
			set[x] = true
		}
	}

	res := []string{}
	for k := range set {
		res = append(res, k)
	}
	return res
}

func RemoveString(xs []string, e string) []string {
	res := []string{}
	for _, x := range xs {
		if x == e {
			continue
		}
		res = append(res, x)
	}
	return res
}

func UniqueKeyFor(kv map[string]string, k string) string {
	K := k
	for i := 1; i < 99; i++ {
		if _, ok := kv[K]; !ok {
			return K
		}
		K = fmt.Sprintf("%s_%d", k, i)
	}
	return fmt.Sprintf("%s_???", k)
}

func IsImage(fn string) bool {
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening file %s", fn)
		return false
	}
	defer f.Close()

	head := make([]byte, 261)
	if _, err := f.Read(head); err != nil {
		log.WithError(err).Warnf("error reading from file %s", fn)
		return false
	}

	return filetype.IsImage(head)
}

func IsAudio(fn string) bool {
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening file %s", fn)
		return false
	}
	defer f.Close()

	head := make([]byte, 261)
	if _, err := f.Read(head); err != nil {
		log.WithError(err).Warnf("error reading from file %s", fn)
		return false
	}

	return filetype.IsAudio(head)
}

func IsVideo(fn string) bool {
	f, err := os.Open(fn)
	if err != nil {
		log.WithError(err).Warnf("error opening file %s", fn)
		return false
	}
	defer f.Close()

	head := make([]byte, 261)
	if _, err := f.Read(head); err != nil {
		log.WithError(err).Warnf("error reading from file %s", fn)
		return false
	}

	return filetype.IsVideo(head)
}

type ImageOptions struct {
	Resize bool
	Width  int
	Height int
}

type AudioOptions struct {
	Resample   bool
	Channels   int
	Samplerate int
	Bitrate    int
}

type VideoOptions struct {
	Resize bool
	Size   int
}

func DownloadImage(conf *Config, url string, resource, name string, opts *ImageOptions) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		log.WithError(err).Errorf("error downloading image from %s", url)
		return "", err
	}
	defer res.Body.Close()

	tf, err := receiveFile(res.Body, "rss2twtxt-*")
	if err != nil {
		return "", err
	}

	if !IsImage(tf.Name()) {
		return "", ErrInvalidImage
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return "", err
	}

	return ProcessImage(conf, tf.Name(), resource, name, opts)
}

func ReceiveAudio(r io.Reader) (string, error) {
	tf, err := receiveFile(r, "twtxt-upload-*")
	if err != nil {
		return "", err
	}

	if !IsAudio(tf.Name()) {
		return "", ErrInvalidAudio
	}

	return tf.Name(), nil
}

func ReceiveImage(r io.Reader) (string, error) {
	tf, err := receiveFile(r, "twtxt-upload-*")
	if err != nil {
		return "", err
	}

	if !IsImage(tf.Name()) {
		return "", ErrInvalidImage
	}

	return tf.Name(), nil
}

func ReceiveVideo(r io.Reader) (string, error) {
	tf, err := receiveFile(r, "twtxt-upload-*")
	if err != nil {
		return "", err
	}

	if !IsVideo(tf.Name()) {
		return "", ErrInvalidVideo
	}

	return tf.Name(), nil
}

func receiveFile(r io.Reader, filePattern string) (*os.File, error) {
	tf, err := ioutil.TempFile("", filePattern)
	if err != nil {
		log.WithError(err).Error("error creating temporary file")
		return nil, err
	}

	if _, err := io.Copy(tf, r); err != nil {
		log.WithError(err).Error("error writing temporary file")
		return tf, err
	}

	if _, err := tf.Seek(0, io.SeekStart); err != nil {
		log.WithError(err).Error("error seeking temporary file")
		return tf, err
	}

	return tf, nil
}

func copyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func TranscodeAudio(conf *Config, ifn string, resource, name string, opts *AudioOptions) (string, error) {
	defer os.Remove(ifn)

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Errorf("error creating %s directory", resource)
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.mp3", uuid))
	} else {
		ofn = fmt.Sprintf("%s.mp3", filepath.Join(p, name))
	}

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	wg := sync.WaitGroup{}

	TranscodeMP3 := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			"-y",
			"-i", ifn,
			"-acodec", "mp3",
			"-strict", "-2",
			"-loglevel", "quiet",
			ReplaceExt(ofn, ".mp3"),
		); err != nil {
			log.WithError(err).Error("error transcoding video")
			errs <- err
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalErr error

	nErrors := 0
	errChan := make(chan error)

	wg.Add(1)

	go TranscodeMP3(ctx, errChan)

	go func(ctx context.Context) {
		for {
			select {
			case err, ok := <-errChan:
				if !ok {
					return
				}
				nErrors++
				log.WithError(err).Errorf("TranscodeVideo() error")

				if errors.Is(err, &ErrCommandKilled{}) {
					finalErr = &ErrTranscodeTimeout{Err: err}
				} else {
					finalErr = &ErrTranscodeFailed{Err: err}
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	wg.Wait()
	close(errChan)

	if nErrors > 0 {
		err = &ErrAudioUploadFailed{Err: finalErr}
		log.WithError(err).Error("TranscodeAudio() too many errors")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(ofn),
	), nil
}

func ProcessImage(conf *Config, ifn string, resource, name string, opts *ImageOptions) (string, error) {
	defer os.Remove(ifn)

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var (
		ofn string
		tfn string
	)

	if name == "" {
		uuid := shortuuid.New()
		tfn = filepath.Join(p, fmt.Sprintf("%s.png", uuid))
		ofn = filepath.Join(p, fmt.Sprintf("%s.orig.png", uuid))
	} else {
		tfn = fmt.Sprintf("%s.png", filepath.Join(p, name))
		ofn = fmt.Sprintf("%s.orig.png", filepath.Join(p, name))
	}

	if _, err := copyFile(ifn, ofn); err != nil {
		log.WithError(err).Error("error copying input file")
		return "", err
	}

	f, err := os.Open(ifn)
	if err != nil {
		log.WithError(err).Error("error opening input file")
		return "", err
	}
	defer f.Close()

	img, _, err := imageorient.Decode(f)
	if err != nil {
		log.WithError(err).Error("imageorient.Decode failed")
		return "", err
	}

	g := gift.New()

	if opts != nil && opts.Resize {
		if opts.Width > 0 && opts.Height > 0 {
			g.Add(gift.ResizeToFit(opts.Width, opts.Height, gift.LanczosResampling))
		} else if (opts.Width+opts.Height > 0) && (opts.Height > 0 || img.Bounds().Size().X > opts.Width) {
			g.Add(gift.Resize(opts.Width, opts.Height, gift.LanczosResampling))
		}
	}

	newImg := image.NewRGBA(g.Bounds(img.Bounds()))

	g.Draw(newImg, img)

	of, err := os.OpenFile(tfn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening thumbnail file")
		return "", err
	}
	defer of.Close()

	if err := png.Encode(of, newImg); err != nil {
		log.WithError(err).Error("error encoding image")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(tfn),
	), nil
}

func TranscodeVideo(conf *Config, ifn string, resource, name string, opts *VideoOptions) (string, error) {
	defer os.Remove(ifn)

	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Errorf("error creating %s directory", resource)
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.mp4", uuid))
	} else {
		ofn = fmt.Sprintf("%s.mp4", filepath.Join(p, name))
	}

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return "", err
	}
	defer of.Close()

	wg := sync.WaitGroup{}

	TranscodeMP4 := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			"-y",
			"-i", ifn,
			"-r", "24",
			"-preset", "ultrafast",
			"-vcodec", "h264",
			"-acodec", "aac",
			"-strict", "-2",
			"-loglevel", "quiet",
			ofn,
		); err != nil {
			log.WithError(err).Error("error transcoding video")
			errs <- err
			return
		}
	}

	GeneratePoster := func(ctx context.Context, errs chan error) {
		defer wg.Done()

		// ffmpeg -ss 00:00:03.000 -i video.mp4 -y -vframes 1 -strict -loglevel quiet poster.png
		// ffmpeg i video.mp4 -y -vf thumbnail -t 3 -vframes 1 -strict -loglevel quiet poster.png
		if err := RunCmd(
			conf.TranscoderTimeout,
			"ffmpeg",
			"-i", ifn,
			"-y",
			"-vf", "thumbnail",
			"-t", "3",
			"-vframes", "1",
			"-strict", "-2",
			"-loglevel", "quiet",
			ReplaceExt(ofn, ".png"),
		); err != nil {
			log.WithError(err).Error("error generating video poster")
			errs <- err
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var finalErr error

	nErrors := 0
	errChan := make(chan error)

	wg.Add(2)

	go TranscodeMP4(ctx, errChan)
	go GeneratePoster(ctx, errChan)

	go func(ctx context.Context) {
		for {
			select {
			case err, ok := <-errChan:
				if !ok {
					return
				}
				nErrors++
				log.WithError(err).Errorf("TranscodeVideo() error")

				if errors.Is(err, &ErrCommandKilled{}) {
					finalErr = &ErrTranscodeTimeout{Err: err}
				} else {
					finalErr = &ErrTranscodeFailed{Err: err}
				}
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	wg.Wait()
	close(errChan)

	if nErrors > 0 {
		err = &ErrVideoUploadFailed{Err: finalErr}
		log.WithError(err).Error("TranscodeVideo() too many errors")
		return "", err
	}

	return fmt.Sprintf(
		"%s/%s/%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		resource, filepath.Base(ofn),
	), nil
}

func StoreUploadedImage(conf *Config, r io.Reader, resource, name string, opts *ImageOptions) (string, error) {
	fn, err := ReceiveImage(r)
	if err != nil {
		log.WithError(err).Error("error receiving image")
		return "", err
	}

	return ProcessImage(conf, fn, resource, name, opts)
}

func NormalizeFeedName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)
	return name
}

func IsTwtAuthentic(conf *Config, twt types.Twt) bool {
	hash := twt.Hash()
	twter := twt.Twter()
	tf, err := ValidateFeed(conf, twter.Nick, twter.URI)
	if err != nil {
		log.WithError(err).Errorf("error validating feed %s to authenticate twt %s", twter, hash)
		return false
	}
	for _, tfTwt := range tf.Twts() {
		if tfTwt.Hash() == twt.Hash() {
			return true
		}
	}
	return false
}

func ValidateFeed(conf *Config, nick, url string) (types.TwtFile, error) {
	var body io.ReadCloser

	if strings.HasPrefix(url, "gopher://") {
		res, err := RequestGopher(conf, url)
		if err != nil {
			log.WithError(err).Errorf("error fetching feed %s", url)
			return nil, err
		}
		body = res.Body
	} else {
		res, err := Request(conf, http.MethodGet, url, nil)
		if err != nil {
			log.WithError(err).Errorf("error fetching feed %s", url)
			return nil, err
		}
		if res.StatusCode != 200 {
			return nil, ErrBadRequest
		}
		body = res.Body
	}

	defer body.Close()

	limitedReader := &io.LimitedReader{R: body, N: conf.MaxFetchLimit}
	twter := types.Twter{Nick: nick, URI: url}
	tf, err := types.ParseFile(limitedReader, &twter)
	if err != nil {
		return nil, err
	}

	return tf, nil
}

func ValidateFeedName(path string, name string) error {
	if !validFeedName.MatchString(name) {
		return ErrInvalidFeedName
	}
	if len(name) > maxFeedNameLength {
		return ErrFeedNameTooLong
	}

	return nil
}

type URI struct {
	Type string
	Path string
}

func (u URI) IsZero() bool {
	return u.Type == "" && u.Path == ""
}

func (u URI) String() string {
	return fmt.Sprintf("%s://%s", u.Type, u.Path)
}

// TwtxtUserAgent ...
type TwtxtUserAgent interface {
	fmt.Stringer

	// IsPod returns true if the Twtxt client's User-Agent appears to be a Yarn.social pod (single or multi-user).
	IsPod() bool

	// PodBaseURL returns the base URL of the client's User-Agent if it appears to be a Yarn.social pod (single or multi-user).
	PodBaseURL() string

	// IsPublicURL returns true if the Twtxt client's User-Agent is from what appears to be the public internet.
	IsPublicURL() bool

	// Followers returns a list of followers for this client follows, in the case of a
	// single user agent, it is simply a list of itself, with a multi-user agent the
	// client (i.e: a `yarnd` pod) is aksed who followers the user/feed by requesting
	// the whoFollows resource
	Followers(conf *Config) types.Followers
}

// TwtxtUserAgent interface guards
var (
	_ TwtxtUserAgent = (*SingleUserAgent)(nil)
	_ TwtxtUserAgent = (*MultiUserAgent)(nil)
	_ TwtxtUserAgent = (*YarndUserAgent)(nil)
)

// twtxtUserAgent is a base class for both single and multi-user Twtxt User Agents.
type twtxtUserAgent struct {
	Client string
}

func (ua *twtxtUserAgent) IsPod() bool {
	return strings.HasPrefix(ua.Client, "yarnd/")
}

func (ua *twtxtUserAgent) podBaseURL(uri, relativeURLToTrim string) string {
	if !ua.IsPod() {
		return ""
	}

	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("error parsing User-Agent URL: %s", uri)
		return ""
	}

	// Throw away the trailing part of the URL to get the base URL for this
	// yarnd instance. It might serve from a subdirectory, so we cannot simply
	// cut off the complete path.
	rel, _ := url.Parse(relativeURLToTrim)
	return NormalizeURL(u.ResolveReference(rel).String())
}

func (ua *twtxtUserAgent) isPublicURL(uri, userAgent string) bool {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warn("error parsing User-Agent URL")
		return false
	}

	ips, err := net.LookupIP(u.Hostname())
	if err != nil {
		log.WithError(err).Warn("error looking up User-Agent IP")
		return false
	}

	if len(ips) == 0 {
		log.Warnf("User-Agent lookup failed for %s or has no resolvable IP", userAgent)
		return false
	}

	ip := ips[0]

	// 0.0.0.0 or ::
	if ip.IsUnspecified() {
		return false
	}

	// Link-local / Loopback
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}

	return !ipip.IsPrivate(ip)
}

// SingleUserAgent is a single Twtxt User Agent whether it be `tt`, `jenny` or a single-user `yarnd` client.
type SingleUserAgent struct {
	twtxtUserAgent
	Nick string
	URI  string
}

func (ua *SingleUserAgent) String() string {
	// <client>/<version> (+<source.url>; @<source.nick>)
	return fmt.Sprintf("%s (+%s; @%s)", ua.Client, ua.URI, ua.Nick)
}

func (ua *SingleUserAgent) PodBaseURL() string {
	// get rid of the trailing '/user/foo/twtxt.txt'
	return ua.podBaseURL(ua.URI, "../..")
}

func (ua *SingleUserAgent) IsPublicURL() bool {
	return ua.isPublicURL(ua.URI, ua.String())
}

func (ua *SingleUserAgent) Followers(conf *Config) types.Followers {
	return types.Followers{
		&types.Follower{
			Nick:       ua.Nick,
			URI:        ua.URI,
			LastSeenAt: time.Now(),
		},
	}
}

// MultiUserAgent is a multi-user Twtxt client, currently only `yarnd` is such a client.
type MultiUserAgent struct {
	twtxtUserAgent
	WhoFollowsURL string
	SupportURL    string
}

func (ua *MultiUserAgent) String() string {
	// <client>/<version> (~<whoFollowsURL>; contact=<supportURL>)
	return fmt.Sprintf("%s (~%s; contact=%s)", ua.Client, ua.WhoFollowsURL, ua.SupportURL)
}

func (ua *MultiUserAgent) PodBaseURL() string {
	// get rid of the trailing '/whoFollows?followers=42&token=abc'
	return ua.podBaseURL(ua.WhoFollowsURL, "./")
}

func (ua *MultiUserAgent) IsPublicURL() bool {
	return ua.isPublicURL(ua.WhoFollowsURL, ua.String())
}

func (ua *MultiUserAgent) Followers(conf *Config) types.Followers {
	var followers types.Followers

	headers := make(http.Header)
	headers.Set("Accept", "application/json")

	res, err := Request(conf, http.MethodGet, ua.WhoFollowsURL, headers)
	if err != nil {
		log.WithError(err).Errorf("error fetching whoFollows from %s", ua)
		return nil
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 2 {
		log.Errorf("HTTP %s response for whoFollows resource from %s", res.Status, ua)
		return nil
	}

	if ctype := res.Header.Get("Content-Type"); ctype != "" {
		mediaType, _, err := mime.ParseMediaType(ctype)
		if err != nil {
			log.WithError(err).Errorf("error parsing content type header '%s' for whoFollows resoruce from %s", ctype, ua)
			return nil
		}
		if mediaType != "application/json" {
			log.Errorf("non-JSON response '%s' for whoFollows resource from %s", ctype, ua)
			return nil
		}
	}

	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.WithError(err).Errorf("error reading response body for whoFollows resource from %s", ua)
		return nil
	}

	kv := make(map[string]string)
	if err := json.Unmarshal(data, &kv); err != nil {
		// XXX: This only exists for backwards compatibility in 0.11.x where this got changed.
		// TODOL Remove post 0.12.x and adhere to the spec (map of nick -> uri)
		if err := json.Unmarshal(data, &followers); err == nil {
			return followers
		}
		log.WithError(err).Errorf("error deserializing whoFollows response from %s", ua)
		return nil
	}
	for k, v := range kv {
		followers = append(followers, &types.Follower{Nick: k, URI: v, LastSeenAt: time.Now()})
	}

	return followers
}

// YarndUserAgent is a generic `yarnd` client.
type YarndUserAgent struct {
	twtxtUserAgent
	Name       string
	SupportURL string
}

func (ua *YarndUserAgent) String() string {
	// <client>/<version> (Pod: <name> Support: <supportURL>)
	return fmt.Sprintf("%s (Pod: %s Support: %s)", ua.Client, ua.Name, ua.SupportURL)
}

func (ua *YarndUserAgent) PodBaseURL() string {
	// get rid of the trailing '/support'
	return ua.podBaseURL(ua.SupportURL, "./")
}

func (ua *YarndUserAgent) IsPublicURL() bool {
	return ua.isPublicURL(ua.SupportURL, ua.String())
}

func (ua *YarndUserAgent) Followers(conf *Config) types.Followers {
	return nil
}

func ParseUserAgent(ua string) (TwtxtUserAgent, error) {
	if match := singleUserUARegex.FindStringSubmatch(ua); match != nil {
		return &SingleUserAgent{
			twtxtUserAgent: twtxtUserAgent{Client: match[1]},
			URI:            match[2],
			Nick:           match[3],
		}, nil
	}

	if match := multiUserUARegex.FindStringSubmatch(ua); match != nil {
		return &MultiUserAgent{
			twtxtUserAgent: twtxtUserAgent{Client: match[1]},
			WhoFollowsURL:  match[2],
			SupportURL:     match[3],
		}, nil
	}

	if match := yarndUserUARegex.FindStringSubmatch(ua); match != nil {
		return &YarndUserAgent{
			twtxtUserAgent: twtxtUserAgent{Client: match[1]},
			Name:           match[2],
			SupportURL:     match[3],
		}, nil
	}

	return nil, ErrInvalidUserAgent
}

func ParseURI(uri string) (*URI, error) {
	parts := strings.Split(uri, "://")
	if len(parts) == 2 {
		return &URI{Type: strings.ToLower(parts[0]), Path: parts[1]}, nil
	}
	return nil, fmt.Errorf("invalid uri: %s", uri)
}

func NormalizeUsername(username string) string {
	return strings.TrimSpace(strings.ToLower(username))
}

func NormalizeURL(url string) string {
	if url == "" {
		return ""
	}

	u, err := urlx.Parse(url)
	if err != nil {
		log.WithError(err).Errorf("NormalizeURL: error parsing url %s", url)
		return ""
	}
	if u.Scheme == "http" && strings.HasSuffix(u.Host, ":80") {
		u.Host = strings.TrimSuffix(u.Host, ":80")
	}
	if u.Scheme == "https" && strings.HasSuffix(u.Host, ":443") {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	u.User = nil
	u.Fragment = ""
	u.Path = strings.TrimSuffix(u.Path, "/")
	norm, err := urlx.Normalize(u)
	if err != nil {
		log.WithError(err).Errorf("error normalizing url %s", url)
		return ""
	}
	return norm
}

// RedirectRefererURL constructs a Redirect URL from the given Request URL
// and possibly Referer, if the Referer's Base URL matches the Pod's Base URL
// will return the Referer URL otherwise the defaultURL. This is primarily used
// to redirect a user from a successful /login back to the page they were on.
func RedirectRefererURL(r *http.Request, conf *Config, defaultURL string) string {
	referer := NormalizeURL(r.Header.Get("Referer"))
	if referer != "" && strings.HasPrefix(referer, conf.BaseURL) {
		return referer
	}

	return defaultURL
}

func HostnameFromURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("HostnameFromURL(): error parsing url: %s", uri)
		return uri
	}

	return u.Hostname()
}

func BaseFromURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("BaseFromURL(): error parsing url: %s", uri)
		return uri
	}

	u.Fragment = ""
	u.RawFragment = ""
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""

	return u.String()
}

func PrettyURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("PrettyURL(): error parsing url: %s", uri)
		return uri
	}

	return fmt.Sprintf("%s/%s", u.Hostname(), strings.TrimPrefix(u.EscapedPath(), "/"))
}

// IsSpecialFeed returns true if the feed is one of the special feeds
// an admin feed or automated feed.
func IsSpecialFeed(feed string) bool {
	return HasString(specialFeeds, strings.ToLower(feed))
}

// IsAdminUserFactory returns a function that returns true if the user provided
// is the configured pod administrator, false otherwise.
func IsAdminUserFactory(conf *Config) func(user *User) bool {
	return func(user *User) bool {
		return NormalizeUsername(conf.AdminUser) == NormalizeUsername(user.Username)
	}
}

func UserURL(url string) string {
	if strings.HasSuffix(url, "/twtxt.txt") {
		return strings.TrimSuffix(url, "/twtxt.txt")
	}
	return url
}

func URLForMedia(baseURL, name string) string {
	return fmt.Sprintf(
		"%s/media/%s",
		strings.TrimSuffix(baseURL, "/"),
		name,
	)
}

func URLForPage(baseURL, page string) string {
	return fmt.Sprintf(
		"%s/%s",
		strings.TrimSuffix(baseURL, "/"),
		page,
	)
}

func URLForTwt(baseURL, hash string) string {
	return fmt.Sprintf(
		"%s/twt/%s",
		strings.TrimSuffix(baseURL, "/"),
		hash,
	)
}

func URLForUser(baseURL, username string) string {
	return fmt.Sprintf(
		"%s/user/%s/twtxt.txt",
		strings.TrimSuffix(baseURL, "/"),
		username,
	)
}

func URLForAvatar(baseURL, username, avatarHash string) string {
	uri := fmt.Sprintf(
		"%s/user/%s/avatar",
		strings.TrimSuffix(baseURL, "/"),
		username,
	)
	if avatarHash != "" {
		uri += "#" + avatarHash
	}
	return uri
}

func URLForExternalProfile(conf *Config, nick, uri string) string {
	return fmt.Sprintf(
		"%s/external?uri=%s&nick=%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		uri, nick,
	)
}

func URLForExternalAvatar(conf *Config, uri string) string {
	return fmt.Sprintf(
		"%s/externalAvatar?uri=%s",
		strings.TrimSuffix(conf.BaseURL, "/"),
		uri,
	)
}

func GetConvLength(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt, u *User) int {
	return func(twt types.Twt, u *User) int {
		if subject, _ := GetTwtConvSubjectHash(cache, archive, twt); subject != "" {
			return len(cache.GetByUserView(u, fmt.Sprintf("subject:%s", subject), false))
		}
		return 0
	}
}

func GetForkLength(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt, u *User) int {
	return func(twt types.Twt, u *User) int {
		return len(cache.GetByUserView(u, fmt.Sprintf("subject:%s", fmt.Sprintf("(#%s)", twt.Hash())), false)) - 1
	}
}

func ExtractHashFromSubject(subject string) string {
	var hash string

	re := regexp.MustCompile(`\(#([a-z0-9]+)\)`)
	match := re.FindStringSubmatch(subject)
	if match != nil {
		hash = match[1]
	} else {
		re = regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
		match = re.FindStringSubmatch(subject)
		if match != nil {
			hash = match[2]
		}
	}

	return hash
}

func GetTwtConvSubjectHash(cache *Cache, archive Archiver, twt types.Twt) (string, string) {
	subject := twt.Subject().String()
	if subject == "" {
		return "", ""
	}

	hash := ExtractHashFromSubject(subject)
	if _, ok := cache.Lookup(hash); !ok && !archive.Has(hash) {
		return "", ""
	}

	return fmt.Sprintf("(#%s)", hash), hash
}

func URLForConvFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		if _, hash := GetTwtConvSubjectHash(cache, archive, twt); hash != "" {
			return fmt.Sprintf(
				"%s/conv/%s",
				strings.TrimSuffix(conf.BaseURL, "/"),
				hash,
			)
		}
		return ""
	}
}

func URLForForkFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		return fmt.Sprintf(
			"%s/conv/%s",
			strings.TrimSuffix(conf.BaseURL, "/"),
			twt.Hash(),
		)
	}
}

func URLForRootConvFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		if _, hash := GetTwtConvSubjectHash(cache, archive, twt); hash != "" && hash != twt.Hash() {
			return fmt.Sprintf(
				"%s/conv/%s",
				strings.TrimSuffix(conf.BaseURL, "/"),
				hash,
			)
		}
		return ""
	}
}

func URLForTag(baseURL, tag string) string {
	return fmt.Sprintf(
		"%s/search?tag=%s",
		strings.TrimSuffix(baseURL, "/"),
		tag,
	)
}

func URLForTask(baseURL, uuid string) string {
	return fmt.Sprintf(
		"%s/task/%s",
		strings.TrimSuffix(baseURL, "/"),
		uuid,
	)
}

func URLForWhoFollows(baseURL string, feed types.Feed, feedFollowers int) string {
	return fmt.Sprintf(
		"%s/whoFollows?followers=%d&token=%s",
		strings.TrimSuffix(baseURL, "/"),
		// Include the number of followers, so feed owners can use this as a vague
		// indicator to avoid refetching our Who Follows Resource if the number did
		// not change since they last checked their followers.
		feedFollowers,
		GenerateWhoFollowsToken(feed.URL),
	)
}

// SafeParseInt ...
func SafeParseInt(s string, d int) int {
	n, e := strconv.Atoi(s)
	if e != nil {
		return d
	}
	return n
}

// ValidateUsername validates the username before allowing it to be created.
// This ensures usernames match a defined pattern and that some usernames
// that are reserved are never used by users.
func ValidateUsername(username string) error {
	username = NormalizeUsername(username)

	if !validUsername.MatchString(username) {
		return ErrInvalidUsername
	}

	for _, reservedUsername := range reservedUsernames {
		if username == reservedUsername {
			return ErrReservedUsername
		}
	}

	if len(username) > maxUsernameLength {
		return ErrUsernameTooLong
	}

	return nil
}

// UnparseTwtFactory is the opposite of CleanTwt and ExpandMentions/ExpandTags
func UnparseTwtFactory(conf *Config) func(text string) string {
	isLocalURL := IsLocalURLFactory(conf)
	return func(text string) string {
		text = strings.ReplaceAll(text, "\u2028", "\n")

		re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
		return re.ReplaceAllStringFunc(text, func(match string) string {
			parts := re.FindStringSubmatch(match)
			prefix, nick, uri := parts[1], parts[2], parts[3]

			switch prefix {
			case "@":
				if uri != "" && !isLocalURL(uri) {
					u, err := url.Parse(uri)
					if err != nil {
						log.WithField("uri", uri).Warn("UnparseTwt(): error parsing uri")
						return match
					}
					return fmt.Sprintf("@%s@%s", nick, u.Hostname())
				}
				return fmt.Sprintf("@%s", nick)
			case "#":
				return fmt.Sprintf("#%s", nick)
			default:
				log.
					WithField("prefix", prefix).
					WithField("nick", nick).
					WithField("uri", uri).
					Warn("UnprocessTwt(): invalid prefix")
			}
			return match
		})
	}
}

// FilterTwts filters out Twts from users/feeds that a User has chosen to mute
func FilterTwts(user *User, twts types.Twts) (filtered types.Twts) {
	if user == nil {
		return twts
	}
	return user.Filter(twts)
}

// CleanTwt cleans a twt's text, replacing new lines with spaces and
// stripping surrounding spaces.
func CleanTwt(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\u2028")
	return text
}

// RenderAudio ...
func RenderAudio(conf *Config, uri, title string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		mp3URI := u.String()

		return fmt.Sprintf(`<audio controls="controls" title="%s">
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, title, mp3URI)
	}

	return fmt.Sprintf(`<audio controls="controls" title="%s">
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, title, uri)
}

// RenderImage ...
func RenderImage(conf *Config, uri, caption string) string {
	isLocalURL := IsLocalURLFactory(conf)

	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("error parsing uri: %s", uri)
		return ""
	}

	title := "Open to view original quality"

	var uuid string
	if matched, err := regexp.MatchString(`\/media\/[a-zA-Z0-9]+(\.png)?`, u.Path); err == nil && matched {
		uuid = strings.TrimSuffix(strings.Split(u.Path, "/")[2], ".png")
	} else {
		uuid = u.Path
	}

	if !isLocalURL(uri) {
		title = fmt.Sprintf(
			`%s on %s`,
			title, u.Hostname(),
		)
	}

	isCaption := ""
	if caption != "" {
		isCaption = fmt.Sprintf(
			`<div class="caption" data-target="%s">%s</div>`,
			uuid, caption,
		)
	}

	return fmt.Sprintf(
		`<div class="center-cropped caption-wrap">
			 <a class="img-orig-open" href="%s" title="%s" target="_blank">
				 %s
				 <img loading=lazy src="%s" data-target="%s" />
			 </a>
		 </div>
		 <dialog id="%s">
        <article class="modal-image">
          <img loading=lazy src="%s?full=1" />
          <footer><p>%s</p></footer>
        </article>
      </dialog>`,
		u.String(), title, isCaption, u.String(), uuid, uuid, u.String(), caption,
	)
}

// RenderVideo ...
func RenderVideo(conf *Config, uri, title string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		u.Path = ReplaceExt(u.Path, "")
		posterURI := u.String()

		return fmt.Sprintf(`<video controls playsinline preload="auto" title="%s" poster="%s">
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
  </video>`, title, posterURI, uri)
	}

	return fmt.Sprintf(`<video controls playsinline preload="auto" title="%s">
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
    </video>`, title, uri)
}

// PreprocessMedia ...
func PreprocessMedia(conf *Config, u *url.URL, title string) string {
	var html string

	// Normalize the domain name
	domain := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")

	whitelisted, local := conf.WhitelistedImage(domain)

	if whitelisted {
		if local {
			// Ensure all local links match our BaseURL scheme
			u.Scheme = conf.baseURL.Scheme
		} else {
			// Ensure all extern links are served over TLS
			u.Scheme = "https"
		}

		switch filepath.Ext(u.Path) {
		case ".mp4":
			html = RenderVideo(conf, u.String(), title)
		case ".mp3":
			html = RenderAudio(conf, u.String(), title)
		default:
			html = RenderImage(conf, u.String(), title)
		}
	} else {
		src := u.String()
		html = fmt.Sprintf(
			`<a href="%s" title="%s" target="_blank"><i class="external-image"></i></a>`,
			src, title,
		)
	}

	return html
}

func FormatForDateTime(t time.Time, timeFormat string) string {
	dateTimeFormat := ""

	if timeFormat == "" {
		timeFormat = "3:04PM"
	}

	dt := time.Since(t)

	if dt > YearAgo {
		dateTimeFormat = "Mon, Jan 2 %s 2006"
	} else if dt > MonthAgo {
		dateTimeFormat = "Mon, Jan 2 %s"
	} else if dt > WeekAgo {
		dateTimeFormat = "Mon, Jan 2 %s"
	} else if dt > DayAgo {
		dateTimeFormat = "Mon 2, %s"
	} else {
		dateTimeFormat = "%s"
	}

	return fmt.Sprintf(dateTimeFormat, timeFormat)
}

type URLProcessor struct {
	conf *Config
	user *User

	Images []string
}

func (p *URLProcessor) RenderNodeHook(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	// Ensure only whitelisted ![](url) images
	image, ok := node.(*ast.Image)
	if ok && entering {
		u, err := url.Parse(string(image.Destination))
		if err != nil {
			log.WithError(err).Warn("TwtFactory: error parsing url")
			return ast.GoToNext, false
		}

		html := PreprocessMedia(p.conf, u, string(image.Title))
		if (p.user != nil && p.user.DisplayImagesPreference == "gallery") || (p.user == nil && p.conf.DisplayImagesPreference == "gallery") {
			p.Images = append(p.Images, html)
		} else {
			_, _ = io.WriteString(w, html)
		}

		return ast.SkipChildren, true
	}

	span, ok := node.(*ast.HTMLSpan)
	if !ok {
		return ast.GoToNext, false
	}

	leaf := span.Leaf
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(leaf.Literal))
	if err != nil {
		log.WithError(err).Warn("error parsing HTMLSpan")
		return ast.GoToNext, false
	}

	// Ensure only whitelisted img src=(s) and fix non-secure links
	img := doc.Find("img")
	if img.Length() > 0 {
		src, ok := img.Attr("src")
		if !ok {
			return ast.GoToNext, false
		}

		alt, _ := img.Attr("alt")

		u, err := url.Parse(src)
		if err != nil {
			log.WithError(err).Warn("error parsing URL")
			return ast.GoToNext, false
		}

		html := PreprocessMedia(p.conf, u, alt)
		if (p.user != nil && p.user.DisplayImagesPreference == "gallery") || (p.user == nil && p.conf.DisplayImagesPreference == "gallery") {
			p.Images = append(p.Images, html)
		} else {
			_, _ = io.WriteString(w, html)
		}

		return ast.GoToNext, true
	}

	// Let it go! Lget it go!
	return ast.GoToNext, false
}

// FormatTwtFactory formats a twt into a valid HTML snippet
func FormatTwtFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt, u *User) template.HTML {
	return func(twt types.Twt, user *User) template.HTML {
		extensions := parser.NoIntraEmphasis | parser.FencedCode |
			parser.Autolink | parser.Strikethrough | parser.SpaceHeadings |
			parser.NoEmptyLineBeforeBlock | parser.HardLineBreak

		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.Smartypants | html.SmartypantsDashes | html.SmartypantsLatexDashes

		openLinksIn := conf.OpenLinksInPreference
		if user != nil {
			openLinksIn = user.OpenLinksInPreference
		}

		if strings.ToLower(openLinksIn) == "newwindow" {
			htmlFlags = htmlFlags | html.HrefTargetBlank
		}

		up := &URLProcessor{conf: conf, user: user}

		opts := html.RendererOptions{
			Flags:          htmlFlags,
			Generator:      "",
			RenderNodeHook: up.RenderNodeHook,
		}

		renderer := html.NewRenderer(opts)

		// copy alt to title if present.
		if cp, ok := twt.(*lextwt.Twt); ok {
			twt = cp.Clone()
			for _, m := range twt.Links() {
				if link, ok := m.(*lextwt.Link); ok {
					link.TextToTitle()
				}
			}
		}

		markdownInput := twt.FormatText(types.MarkdownFmt, conf)
		if subject, _ := GetTwtConvSubjectHash(cache, archive, twt); subject != "" {
			markdownInput = strings.ReplaceAll(markdownInput, subject, "")
			markdownInput = strings.TrimSpace(markdownInput)
		}

		md := []byte(markdownInput)
		maybeUnsafeHTML := markdown.ToHTML(md, mdParser, renderer)

		p := bluemonday.UGCPolicy()
		p.AllowAttrs("id").OnElements("dialog")
		p.AllowAttrs("id", "controls").OnElements("audio")
		p.AllowAttrs("id", "controls", "playsinline", "preload", "poster").OnElements("video")
		p.AllowAttrs("src", "type").OnElements("source")
		p.AllowAttrs("aria-label", "class", "data-target", "target").OnElements("a")
		p.AllowAttrs("class", "data-target").OnElements("i", "div")
		p.AllowAttrs("alt", "loading", "data-target").OnElements("a", "img")
		p.AllowAttrs("style").OnElements("a", "code", "img", "p", "pre", "span")
		html := p.SanitizeBytes(maybeUnsafeHTML)

		if len(up.Images) > 0 && ((user != nil && user.DisplayImagesPreference == "gallery") || (user == nil && conf.DisplayImagesPreference == "gallery")) {
			html = append(html, []byte(`<div class="image-gallery">`)...)
			html = append(html, []byte(strings.Join(up.Images, ""))...)
			html = append(html, []byte(`</div>`)...)
		}

		return template.HTML(html)
	}
}

func GetRootTwtFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt, u *User) types.Twt {
	return func(twt types.Twt, u *User) types.Twt {
		_, hash := GetTwtConvSubjectHash(cache, archive, twt)
		if hash == "" {
			return types.NilTwt
		}

		var rootTwt types.Twt

		if twt, inCache := cache.Lookup(hash); inCache {
			rootTwt = twt
		} else if twt, err := archive.Get(hash); err == nil {
			rootTwt = twt
		} else {
			log.Warnf("unable to get context for twt: %s", hash)
			return types.NilTwt
		}

		if u.HasMuted(rootTwt.Twter().URI) {
			return types.NilTwt
		}

		return rootTwt
	}
}

// FormatTwtContextFactory formats a twt's context into a valid HTML snippet
// A Twt's Context is defined as the content of the Root Twt of the Conversation
// rendered in plain text up to a maximu length with an elipsis if longer...
func FormatTwtContextFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt, u *User) template.HTML {
	getRootTwt := GetRootTwtFactory(conf, cache, archive)
	return func(twt types.Twt, u *User) template.HTML {
		rootTwt := getRootTwt(twt, u)
		if rootTwt.IsZero() {
			return template.HTML("")
		}

		renderHookProcessURLs := func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			// Ensure only whitelisted ![](url) images
			image, ok := node.(*ast.Image)
			if ok && entering {
				u, err := url.Parse(string(image.Destination))
				if err != nil {
					log.WithError(err).Warn("TwtFactory: error parsing url")
					return ast.GoToNext, false
				}

				src := u.String()
				html := fmt.Sprintf(
					`<a href="%s" alt="%s" target="_blank"><i class="external-image"></i></a>`,
					src, image.Title,
				)

				_, _ = io.WriteString(w, html)

				return ast.SkipChildren, true
			}

			span, ok := node.(*ast.HTMLSpan)
			if !ok {
				return ast.GoToNext, false
			}

			leaf := span.Leaf
			doc, err := goquery.NewDocumentFromReader(bytes.NewReader(leaf.Literal))
			if err != nil {
				log.WithError(err).Warn("error parsing HTMLSpan")
				return ast.GoToNext, false
			}

			// Ensure only whitelisted img src=(s) and fix non-secure links
			img := doc.Find("img")
			if img.Length() > 0 {
				src, ok := img.Attr("src")
				if !ok {
					return ast.GoToNext, false
				}

				alt, _ := img.Attr("alt")

				u, err := url.Parse(src)
				if err != nil {
					log.WithError(err).Warn("error parsing URL")
					return ast.GoToNext, false
				}

				html := fmt.Sprintf(
					`<a href="%s" alt="%s" target="_blank"><i class="external-image"></i></a>`,
					u, alt,
				)

				_, _ = io.WriteString(w, html)

				return ast.GoToNext, true
			}

			// Let it go! Lget it go!
			return ast.GoToNext, false
		}

		extensions := parser.NoExtensions
		mdParser := parser.NewWithExtensions(extensions)
		htmlFlags := html.FlagsNone

		openLinksIn := conf.OpenLinksInPreference
		if u != nil {
			openLinksIn = u.OpenLinksInPreference
		}

		if strings.ToLower(openLinksIn) == "newwindow" {
			htmlFlags = htmlFlags | html.HrefTargetBlank
		}

		opts := html.RendererOptions{
			Flags:          htmlFlags,
			Generator:      "",
			RenderNodeHook: renderHookProcessURLs,
		}

		renderer := html.NewRenderer(opts)

		markdownInput := rootTwt.FormatText(types.MarkdownFmt, conf)
		if subject, _ := GetTwtConvSubjectHash(cache, archive, rootTwt); subject != "" {
			markdownInput = strings.ReplaceAll(markdownInput, subject, "")
			markdownInput = strings.TrimSpace(markdownInput)
		}

		md := []byte(markdownInput)
		maybeUnsafeHTML := markdown.ToHTML(md, mdParser, renderer)

		p := bluemonday.UGCPolicy()
		p.AllowAttrs("id", "controls").OnElements("audio")
		p.AllowAttrs("id", "controls", "playsinline", "preload", "poster").OnElements("video")
		p.AllowAttrs("src", "type").OnElements("source")
		p.AllowAttrs("target").OnElements("a")
		p.AllowAttrs("class").OnElements("i")
		p.AllowAttrs("alt", "loading").OnElements("a", "img")
		p.AllowAttrs("style").OnElements("a", "code", "img", "p", "pre", "span")
		html := p.SanitizeBytes(maybeUnsafeHTML)

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
		if err != nil {
			log.WithError(err).Warn("error parsing twt context html")
			return template.HTML("")
		}

		firstParagraph, err := doc.Find("p").First().Html()
		if err != nil {
			log.WithError(err).Warn("error finding first paragraph for twt context")
			return template.HTML("")
		}

		return template.HTML(firstParagraph)
	}
}

// FormatMentionsAndTags turns `@<nick URL>` into `<a href="URL">@nick</a>`
// and `#<tag URL>` into `<a href="URL">#tag</a>` and a `!<hash URL>`
// into a `<a href="URL">!hash</a>`.
func FormatMentionsAndTags(conf *Config, text string, format TwtTextFormat) string {
	isLocalURL := IsLocalURLFactory(conf)
	re := regexp.MustCompile(`(@|#)<([^ ]+) *([^>]+)>`)
	return re.ReplaceAllStringFunc(text, func(match string) string {
		parts := re.FindStringSubmatch(match)
		prefix, nick, url := parts[1], parts[2], parts[3]

		if format == TextFmt {
			switch prefix {
			case "@":
				if isLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf("%s@%s", nick, conf.baseURL.Hostname())
				}
				return fmt.Sprintf("@%s", nick)
			default:
				return fmt.Sprintf("%s%s", prefix, nick)
			}
		}

		if format == HTMLFmt {
			switch prefix {
			case "@":
				if isLocalURL(url) && strings.HasSuffix(url, "/twtxt.txt") {
					return fmt.Sprintf(`<a href="%s">@%s</a>`, UserURL(url), nick)
				}
				return fmt.Sprintf(`<a href="%s">@%s</a>`, URLForExternalProfile(conf, nick, url), nick)
			default:
				return fmt.Sprintf(`<a href="%s">%s%s</a>`, url, prefix, nick)
			}
		}

		switch prefix {
		case "@":
			// Using (#) anchors to add the nick to URL for now. The Fluter app needs it since
			// 	the Markdown plugin doesn't include the link text that contains the nick in its onTap callback
			// https://github.com/flutter/flutter_markdown/issues/286
			return fmt.Sprintf(`[@%s](%s#%s)`, nick, url, nick)
		default:
			return fmt.Sprintf(`[%s%s](%s)`, prefix, nick, url)
		}
	})
}

// FormatRequest generates ascii representation of a request
func FormatRequest(r *http.Request) string {
	return fmt.Sprintf(
		"%s %v %s%v %v (%s)",
		r.RemoteAddr,
		r.Method,
		r.Host,
		r.URL,
		r.Proto,
		r.UserAgent(),
	)
}

func GetMediaNamesFromText(text string) []string {

	var mediaNames []string

	textSplit := strings.Split(text, "![](")

	for i, textSplitItem := range textSplit {
		if i > 0 {
			mediaEndIndex := strings.Index(textSplitItem, ")")
			mediaURL := textSplitItem[:mediaEndIndex]

			mediaURLSplit := strings.Split(mediaURL, "media/")
			for j, mediaURLSplitItem := range mediaURLSplit {
				if j > 0 {
					mediaPath := mediaURLSplitItem
					mediaNames = append(mediaNames, mediaPath)
				}
			}
		}
	}

	return mediaNames
}

func NewFeedLookup(conf *Config, db Store, user *User) types.FeedLookup {
	return types.FeedLookupFn(func(alias string) *types.Twter {
		for followedAs, followedURL := range user.Following {
			if strings.EqualFold(alias, followedAs) {
				u, err := url.Parse(followedURL)
				if err != nil {
					log.WithError(err).Warnf("error looking up follow alias %s for user %s", alias, user)
					return &types.Twter{}
				}
				parts := strings.SplitN(followedAs, "@", 2)

				if len(parts) == 2 && u.Hostname() == parts[1] {
					return &types.Twter{Nick: parts[0], URI: followedURL}
				}

				return &types.Twter{Nick: followedAs, URI: followedURL}
			}
		}

		username := NormalizeUsername(alias)
		if db.HasUser(username) || db.HasFeed(username) {
			return &types.Twter{Nick: username, URI: URLForUser(conf.BaseURL, username)}
		}

		return &types.Twter{}
	})
}

// TextWithEllipsis formats a a string with at most `maxLength` characters
// using an ellipsis (...) tto indicate more content...
func TextWithEllipsis(text string, maxLength int) string {
	if len(text) > maxLength {
		return fmt.Sprintf("%s ...", text[:maxLength])
	}
	return text
}

// GetArchivedFeeds ...
func GetArchivedFeeds(conf *Config, feed string) ([]string, error) {
	fns, err := filepath.Glob(filepath.Join(conf.Data, feedsDir, fmt.Sprintf("%s.*", feed)))
	if err != nil {
		return nil, err
	}
	sort.Strings(fns)
	return fns, nil
}

// ParseArchivedFeedIds ...
func ParseArchivedFeedIds(fns []string) ([]int, error) {
	var ids []int
	for _, fn := range fns {
		base := filepath.Base(fn)
		// Split the archived feed's base filename into 3 parts
		// <feed>.<id>[.<rest>]
		// This is so we can in future support compressed archives
		// like prologic.1.gz
		parts := strings.SplitN(base, ".", 3)
		if len(parts) < 2 {
			return nil, fmt.Errorf("unexpected number of parts in archived feed %s expected at least 2", fn)
		}
		// the <id> is always the 2nd part of the archived feed's filename
		idPart := parts[1]
		id, err := strconv.ParseInt(idPart, 10, 32)
		if err != nil {
			return nil, err
		}
		ids = append(ids, int(id))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ids)))
	return ids, nil
}

func RotateFeed(conf *Config, feed string) error {
	// Get old archived feeds
	archivedFeeds, err := GetArchivedFeeds(conf, feed)
	if err != nil {
		log.WithError(err).Error("error getting list of archived feeds")
		return fmt.Errorf("error getting list of archived feeds for %s: %w", feed, err)
	}

	// Parse archived feed ids
	archivedFeedIds, err := ParseArchivedFeedIds(archivedFeeds)
	if err != nil {
		log.WithError(err).Error("error parsing archived feed ids")
		return fmt.Errorf("error parsing archived feed ids for %s: %w", feed, err)
	}

	// Shuffle old archived feeds
	for _, archiveFeedID := range archivedFeedIds {
		oldFn := filepath.Join(conf.Data, feedsDir, fmt.Sprintf("%s.%d", feed, archiveFeedID))
		newFn := filepath.Join(conf.Data, feedsDir, fmt.Sprintf("%s.%d", feed, (archiveFeedID+1)))

		if FileExists(newFn) {
			return fmt.Errorf("error shuffling archived feed %s would override archived feed %s", oldFn, newFn)
		}

		if err := os.Rename(oldFn, newFn); err != nil {
			log.WithError(err).Errorf("error renaming archived feed %s -> %s", oldFn, newFn)
		}
	}

	oldFn := filepath.Join(conf.Data, feedsDir, feed)
	newFn := filepath.Join(conf.Data, feedsDir, feed+".0")

	if FileExists(newFn) {
		return fmt.Errorf("error rotating feed %s would override archived feed %s", feed, newFn)
	}

	if err := os.Rename(oldFn, newFn); err != nil {
		log.WithError(err).Errorf("error renaming active feed %s -> %s", oldFn, newFn)
	}

	return nil
}

// MemoryUsage returns information about thememory used by the runtime
func MemoryUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return fmt.Sprintf(
		"Alloc = %s TotalAlloc = %s Sys = %s NumGC = %d",
		humanize.Bytes(m.Alloc), humanize.Bytes(m.TotalAlloc),
		humanize.Bytes(m.Sys), m.NumGC,
	)
}
