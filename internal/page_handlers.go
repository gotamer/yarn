package internal

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/james4k/fmatter"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	pagesDir = "pages"

	markdownErrorPageTemplate = `# Error loading page

An error occurred trying to read the source of this page {{ .Page }}
Please try again later...`
)

//go:embed pages/*.md
var builtinPages embed.FS

type FrontMatter struct {
	Title       string
	Description string
}

// PageHandler ...
func (s *Server) PageHandler(page string) httprouter.Handle {
	var markdownTemplate string

	name := filepath.Join(pagesDir, fmt.Sprintf("%s.md", page))
	fn := filepath.Join(s.config.Data, name)

	if FileExists(fn) {
		if data, err := os.ReadFile(fn); err != nil {
			log.WithError(err).Errorf("error reading custom page %s", page)
			markdownTemplate = markdownErrorPageTemplate
		} else {
			markdownTemplate = string(data)
		}
	} else {
		if data, err := builtinPages.ReadFile(name); err != nil {
			log.WithError(err).Errorf("error reading custom page %s", page)
			markdownTemplate = markdownErrorPageTemplate
		} else {
			markdownTemplate = string(data)
		}
	}

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		ctx := NewContext(s, r)

		markdownContent, err := RenderHTML(markdownTemplate, ctx)
		if err != nil {
			log.WithError(err).Errorf("error rendering page %s", name)
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorRenderingPage")
			s.render("error", w, ctx)
			return
		}

		var frontmatter FrontMatter
		content, err := fmatter.Read([]byte(markdownContent), &frontmatter)
		if err != nil {
			log.WithError(err).Error("error parsing front matter")
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingPage")
			s.render("error", w, ctx)
			return
		}

		extensions := parser.CommonExtensions | parser.AutoHeadingIDs
		p := parser.NewWithExtensions(extensions)

		htmlFlags := html.CommonFlags
		opts := html.RendererOptions{
			Flags:     htmlFlags,
			Generator: "",
		}
		renderer := html.NewRenderer(opts)

		html := markdown.ToHTML(content, p, renderer)

		var title string

		if frontmatter.Title != "" {
			title = frontmatter.Title
		} else {
			title = strings.Title(name)
		}
		ctx.Title = title
		ctx.Meta.Description = frontmatter.Description

		ctx.Page = name
		ctx.Content = template.HTML(html)

		s.render("page", w, ctx)
	}
}
