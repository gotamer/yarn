package types

import (
	"bytes"
	"io"
	"strings"
)

const PreambleBufferSize = 5

var PreambleMarker = []byte("\n\n")

type ReadSeekerReaderAt interface {
	io.Reader
	io.Seeker
	io.ReaderAt
}

type PreambleFeed struct {
	r   io.ReadSeeker
	pre *strings.Builder
}

func (p *PreambleFeed) Preamble() string                             { return p.pre.String() }
func (p *PreambleFeed) Seek(offset int64, whence int) (int64, error) { return p.r.Seek(offset, whence) }
func (p *PreambleFeed) Read(b []byte) (n int, err error)             { return p.r.Read(b) }

func ReadPreambleFeed(r ReadSeekerReaderAt, size int64) (*PreambleFeed, error) {
	b := make([]byte, PreambleBufferSize)
	p := &PreambleFeed{r: r, pre: &strings.Builder{}}

	// Read the first byte
	i, err := r.Read(b[:1])
	if err != nil {
		if err == io.EOF {
			return p, nil
		}
		return p, err
	}

	// If first byte is not a comment, return the entire feed file with no preamble
	if i > 0 && b[0] != '#' {
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return p, err
		}
		return p, nil
	}

	read := i
	eof := false
	for !eof {
		n := bytes.Index(b[:i], PreambleMarker)
		if n > -1 {
			p.pre.Write(b[:n])
			pos := int64((((read / PreambleBufferSize) - 1) * PreambleBufferSize) + n + 1)
			p.r = io.NewSectionReader(r, pos, (size - pos))
			return p, nil
		}

		_, err = p.pre.Write(b[:i])
		if err != nil {
			return nil, err
		}

		i, err = r.Read(b)
		if err != nil {
			if err == io.EOF {
				eof = true
				continue
			}

			return p, err
		}

		read += i
	}

	// Feed just contains a preamble!
	p.r = io.NewSectionReader(r, size, size)
	return p, nil
}
