package internal

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"math"
	"path/filepath"
	"strings"
	text_template "text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	humanize "github.com/dustin/go-humanize"
	sync "github.com/sasha-s/go-deadlock"
	log "github.com/sirupsen/logrus"
)

const (
	baseTemplate     = "base.html"
	partialsTemplate = "partials.html"
	baseName         = "base"
)

var customTimeMagnitudes = []humanize.RelTimeMagnitude{
	{D: time.Second, Format: "now", DivBy: time.Second},
	{D: time.Minute, Format: "%ds %s", DivBy: time.Second},
	{D: time.Hour, Format: "%dm %s", DivBy: time.Minute},
	{D: humanize.Day, Format: "%dh %s", DivBy: time.Hour},
	{D: humanize.Week, Format: "%dd %s", DivBy: humanize.Day},
	{D: humanize.Year, Format: "%dw %s", DivBy: humanize.Week},
	{D: humanize.LongTime, Format: "%dy %s", DivBy: humanize.Year},
	{D: math.MaxInt64, Format: "a long while %s", DivBy: 1},
}

func CustomRelTime(a, b time.Time, albl, blbl string) string {
	return humanize.CustomRelTime(a, b, albl, blbl, customTimeMagnitudes)
}

func CustomTime(then time.Time) string {
	return CustomRelTime(then, time.Now(), "ago", "from now")
}

type TemplateManager struct {
	sync.RWMutex

	debug   bool
	tmplFS  fs.FS
	tmplMap map[string]*template.Template
	funcMap template.FuncMap
}

func NewTemplateManager(conf *Config, translator *Translator, cache *Cache, archive Archiver) (*TemplateManager, error) {
	tmplMap := make(map[string]*template.Template)

	funcMap := sprig.FuncMap()

	funcMap["time"] = CustomTime
	funcMap["hostnameFromURL"] = HostnameFromURL
	funcMap["baseFromURL"] = BaseFromURL
	funcMap["prettyURL"] = PrettyURL
	funcMap["isLocalURL"] = IsLocalURLFactory(conf)
	funcMap["formatTwt"] = FormatTwtFactory(conf, cache, archive)
	funcMap["unparseTwt"] = UnparseTwtFactory(conf)
	funcMap["formatTwtContext"] = FormatTwtContextFactory(conf, cache, archive)
	funcMap["getRootTwt"] = GetRootTwtFactory(conf, cache, archive)
	funcMap["formatForDateTime"] = FormatForDateTime
	funcMap["urlForConv"] = URLForConvFactory(conf, cache, archive)
	funcMap["urlForFork"] = URLForForkFactory(conf, cache, archive)
	funcMap["urlForRootConv"] = URLForRootConvFactory(conf, cache, archive)
	funcMap["getConvLength"] = GetConvLength(conf, cache, archive)
	funcMap["getForkLength"] = GetForkLength(conf, cache, archive)
	funcMap["isAdminUser"] = IsAdminUserFactory(conf)
	funcMap["isSpecialFeed"] = IsSpecialFeed
	funcMap["isFeatureEnabled"] = func(name string) bool {
		return IsFeatureEnabled(conf.Features, name)
	}

	funcMap["html"] = func(text string) template.HTML { return template.HTML(text) }
	funcMap["tr"] = func(ctx *Context, msgid string, data ...interface{}) string {
		return translator.Translate(ctx, msgid, data...)
	}

	m := &TemplateManager{
		debug:   conf.Debug,
		tmplFS:  conf.TemplatesFS(),
		tmplMap: tmplMap,
		funcMap: funcMap,
	}

	if err := m.LoadTemplates(); err != nil {
		log.WithError(err).Error("error loading templates")
		return nil, fmt.Errorf("error loading templates: %w", err)
	}

	return m, nil
}

func (m *TemplateManager) LoadTemplates() error {
	m.Lock()
	defer m.Unlock()

	err := fs.WalkDir(m.tmplFS, ".", func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			log.WithError(err).Error("error walking templates")
			return fmt.Errorf("error walking templates: %w", err)
		}

		fname := info.Name()
		if !info.IsDir() && filepath.Base(path) != baseTemplate {
			// Skip partials.html and also editor swap files, to improve the development
			// cycle. Editors often add suffixes to their swap files, e.g "~" or ".swp"
			// (Vim) and those files are not parsable as templates, causing panics.
			if fname == partialsTemplate || !strings.HasSuffix(fname, ".html") {
				return nil
			}

			name := strings.TrimSuffix(fname, filepath.Ext(fname))
			t := template.New(name).Option("missingkey=zero")
			t.Funcs(m.funcMap)

			if f, err := fs.ReadFile(m.tmplFS, path); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return fmt.Errorf("error parsing template %s: %w", path, err)
			}

			if f, err := fs.ReadFile(m.tmplFS, partialsTemplate); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return fmt.Errorf("error parsing partials template %s: %w", partialsTemplate, err)
			}

			if f, err := fs.ReadFile(m.tmplFS, baseTemplate); err == nil {
				template.Must(t.Parse(string(f)))
			} else {
				return fmt.Errorf("error parsing base template %s: %w", baseTemplate, err)
			}

			m.tmplMap[name] = t
		}
		return nil
	})
	if err != nil {
		log.WithError(err).Error("error loading templates")
		return fmt.Errorf("error loading templates: %w", err)
	}
	return nil
}

func (m *TemplateManager) Add(name string, template *template.Template) {
	m.Lock()
	defer m.Unlock()

	m.tmplMap[name] = template
}

func (m *TemplateManager) Exec(name string, ctx *Context) (io.WriterTo, error) {
	if m.debug {
		log.Debug("reloading templates in debug mode...")
		if err := m.LoadTemplates(); err != nil {
			log.WithError(err).Error("error reloading templates")
			return nil, fmt.Errorf("error reloading templates: %w", err)
		}
	}

	m.RLock()
	template, ok := m.tmplMap[name]
	m.RUnlock()

	if !ok {
		log.WithField("name", name).Errorf("template not found")
		return nil, fmt.Errorf("no such template: %s", name)
	}

	if ctx == nil {
		ctx = &Context{}
	}

	buf := bytes.NewBuffer([]byte{})
	err := template.ExecuteTemplate(buf, baseName, ctx)
	if err != nil {
		log.WithError(err).WithField("name", name).Errorf("error executing template")
		return nil, fmt.Errorf("error executing template %s: %w", name, err)
	}

	return buf, nil
}

// RenderHTML ...
func RenderHTML(tpl string, ctx *Context) (string, error) {
	t := template.Must(template.New("tpl").Parse(tpl))
	buf := bytes.NewBuffer([]byte{})
	err := t.Execute(buf, ctx)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// RenderPlainText ...
func RenderPlainText(tpl string, ctx *Context) (string, error) {
	t := text_template.Must(text_template.New("tpl").Parse(tpl))
	buf := bytes.NewBuffer([]byte{})
	err := t.Execute(buf, ctx)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
