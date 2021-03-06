package lextwt

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"git.mills.io/yarnsocial/yarn/types"
)

// Parser
type parser struct {
	l       *lexer
	curTok  Token
	curPos  int
	nextTok Token
	nextPos int

	twter *types.Twter

	lit   []rune
	frame []int

	errs []error

	skipSubject bool
}

func NewParser(l *lexer) *parser {
	p := &parser{
		l: l,

		// As tokens are read they are appended here and stored in the resulting Elem.
		// The buffer is here so text can be recovered in the event a mention/tag fails to fully parse
		// and to limit memory allocs.
		lit: make([]rune, 0, 1024),
	}

	// Prime the parser queue
	p.next()
	p.next()

	return p
}

func (p *parser) SetTwter(twter *types.Twter) {
	p.twter = twter
}

// ParseLine from tokens
// Forms parsed:
//   #... -> ParseComment
//   [digit]... -> ParseTwt
func (p *parser) ParseLine() Line {
	var e Line

	switch p.curTok.Type {
	case TokHASH:
		e = p.ParseComment()
	case TokNUMBER:
		e = p.ParseTwt()
		if e.IsNil() {
			p.nextLine()
		}
	default:
		p.nextLine()
	}
	if !(p.expect(TokNL) || p.expect(TokEOF)) {
		return nil
	}
	p.next()
	p.clear()

	return e
}

// ParseComment from tokens
// Forms parsed:
//   # comment
//   # key = value
func (p *parser) ParseComment() *Comment {
	if !p.curTokenIs(TokHASH) {
		return nil
	}

	p.append(p.curTok.Literal...)

	isValidKey := true
	inValue := false
	var label string
	var value []rune
	for !p.nextTokenIs(TokNL, TokEOF) {
		p.next()
		p.append(p.curTok.Literal...)

		if !isValidKey {
			// Since the key is not valid for sure we know we cannot construct
			// any metadata field and save any further parsing efforts.
			continue
		}

		if inValue {
			// everything is allowed in values
			value = append(value, p.curTok.Literal...)
		} else {
			// attempt to parse a key

			if p.curTokenIs(TokSPACE) {
				// ignore surrounding whitespace in key
				continue
			}

			if !p.curTokenIs(TokSTRING) {
				// reject invalid characters such as '#' and '=' in key and
				// also empty keys
				isValidKey = false
				continue
			}

			if p.peekTokenIs(TokEQUAL) {
				label = strings.TrimSpace(string(p.curTok.Literal))
				inValue = true
				p.next()
				p.append(p.curTok.Literal...)
			}
		}
	}

	val := strings.TrimSpace(string(value))
	if val == "" {
		// Empty values are not allowed, so don't make a metadata field out of
		// this commit. Comments with both empty key and value are no metadata.
		label = ""
	}
	return NewCommentValue(p.Literal(), label, val)
}

// ParseTwt from tokens
// Forms parsed:
//   [Date]\t... -> ParseElem (will consume all elems till end of line/file.)
func (p *parser) ParseTwt() *Twt {
	twt := &Twt{twter: p.twter}

	if !p.expect(TokNUMBER) {
		return nil
	}
	twt.pos = p.curPos
	twt.dt = p.ParseDateTime()
	if twt.dt == nil {
		return nil
	}
	p.push()

	if !p.expect(TokTAB) {
		return nil
	}
	p.append(p.curTok.Literal...)
	p.push()

	p.next()

	p.skipSubject = false
	for elem := p.ParseElem(); elem != nil; elem = p.ParseElem() {
		p.push()
		twt.append(elem)
	}

	return twt
}

// ParseDateTime from tokens
// Forms parsed:
//   YYYY-MM-DD'T'HH:mm[:ss[.nnnnnnnn]]('Z'|('+'|'-')th[:tm])
//   YYYY = year, MM = month, DD = day, HH = 24hour, mm = minute, ss = sec, nnnnnnnn = nsec, th = timezone hour, tm = timezone minute
func (p *parser) ParseDateTime() *DateTime {
	var ok bool
	var year, month, day, hour, min, sec, nsec, sign, tzhour, tzmin int
	loc := time.UTC

	// Year
	if !p.expectLen(p.curTok, 4, 4) {
		return nil
	}
	p.append(p.curTok.Literal...)
	if year, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.append(p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Month
	if !p.expectLen(p.curTok, 2, 2) {
		return nil
	}
	p.append(p.curTok.Literal...)
	if month, ok = p.parseDigit(); !ok {
		return nil
	}

	// Hyphen
	p.append(p.curTok.Literal...)
	if !(p.expect(TokHYPHEN) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Day
	if !p.expectLen(p.curTok, 2, 2) {
		return nil
	}
	p.append(p.curTok.Literal...)
	if day, ok = p.parseDigit(); !ok {
		return nil
	}

	// T
	p.append(p.curTok.Literal...)
	if !(p.expect(TokT) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Hour
	if !p.expectLen(p.curTok, 1, 2) {
		return nil
	}
	p.append(p.curTok.Literal...)
	if hour, ok = p.parseDigit(); !ok {
		return nil
	}

	// Colon
	p.append(p.curTok.Literal...)
	if !(p.expect(TokCOLON) && p.expectNext(TokNUMBER)) {
		return nil
	}

	// Minute
	if !p.expectLen(p.curTok, 2, 2) {
		return nil
	}
	p.append(p.curTok.Literal...)
	if min, ok = p.parseDigit(); !ok {
		return nil
	}

	// Optional Second
	if p.curTokenIs(TokCOLON) {
		p.append(p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// Second
		if !p.expectLen(p.curTok, 2, 2) {
			return nil
		}
		p.append(p.curTok.Literal...)
		if sec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// Optional NSec
	if p.curTokenIs(TokDOT) {
		p.append(p.curTok.Literal...)
		if !p.expectNext(TokNUMBER) {
			return nil
		}

		// NSecond
		if !p.expectLen(p.curTok, 1, 9) {
			return nil
		}
		p.append(p.curTok.Literal...)
		if nsec, ok = p.parseDigit(); !ok {
			return nil
		}
	}

	// UTC Timezone
	if p.curTokenIs(TokZ) {
		p.append(p.curTok.Literal...)
		p.next()

	} else if p.curTokenIs(TokPLUS) || p.curTokenIs(TokHYPHEN) {
		sign = 1
		tzfmt := "UTC+%02d%02d"

		p.append(p.curTok.Literal...)
		if p.curTokenIs(TokHYPHEN) {
			tzfmt = "UTC-%02d%02d"
			sign = -1
		}
		// TZHour
		if !p.expectNext(TokNUMBER) {
			return nil
		}
		if !p.expectLen(p.curTok, 2, 4) {
			return nil
		}
		p.append(p.curTok.Literal...)
		if tzhour, ok = p.parseDigit(); !ok {
			return nil
		}

		if tzhour > 24 {
			tzmin = tzhour % 100
			tzhour = tzhour / 100
		}

		// Optional tzmin with colon
		if p.curTokenIs(TokCOLON) {
			p.append(p.curTok.Literal...)
			if !p.expectNext(TokNUMBER) {
				return nil
			}

			// TZMin
			if !p.expectLen(p.curTok, 2, 2) {
				return nil
			}
			p.append(p.curTok.Literal...)
			if tzmin, ok = p.parseDigit(); !ok {
				return nil
			}
		}

		loc = time.FixedZone(fmt.Sprintf(tzfmt, tzhour, tzmin), sign*tzhour*3600+tzmin*60)
	}

	return &DateTime{dt: time.Date(year, time.Month(month), day, hour, min, sec, nsec, loc), lit: p.Literal()}
}

// ParseElem from tokens
// Forms parsed:
//   #... -> ParseTag
//   @... -> ParseMention
//   Text -> ParseText
//   (...) -> ParseSubject
//   `...` -> ParseCode
//   Text :// ... -> ParseLink
//   [...](...) -> ParseLink
//   ![...](...) -> ParseLink
//   <...> -> ParseLink
// If the parse fails for Tag or Mention it will fallback to Text
func (p *parser) ParseElem() Elem {
	var e Elem

	switch p.curTok.Type {
	case TokLBRACK, TokBANG, TokLT:
		e = p.ParseLink()
		p.skipSubject = true // if parsing a non text or mention -> enable subject skip
	case TokCODE:
		e = p.ParseCode()
		p.skipSubject = true
	case TokLS:
		e = p.ParseLineSeparator()
		p.skipSubject = true
	case TokLPAREN:
		e = p.parseSubjectOrText()
		p.skipSubject = true
	case TokHASH:
		e = p.ParseTag()
		p.skipSubject = true
	case TokAT:
		e = p.ParseMention()
	case TokNL, TokEOF:
		p.skipSubject = true
		return nil
	default:
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			e = p.ParseLink()
			p.skipSubject = true
		} else {
			e = p.ParseText()
			if text, ok := e.(*Text); ok && !text.IsSpace() {
				p.skipSubject = true
			}
		}
	}

	// If parsing above failed convert to Text
	if e == nil || e.IsNil() {
		e = p.ParseText()
		if text, ok := e.(*Text); ok && !text.IsSpace() {
			p.skipSubject = true
		}
	}

	return e
}

// ParseMention from tokens
// Forms parsed:
//   @name
//   @name@domain
//   @<target>
//   @<name target>
//   @<name@domain>
//   @<name@domain target>
func (p *parser) ParseMention() *Mention {
	m := &Mention{}

	// form: @nick
	if p.curTokenIs(TokAT) && p.peekTokenIs(TokSTRING) {
		p.append(p.curTok.Literal...) // @
		p.next()

		m.name = string(p.curTok.Literal)

		p.append(p.curTok.Literal...)
		p.next()

		if p.curTokenIs(TokAT) && p.peekTokenIs(TokSTRING) {
			p.append(p.curTok.Literal...)
			p.next()

			p.push()
			p.append(p.curTok.Literal...) // domain text
			for !p.nextTokenIs(TokGT, TokRPAREN, TokSPACE, TokEOF) {
				p.next()
				p.append(p.curTok.Literal...) // domain text
			}
			m.domain = p.Literal()
			p.pop()
		}

		m.lit = p.Literal()
		return m
	}

	// forms: @<...>
	if p.curTokenIs(TokAT) && p.peekTokenIs(TokLT) {
		p.append(p.curTok.Literal...) // @
		p.next()

		p.append(p.curTok.Literal...) // <
		p.next()

		// form: @<nick scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSPACE) {
			m.name = string(p.curTok.Literal)

			p.append(p.curTok.Literal...) // string
			p.next()
			if !p.curTokenIs(TokSPACE) {
				return nil
			}
		}

		// form: @<nick@domain scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokAT) {
			m.name = string(p.curTok.Literal)

			p.append(p.curTok.Literal...) // string
			p.next()

			p.append(p.curTok.Literal...) // @
			p.next()

			p.push()
			p.append(p.curTok.Literal...) // domain text
			for !p.nextTokenIs(TokGT, TokRPAREN, TokSPACE, TokEOF) {
				p.next()
				p.append(p.curTok.Literal...) // domain text
			}
			m.domain = p.Literal()
			p.pop()

			if !p.curTokenIs(TokSPACE) {
				return nil
			}
		}

		if p.curTokenIs(TokSPACE) {
			p.append(p.curTok.Literal...)
			p.next()
		}

		// form: #<[...]scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			p.push()
			l := p.ParseLink()
			p.pop()

			if l == nil {
				return nil // bad url
			}

			m.target = l.target
		}

		if !p.curTokenIs(TokGT) {
			return nil
		}
		p.append(p.curTok.Literal...) // >
		p.next()

		m.lit = p.Literal()

		return m
	}

	return nil
}

// ParseTag from tokens
// Forms parsed:
//   #tag
//   #<target>
//   #<tag target>
func (p *parser) ParseTag() *Tag {
	tag := &Tag{}

	// form: #tag
	if p.curTokenIs(TokHASH) && p.peekTokenIs(TokSTRING) {
		p.append(p.curTok.Literal...) // #
		p.next()

		p.append(p.curTok.Literal...) // string
		tag.lit = p.Literal()
		tag.tag = string(p.curTok.Literal)

		p.next()

		return tag
	}

	// form: #<...>
	if p.curTokenIs(TokHASH) && p.peekTokenIs(TokLT) {
		p.append(p.curTok.Literal...) // #
		p.next()

		p.append(p.curTok.Literal...) // <
		p.next()

		// form: #<tag scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSPACE) {
			p.append(p.curTok.Literal...) // string
			tag.tag = string(p.curTok.Literal)
			p.next()

			p.append(p.curTok.Literal...) // space
			p.next()
		}

		// form: #<scheme://example.com>
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			p.push()
			l := p.ParseLink()
			p.pop()

			if l == nil {
				return nil // bad url
			}

			tag.target = l.target
		}

		if !p.curTokenIs(TokGT) {
			return nil
		}

		p.append(p.curTok.Literal...) // >
		p.next()

		tag.lit = p.Literal()

		return tag
	}

	return nil
}

// parseSubjectOrText for handling subject skipping.
func (p *parser) parseSubjectOrText() Elem {
	if p.skipSubject {
		p.append(p.curTok.Literal...)
		p.next()
		return &Text{p.Literal()}
	}
	return p.ParseSubject()
}

// ParseSubject from tokens
// Forms parsed:
//   (#tag)
//   (#<target>)
//   (#<tag target>)
//   (re: something)
func (p *parser) ParseSubject() *Subject {

	subject := &Subject{}

	p.append(p.curTok.Literal...) // (
	p.next()

	// form: (#tag)
	if p.curTokenIs(TokHASH) {
		p.push()
		subject.tag = p.ParseTag()
		p.pop()

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}
		p.append(p.curTok.Literal...) // )
		p.next()

		return subject
	}

	// form: (text)
	if !p.curTokenIs(TokRPAREN) {
		p.push()
		p.append(p.curTok.Literal...) // subject text
		for !p.nextTokenIs(TokGT, TokRPAREN, TokEOF) {
			p.next()
			p.append(p.curTok.Literal...) // subject text
		}
		subject.subject = p.Literal()
		p.pop()

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}

		p.append(p.curTok.Literal...) // )
		p.next()

		return subject
	}

	return nil
}

// ParseText from tokens.
// Forms parsed:
//   combination of string and space tokens.
func (p *parser) ParseText() *Text {
	// Ensure we arnt at the end of line.
	if !p.curTokenIs(TokNL, TokEOF) {
		p.append(p.curTok.Literal...)
		p.next()
	}

	for p.curTokenIs(TokSTRING, TokSPACE) ||
		// We don't want to parse an email address or link accidentally as a mention or tag. So check if it is preceded with a space.
		(p.curTokenIs(TokHASH, TokAT, TokLT, TokLPAREN) && (len(p.lit) == 0 || !unicode.IsSpace(p.lit[len(p.lit)-1]))) {

		// If end of line break out.
		if p.curTokenIs(TokNL, TokEOF) {
			break
		}

		// if it looks like a link break out.
		if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
			break
		}

		p.append(p.curTok.Literal...)
		p.next()
	}

	txt := &Text{p.Literal()}

	return txt
}

// ParseLineSeparator from tokens.
// Forms parsed:
//   \u2028
func (p *parser) ParseLineSeparator() Elem {
	p.append(p.curTok.Literal...)
	p.next()
	return LineSeparator
}

// ParseLink from tokens.
// Forms parsed:
//   scheme://example.com
//	 <scheme://example.com>
//   [a link](scheme://example.com)
//   ![a image](scheme://example.com/img.png)
//   ![a image](scheme://example.com/img.png "a title")
//   ![a image](scheme://example.com/img.png 'a title')
//
func (p *parser) ParseLink() *Link {
	link := &Link{linkType: LinkStandard}

	if p.curTokenIs(TokSTRING) && p.peekTokenIs(TokSCHEME) {
		link.linkType = LinkNaked

		p.append(p.curTok.Literal...) // scheme
		p.next()

		p.append(p.curTok.Literal...) // link text
		for !p.nextTokenIs(TokGT, TokRPAREN, TokSPACE, TokNL, TokLS, TokEOF) {
			p.next()
			p.append(p.curTok.Literal...) // link text

			// Allow excaped chars to not close.
			if p.curTokenIs(TokBSLASH) {
				p.next()
				p.append(p.curTok.Literal...) // text
			}
		}

		link.target = p.Literal()

		return link
	}

	// Plain Link
	if p.curTokenIs(TokLT) && p.peekTokenIs(TokSTRING) {
		link.linkType = LinkPlain
		p.append(p.curTok.Literal...) // <
		p.next()

		p.push()
		l := p.ParseLink()
		p.pop()

		if l == nil {
			return nil
		}
		if !p.curTokenIs(TokGT) {
			return nil
		}

		p.append(p.curTok.Literal...) // >
		p.next()
		link.target = l.target

		return link
	}

	// Media Link
	if p.curTokenIs(TokBANG) && p.peekTokenIs(TokLBRACK) {
		link.linkType = LinkMedia
		p.append(p.curTok.Literal...) // !
		p.next()
	}

	if !p.curTokenIs(TokLBRACK) {
		return nil
	}

	// Parse Text
	p.append(p.curTok.Literal...) // [
	p.next()

	if !p.curTokenIs(TokRBRACK) {
		p.push()
		p.append(p.curTok.Literal...) // text
		p.next()

		for !p.curTokenIs(TokRBRACK, TokLBRACK, TokRPAREN, TokLPAREN, TokNL, TokEOF) {
			p.append(p.curTok.Literal...) // text
			p.next()

			// Allow excaped chars to not close.
			if p.curTokenIs(TokBSLASH) {
				p.append(p.curTok.Literal...) // text
				p.next()
			}
		}
		link.text = p.Literal()
		p.pop()
	}

	if !p.curTokenIs(TokRBRACK) {
		return nil
	}

	p.append(p.curTok.Literal...) // ]
	p.next()

	// Parse Target
	if p.curTokenIs(TokLPAREN) && !p.peekTokenIs(TokRPAREN) {
		p.append(p.curTok.Literal...) // (
		p.next()

		p.push()
		l := p.ParseLink()
		p.pop()

		if l == nil && !p.curTokenIs(TokRBRACK) {
			p.push()
			p.append(p.curTok.Literal...) // text
			p.next()

			for !p.curTokenIs(TokRBRACK, TokLBRACK, TokRPAREN, TokLPAREN, TokEOF) {
				p.append(p.curTok.Literal...) // text
				p.next()

				// Allow excaped chars to not close.
				if p.curTokenIs(TokBSLASH) {
					p.append(p.curTok.Literal...) // text
					p.next()
				}
			}
			l = &Link{target: p.Literal()}
			p.pop()
		}

		if l == nil {
			return nil
		}

		link.target = l.target

		if p.curTokenIs(TokSPACE) {
			p.push()

			p.append(p.curTok.Literal...) // SPACE
			p.next()

			if p.curTokenIs(TokSQUOTE, TokDQUOTE) {
				end := p.curTok.Type

				p.push()
				p.append(p.curTok.Literal...) // ' or "
				p.next()

				for !p.curTokenIs(end, TokRBRACK, TokLBRACK, TokRPAREN, TokLPAREN, TokEOF) {
					p.append(p.curTok.Literal...) // text
					p.next()

					// Allow excaped chars to not close.
					if p.curTokenIs(TokBSLASH) {
						p.append(p.curTok.Literal...) // text
						p.next()
					}
				}

				p.append(p.curTok.Literal...) // ' or " (matching above)
				p.next()

				link.title = p.Literal()

				p.pop()
			}
			p.pop()
		}

		if !p.curTokenIs(TokRPAREN) {
			return nil
		}

		p.append(p.curTok.Literal...) // )
		p.next()

		return link
	}

	return nil
}

// ParseCode from tokens
// Forms parsed:
//   `inline code`
//   ```
//   block code
//   ```
func (p *parser) ParseCode() *Code {
	code := &Code{}
	p.append(p.curTok.Literal...) // )

	lit := p.Literal()
	code.codeType = CodeInline
	if len(lit) > 3 && lit[:3] == "```" {
		code.codeType = CodeBlock
	}
	code.lit = strings.Trim(lit, "`")
	p.next()

	return code
}

func (p *parser) Literal() string { return string(p.lit[p.pos():]) }

func (p *parser) Errs() ListError {
	if len(p.errs) == 0 {
		return nil
	}
	return p.errs
}

type ListError []error

func (e ListError) Error() string {
	var b strings.Builder
	for _, err := range e {
		b.WriteString(err.Error())
		b.WriteRune('\n')
	}
	return b.String()
}

// Parser evaluation functions.

func (p *parser) IsEOF() bool {
	return p.curTokenIs(TokEOF)
}

func (p *parser) append(lis ...rune) { p.lit = append(p.lit, lis...) }

// frame functions
func (p *parser) pos() int {
	if len(p.frame) == 0 {
		return 0
	}
	return p.frame[len(p.frame)-1]
}
func (p *parser) push() {
	p.frame = append(p.frame, len(p.lit))
}
func (p *parser) pop() {
	if len(p.frame) == 0 {
		return
	}
	p.frame = p.frame[:len(p.frame)-1]
}
func (p *parser) clear() {
	p.frame = p.frame[:0]
	p.lit = p.lit[:0]
}

// next promotes the next token and loads a new one.
// the parser keeps two buffers to store tokens and alternates them here.
func (p *parser) next() {
	p.curPos, p.nextPos = p.nextPos, p.l.rpos
	p.curTok, p.nextTok = p.nextTok, p.curTok
	p.nextTok.Literal = p.nextTok.Literal[:0]
	p.l.NextTok()
	p.nextTok.Type = p.l.Token
	p.nextTok.Literal = append(p.nextTok.Literal, p.l.Literal...)
}

func (p *parser) nextLine() {
	for !p.curTokenIs(TokNL, TokEOF) {
		p.next()
	}
}

// curTokenIs returns true if any of provided TokTypes match current token.
func (p *parser) curTokenIs(tokens ...TokType) bool {
	for _, t := range tokens {
		if p.curTok.Type == t {
			return true
		}
	}
	return false
}

// peekTokenIs returns true if any of provided TokTypes match next token.
func (p *parser) peekTokenIs(tokens ...TokType) bool {
	for _, t := range tokens {
		if p.nextTok.Type == t {
			return true
		}
	}
	return false
}

// nextTokenIs returns true if any of provided TokTypes match next token and reads next token. to next token.
func (p *parser) nextTokenIs(tokens ...TokType) bool {
	if p.peekTokenIs(tokens...) {
		p.next()
		return true
	}

	return false
}

// expect returns true if the current token matches. adds error if not.
// Need to come up with a good proxy for failed parsing of a twtxt line.
// Current mode is to treat failed elements as text.
func (p *parser) expect(t TokType) bool {
	// return p.curTokenIs(t)

	if p.curTokenIs(t) {
		return true
	}

	p.addError(fmt.Errorf("%w: expected current %v, found %v", ErrParseToken, t, p.curTok.Type))
	return false
}

// expectNext returns true if the current token matches and reads to next token. adds error if not.
func (p *parser) expectNext(t TokType) bool {
	// return p.peekTokenIs(t)

	if p.peekTokenIs(t) {
		p.next()
		return true
	}

	p.addError(fmt.Errorf("%w: expected next %v, found %v", ErrParseToken, t, p.nextTok.Type))
	return false
}

func (p *parser) expectLen(t Token, min, max int) bool {
	if len(t.Literal) < min {
		p.addError(fmt.Errorf("%w: expected min len %v got %v", ErrParseToken, min, len(t.Literal)))
		return false
	}

	if len(t.Literal) > max {
		p.addError(fmt.Errorf("%w: expected max len %v got %v", ErrParseToken, max, len(t.Literal)))
		return false
	}

	return true
}

// parseDigit converts current token to int. adds error if fails.
func (p *parser) parseDigit() (int, bool) {
	if !p.curTokenIs(TokNUMBER) {
		p.addError(fmt.Errorf("%w: expected digit, found %T", ErrParseToken, p.curTok.Type))
		return 0, false
	}

	i, err := strconv.Atoi(string(p.curTok.Literal))

	p.addError(err)
	p.next()

	return i, err == nil
}

// addError to parser.
func (p *parser) addError(err error) {
	if err != nil {
		p.errs = append(p.errs, err)
	}
}

var ErrParseElm = errors.New("error parsing element")
var ErrParseToken = errors.New("error parsing digit")
