package internal

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"html/template"
	"image"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	"github.com/PuerkitoBio/goquery"
	"github.com/audiolion/ipip"
	"github.com/disintegration/gift"
	"github.com/disintegration/imageorient"
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

	maxUsernameLength = 15 // avg 6 chars / 2 syllables per name commonly
	maxFeedNameLength = 25 // avg 4.7 chars per word in English so ~5 words

	requestTimeout = time.Second * 30

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
	twtxtBots = []string{
		statsBot,
		twtxtBot,
	}

	validFeedName  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
	validUsername  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]+$`)
	userAgentRegex = regexp.MustCompile(`(.+) \(\+(https?://\S+/\S+); @(\S+)\)`)

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

func GenerateAvatar(conf *Config, username string) (image.Image, error) {
	ig, err := identicon.New(conf.Name, 7, 4)
	if err != nil {
		log.WithError(err).Error("error creating identicon generator")
		return nil, err
	}

	ii, err := ig.Draw(username)
	if err != nil {
		log.WithError(err).Errorf("error generating avatar for %s", username)
		return nil, err
	}

	return ii.Image(AvatarResolution), nil
}

func ReplaceExt(fn, newExt string) string {
	oldExt := filepath.Ext(fn)
	return fmt.Sprintf("%s%s", strings.TrimSuffix(fn, oldExt), newExt)
}

func HasExternalAvatarChanged(conf *Config, twter types.Twter) bool {
	uri := twter.URL
	slug := Slugify(uri)
	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.cbf", slug))

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

func GetExternalAvatar(conf *Config, twter types.Twter) string {
	uri := twter.URL
	slug := Slugify(uri)
	fn := filepath.Join(conf.Data, externalDir, fmt.Sprintf("%s.png", slug))

	//
	// Use an already cached Avatar (unless there's a new one!)
	//

	if FileExists(fn) && !HasExternalAvatarChanged(conf, twter) {
		return URLForExternalAvatar(conf, uri)
	}

	// Use the Avatar advertised in the feed
	if twter.Avatar != "" {
		u, err := url.Parse(twter.Avatar)
		if err != nil {
			log.WithError(err).Errorf("error parsing avatar url %s", twter.Avatar)
			return ""
		}

		opts := &ImageOptions{Resize: true, Width: AvatarResolution, Height: AvatarResolution}
		if _, err := DownloadImage(conf, u.String(), externalDir, slug, opts); err != nil {
			log.WithError(err).Errorf("error downloading external avatar: %s", u)
			return ""
		}
		if err := os.WriteFile(ReplaceExt(fn, ".cbf"), []byte(FastHashString(u.String())), 0644); err != nil {
			log.WithError(err).Warnf("error writing avatar cbf for %s", slug)
		}
		return URLForExternalAvatar(conf, uri)
	}

	//
	// Auto-generate one
	//

	log.Warnf("unable to find a suitable avatar for %s generating one", uri)

	img, err := GenerateAvatar(conf, twter.DomainNick())
	if err != nil {
		log.WithError(err).Errorf("error generating avatar for %s", twter)
		return ""
	}

	of, err := os.OpenFile(fn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
		return ""
	}
	defer of.Close()

	if err := png.Encode(of, img); err != nil {
		log.WithError(err).Error("error encoding image")
		return ""
	}

	if avatarHash, err := FastHashFile(fn); err == nil {
		if err := os.WriteFile(ReplaceExt(fn, ".cbf"), []byte(avatarHash), 0644); err != nil {
			log.WithError(err).Warnf("error writing avatar cbf for %s", slug)
		}
	} else {
		log.WithError(err).Warnf("error computing avatar cbf for %s", fn)
	}

	return URLForExternalAvatar(conf, uri)
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
		Timeout: requestTimeout,
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

func IsExternalFeedFactory(conf *Config) func(url string) bool {
	baseURL := NormalizeURL(conf.BaseURL)
	externalBaseURL := fmt.Sprintf("%s/external", strings.TrimSuffix(baseURL, "/"))

	return func(url string) bool {
		if NormalizeURL(url) == "" {
			return false
		}
		return strings.HasPrefix(NormalizeURL(url), externalBaseURL)
	}
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
	if tf != nil {
		defer tf.Close()
	}
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

func TranscodeAudio(conf *Config, ifn string, resource, name string, opts *AudioOptions) (string, error) {
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
	p := filepath.Join(conf.Data, resource)
	if err := os.MkdirAll(p, 0755); err != nil {
		log.WithError(err).Error("error creating avatars directory")
		return "", err
	}

	var ofn string

	if name == "" {
		uuid := shortuuid.New()
		ofn = filepath.Join(p, fmt.Sprintf("%s.png", uuid))
	} else {
		ofn = fmt.Sprintf("%s.png", filepath.Join(p, name))
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
		} else {
			log.Warnf(
				"not resizing image with bounds %s to %dx%d",
				img.Bounds(), opts.Width, opts.Height,
			)
		}
	}

	newImg := image.NewRGBA(g.Bounds(img.Bounds()))

	g.Draw(newImg, img)

	of, err := os.OpenFile(ofn, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.WithError(err).Error("error opening output file")
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
		resource, strings.TrimSuffix(filepath.Base(ofn), filepath.Ext(ofn)),
	), nil
}

func TranscodeVideo(conf *Config, ifn string, resource, name string, opts *VideoOptions) (string, error) {
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

func ValidateFeed(conf *Config, nick, url string) (types.Twter, error) {
	var body io.ReadCloser

	if strings.HasPrefix(url, "gopher://") {
		res, err := RequestGopher(conf, url)
		if err != nil {
			log.WithError(err).Errorf("error fetching feed %s", url)
			return types.Twter{}, err
		}
		body = res.Body
	} else {
		res, err := Request(conf, http.MethodGet, url, nil)
		if err != nil {
			log.WithError(err).Errorf("error fetching feed %s", url)
			return types.Twter{}, err
		}
		if res.StatusCode != 200 {
			return types.Twter{}, ErrBadRequest
		}
		body = res.Body
	}

	defer body.Close()

	limitedReader := &io.LimitedReader{R: body, N: conf.MaxFetchLimit}
	twter := types.Twter{Nick: nick, URL: url}
	tf, err := types.ParseFile(limitedReader, twter)
	if err != nil {
		return types.Twter{}, err
	}

	return tf.Twter(), nil
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

type TwtxtUserAgent struct {
	Client string
	Nick   string
	URL    string
}

func (ua TwtxtUserAgent) IsPublicURL() bool {
	u, err := url.Parse(ua.URL)
	if err != nil {
		log.WithError(err).Warn("error parsing User-Agent URL")
		return false
	}

	hostname := u.Hostname()

	ips, err := net.LookupIP(hostname)
	if err != nil {
		log.WithError(err).Warn("error looking up User-Agent IP")
		return false
	}

	if len(ips) == 0 {
		log.Warn("error User-Agent lookup failed or has no resolvable IP")
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

func DetectFollowerFromUserAgent(ua string) (*TwtxtUserAgent, error) {
	match := userAgentRegex.FindStringSubmatch(ua)
	if match == nil {
		return nil, ErrInvalidUserAgent
	}

	return &TwtxtUserAgent{
		Client: match[1],
		URL:    match[2],
		Nick:   match[3],
	}, nil
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

func PrettyURL(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		log.WithError(err).Warnf("PrettyURL(): error parsing url: %s", uri)
		return uri
	}

	return fmt.Sprintf("%s/%s", u.Hostname(), strings.TrimPrefix(u.EscapedPath(), "/"))
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

func URLForConvFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		subject := twt.Subject().String()
		if subject == "" {
			return ""
		}

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

		if _, ok := cache.Lookup(hash); !ok && !archive.Has(hash) {
			return ""
		}

		return fmt.Sprintf(
			"%s/conv/%s",
			strings.TrimSuffix(conf.BaseURL, "/"),
			hash,
		)
	}
}

func URLForRootConvFactory(conf *Config, cache *Cache, archive Archiver) func(twt types.Twt) string {
	return func(twt types.Twt) string {
		subject := twt.Subject().String()
		if subject == "" {
			return ""
		}

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

		if _, ok := cache.Lookup(hash); !ok && !archive.Has(hash) {
			return ""
		}

		if twt.Hash() == hash {
			return ""
		}

		return fmt.Sprintf(
			"%s/conv/%s",
			strings.TrimSuffix(conf.BaseURL, "/"),
			hash,
		)
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
		GenerateToken(feed.URL),
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
func RenderAudio(conf *Config, uri string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		mp3URI := u.String()

		return fmt.Sprintf(`<audio controls="controls">
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, mp3URI)
	}

	return fmt.Sprintf(`<audio controls="controls">
  <source type="audio/mp3" src="%s"></source>
  Your browser does not support the audio element.
</audio>`, uri)
}

// RenderVideo ...
func RenderVideo(conf *Config, uri string) string {
	isLocalURL := IsLocalURLFactory(conf)

	if isLocalURL(uri) {
		u, err := url.Parse(uri)
		if err != nil {
			log.WithError(err).Warnf("error parsing uri: %s", uri)
			return ""
		}

		u.Path = ReplaceExt(u.Path, "")
		posterURI := u.String()

		return fmt.Sprintf(`<video controls playsinline preload="auto" poster="%s">
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
  </video>`, posterURI, uri)
	}

	return fmt.Sprintf(`<video controls playsinline preload="auto">
    <source type="video/mp4" src="%s" />
    Your browser does not support the video element.
    </video>`, uri)
}

// PreprocessMedia ...
func PreprocessMedia(conf *Config, u *url.URL, alt string) string {
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
			html = RenderVideo(conf, u.String())
		case ".mp3":
			html = RenderAudio(conf, u.String())
		default:
			src := u.String()
			html = fmt.Sprintf(`<img alt="%s" src="%s" loading=lazy>`, alt, src)
		}
	} else {
		src := u.String()
		html = fmt.Sprintf(
			`<a href="%s" alt="%s" target="_blank"><i class="icss-image"></i></a>`,
			src, alt,
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

// FormatTwtFactory formats a twt into a valid HTML snippet
func FormatTwtFactory(conf *Config) func(twt types.Twt) template.HTML {
	return func(twt types.Twt) template.HTML {
		renderHookProcessURLs := func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
			// Ensure only whitelisted ![](url) images
			image, ok := node.(*ast.Image)
			if ok && entering {
				u, err := url.Parse(string(image.Destination))
				if err != nil {
					log.WithError(err).Warn("TwtFactory: error parsing url")
					return ast.GoToNext, false
				}

				html := PreprocessMedia(conf, u, string(image.Title))

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

				html := PreprocessMedia(conf, u, alt)

				_, _ = io.WriteString(w, html)

				return ast.GoToNext, true
			}

			// Let it go! Lget it go!
			return ast.GoToNext, false
		}

		extensions := parser.NoIntraEmphasis | parser.FencedCode |
			parser.Autolink | parser.Strikethrough | parser.SpaceHeadings |
			parser.NoEmptyLineBeforeBlock

		mdParser := parser.NewWithExtensions(extensions)

		htmlFlags := html.Smartypants | html.SmartypantsDashes | html.SmartypantsLatexDashes
		opts := html.RendererOptions{
			Flags:          htmlFlags,
			Generator:      "",
			RenderNodeHook: renderHookProcessURLs,
		}

		renderer := html.NewRenderer(opts)

		md := []byte(twt.FormatText(types.HTMLFmt, conf))
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

		return template.HTML(html)
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
					return &types.Twter{Nick: parts[0], URL: followedURL}
				}

				return &types.Twter{Nick: followedAs, URL: followedURL}
			}
		}

		username := NormalizeUsername(alias)
		if db.HasUser(username) || db.HasFeed(username) {
			return &types.Twter{Nick: username, URL: URLForUser(conf.BaseURL, username)}
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
