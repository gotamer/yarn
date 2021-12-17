package internal

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"git.mills.io/yarnsocial/yarn/types"
	"github.com/julienschmidt/httprouter"
	"github.com/vcraescu/go-paginator"
	"github.com/vcraescu/go-paginator/adapter"
)

// SearchHandler ...
func (s *Server) SearchHandler() httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		ctx := NewContext(s, r)
		ctx.Translate(s.translator)

		tag := r.URL.Query().Get("tag")

		if tag == "" {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorNoTag")
			s.render("error", w, ctx)
		}

		var twts types.Twts

		// If the tag matches a Twt by hash?
		// Add it to the list of twts
		if twt, ok := s.cache.Lookup(tag); ok {
			twts = append(twts, twt)
		} else {
			// If the twt is not in the cache look for it in the archive
			if twt, err := s.archive.Get(tag); err == nil {
				twts = append(twts, twt)
			}
		}

		twts = append(twts, s.cache.GetByUserView(ctx.User, fmt.Sprintf("tag:%s", strings.ToLower(tag)), false)...)
		sort.Sort(sort.Reverse(twts))

		var pagedTwts types.Twts

		page := SafeParseInt(r.FormValue("p"), 1)
		pager := paginator.New(adapter.NewSliceAdapter(twts), s.config.TwtsPerPage)
		pager.SetPage(page)

		if err := pager.Results(&pagedTwts); err != nil {
			ctx.Error = true
			ctx.Message = s.tr(ctx, "ErrorLoadingSearch")
			s.render("error", w, ctx)
			return
		}

		ctx.Twts = FilterTwts(ctx.User, pagedTwts)
		ctx.Pager = &pager

		ctx.SearchQuery = fmt.Sprintf("tag=%s", tag)

		s.render("search", w, ctx)
	}
}
