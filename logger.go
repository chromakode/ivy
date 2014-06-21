package ivy

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jmhodges/levigo"
	"io"
	"regexp"
	"time"
)

type logEntry struct {
	timestamp time.Time
	path      string
	line      string
}

type logger struct {
	db  *levigo.DB
	log chan logEntry
}

var ro = levigo.NewReadOptions()
var wo = levigo.NewWriteOptions()

var ErrInvalidGlob = errors.New("ivy log: invalid glob")

func newLogger(logDir string) (newLogger *logger, err error) {
	opts := levigo.NewOptions()
	opts.SetBlockSize(2 * 44100)
	opts.SetCache(levigo.NewLRUCache(256 * 1024 * 1024))
	opts.SetCompression(levigo.SnappyCompression)
	opts.SetCreateIfMissing(true)
	db, err := levigo.Open(logDir, opts)
	if err != nil {
		return nil, err
	}

	newLogger = &logger{
		db:  db,
		log: make(chan logEntry),
	}

	return
}

func (logger *logger) write(entry logEntry) {
	key := entry.path + timeString(entry.timestamp)
	logger.db.Put(wo, []byte(key), []byte(entry.line))
}

var invalidGlob = regexp.MustCompile(`\*\*|\w\*|\*\w|\/$`)

func (logger *logger) readGlobbedLog(globbedPath string, lineCount int) (reader io.ReadSeeker, maxModTime time.Time, err error) {
	var lines [][]byte

	if invalidGlob.MatchString(globbedPath) {
		return nil, maxModTime, ErrInvalidGlob
	}

	it := logger.db.NewIterator(ro)
	defer it.Close()

	globParts := bytes.Split([]byte(globbedPath), []byte{'*'})

	var recurse func([]byte, int) (time.Time, error)
	recurse = func(basePath []byte, pathIdx int) (maxModTime time.Time, err error) {
		// keys are of the format /path/path/path@timestamp
		if pathIdx == len(globParts) {
			// path matches glob; collect the lines backwards from path@@ to path@
			firstKey := append(basePath, '@')
			startKey := firstKey
			endKey := append(basePath, '@', '@') // @@ is lexographically after @<number>

			// step back from the end lineCount lines
			if lineCount > 0 {
				linesIt := logger.db.NewIterator(ro)
				defer linesIt.Close()

				linesIt.Seek(endKey)
				if !linesIt.Valid() {
					// end of the db
					linesIt.SeekToLast()

					// empty db
					if !linesIt.Valid() {
						return
					}
				}

				linesIt.Prev()
				for linesIt, count := linesIt, 0; linesIt.Valid() && bytes.Compare(linesIt.Key(), firstKey) != -1 && count < lineCount; linesIt.Prev() {
					startKey = linesIt.Key()
					count++
				}
			}

			it.Seek(startKey)

			var lastLine []byte
			for it = it; it.Valid() && bytes.Compare(it.Key(), endKey) == -1; it.Next() {
				lastLine = it.Value()
				lines = append(lines, lastLine)
			}

			// the latest line should have the latest timestamp
			if lastLine != nil {
				var sec int64
				var nsec int64
				fmt.Fscanf(bytes.NewReader(lastLine), timeStringFormat, &sec, &nsec)
				maxModTime = time.Unix(sec, nsec)
			}
		} else {
			it.Seek(basePath)
			nextPart := globParts[pathIdx]
			endPath := append([]byte{}, basePath...)
			endPath[len(endPath)-1]++
			// look for paths of the form basePath/*subpath*/nextPart
			for it = it; it.Valid(); {
				key := it.Key()
				if bytes.Compare(key, endPath) != -1 {
					break
				}

				// strip @
				timestampSep := bytes.IndexByte(key, '@')
				if timestampSep == -1 {
					panic(fmt.Errorf("ivy log: invalid log key: %q", key))
				}
				key = key[:timestampSep]

				// get the index of the next '/', or if none, the end of the key
				pathSep := bytes.IndexByte(key[len(basePath):], '/')
				if pathSep == -1 {
					pathSep = len(key)
				} else {
					pathSep += len(basePath)
				}

				if bytes.HasPrefix(key[pathSep:], nextPart) {
					localMaxModTime, err := recurse(key[:pathSep+len(nextPart)], pathIdx+1)
					if err != nil {
						return maxModTime, err
					}
					if localMaxModTime.After(maxModTime) {
						maxModTime = localMaxModTime
					}
				}

				nextKey := key[:pathSep]
				// if we're past basepath/*subpath*/nextPart, seek to following subpath
				if bytes.Compare(key[pathSep:], nextPart) != -1 {
					nextKey[len(nextKey)-1]++
				}
				nextKey = append(nextKey, nextPart...)
				it.Seek(nextKey)
			}
		}

		err = it.GetError()
		return
	}

	maxModTime, err = recurse(globParts[0], 1)
	if err != nil {
		return
	}

	reader = bytes.NewReader(bytes.Join(lines, []byte{'\n'}))
	return
}

func (logger *logger) run() {
	for {
		entry := <-logger.log
		logger.write(entry)
	}
}
