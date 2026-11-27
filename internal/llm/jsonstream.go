package llm

import (
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// JSONFieldEvent is emitted by JSONStreamExtractor as top-level string fields
// of a streamed JSON object are parsed.
type JSONFieldEvent struct {
	Field string
	Delta string // incremental unescaped chunk (for streamed fields)
	Value string // complete field value (when Done is true)
	Done  bool
}

type jsonStreamState int

const (
	jsExpectObject jsonStreamState = iota
	jsExpectKeyOrEnd
	jsInKey
	jsExpectColon
	jsExpectValue
	jsInStringValue
	jsSkipOtherValue
	jsDone
)

// JSONStreamExtractor incrementally parses a streamed flat JSON object and
// emits unescaped string deltas in real time for selected fields (e.g. "content"),
// and completed string values for metadata fields (e.g. "join_mode", "intent").
type JSONStreamExtractor struct {
	streamField string
	state       jsonStreamState

	currentKey strings.Builder
	currentVal strings.Builder

	// String escape handling
	inEscape    bool
	unicodeHex  []byte
	highSurr    rune
	hasHighSurr bool

	// Nested skip depth for non-string values
	skipDepth int
	skipInStr bool
	skipEsc   bool
}

// NewJSONStreamExtractor creates an extractor that streams deltas live for
// streamField and emits completed values for all top-level string fields.
func NewJSONStreamExtractor(streamField string) *JSONStreamExtractor {
	return &JSONStreamExtractor{
		streamField: streamField,
		state:       jsExpectObject,
	}
}

// Feed processes an incoming JSON text chunk and returns any field events produced.
func (e *JSONStreamExtractor) Feed(chunk string) []JSONFieldEvent {
	var events []JSONFieldEvent
	var streamBuf strings.Builder

	flushStream := func() {
		if streamBuf.Len() > 0 {
			events = append(events, JSONFieldEvent{
				Field: e.streamField,
				Delta: streamBuf.String(),
			})
			streamBuf.Reset()
		}
	}

	writeDecodedRune := func(r rune) {
		if e.state == jsInKey {
			e.currentKey.WriteRune(r)
			return
		}
		key := e.currentKey.String()
		if key == e.streamField {
			streamBuf.WriteRune(r)
		} else {
			e.currentVal.WriteRune(r)
		}
	}

	flushPendingSurrogate := func() {
		if e.hasHighSurr {
			writeDecodedRune(utf8.RuneError)
			e.hasHighSurr = false
			e.highSurr = 0
		}
	}

	for i := 0; i < len(chunk); {
		b := chunk[i]
		// Handle multi-byte UTF-8 runes directly when inside strings and not escaping.
		if (e.state == jsInKey || e.state == jsInStringValue) && !e.inEscape && len(e.unicodeHex) == 0 && b >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(chunk[i:])
			flushPendingSurrogate()
			writeDecodedRune(r)
			i += size
			continue
		}
		i++

		switch e.state {
		case jsExpectObject:
			if b == '{' {
				e.state = jsExpectKeyOrEnd
			}

		case jsExpectKeyOrEnd:
			switch b {
			case ' ', '\t', '\r', '\n', ',':
			case '}':
				e.state = jsDone
			case '"':
				e.currentKey.Reset()
				e.inEscape = false
				e.unicodeHex = e.unicodeHex[:0]
				e.hasHighSurr = false
				e.state = jsInKey
			}

		case jsInKey, jsInStringValue:
			if len(e.unicodeHex) > 0 || (e.inEscape && b == 'u' && len(e.unicodeHex) == 0) {
				if e.inEscape && b == 'u' {
					e.inEscape = false
					e.unicodeHex = append(e.unicodeHex[:0], 'u') // sentinel to start collecting 4 hex digits
					continue
				}
				// Collecting 4 hex digits after sentinel 'u'
				if len(e.unicodeHex) > 0 && e.unicodeHex[0] == 'u' {
					e.unicodeHex = append(e.unicodeHex, b)
					if len(e.unicodeHex) == 5 {
						hexStr := string(e.unicodeHex[1:5])
						e.unicodeHex = e.unicodeHex[:0]
						v, err := strconv.ParseUint(hexStr, 16, 16)
						if err != nil {
							flushPendingSurrogate()
							writeDecodedRune(utf8.RuneError)
							continue
						}
						r := rune(v)
						if e.hasHighSurr {
							if utf16.IsSurrogate(r) {
								combined := utf16.DecodeRune(e.highSurr, r)
								e.hasHighSurr = false
								e.highSurr = 0
								writeDecodedRune(combined)
							} else {
								flushPendingSurrogate()
								writeDecodedRune(r)
							}
						} else if r >= 0xD800 && r <= 0xDBFF {
							e.highSurr = r
							e.hasHighSurr = true
						} else {
							writeDecodedRune(r)
						}
					}
					continue
				}
			}

			if e.inEscape {
				e.inEscape = false
				flushPendingSurrogate()
				switch b {
				case '"':
					writeDecodedRune('"')
				case '\\':
					writeDecodedRune('\\')
				case '/':
					writeDecodedRune('/')
				case 'b':
					writeDecodedRune('\b')
				case 'f':
					writeDecodedRune('\f')
				case 'n':
					writeDecodedRune('\n')
				case 'r':
					writeDecodedRune('\r')
				case 't':
					writeDecodedRune('\t')
				default:
					writeDecodedRune(rune(b))
				}
				continue
			}

			if b == '\\' {
				e.inEscape = true
				continue
			}

			if b == '"' {
				flushPendingSurrogate()
				if e.state == jsInKey {
					e.state = jsExpectColon
				} else {
					flushStream()
					key := e.currentKey.String()
					if key == e.streamField {
						events = append(events, JSONFieldEvent{
							Field: key,
							Done:  true,
						})
					} else {
						events = append(events, JSONFieldEvent{
							Field: key,
							Value: e.currentVal.String(),
							Done:  true,
						})
					}
					e.currentVal.Reset()
					e.state = jsExpectKeyOrEnd
				}
				continue
			}

			flushPendingSurrogate()
			writeDecodedRune(rune(b))

		case jsExpectColon:
			if b == ':' {
				e.state = jsExpectValue
			}

		case jsExpectValue:
			switch b {
			case ' ', '\t', '\r', '\n':
			case '"':
				e.currentVal.Reset()
				e.inEscape = false
				e.unicodeHex = e.unicodeHex[:0]
				e.hasHighSurr = false
				e.state = jsInStringValue
			case '{', '[':
				e.skipDepth = 1
				e.skipInStr = false
				e.skipEsc = false
				e.state = jsSkipOtherValue
			default:
				// number, boolean, null
				e.skipDepth = 0
				e.skipInStr = false
				e.skipEsc = false
				e.state = jsSkipOtherValue
			}

		case jsSkipOtherValue:
			if e.skipDepth == 0 {
				if b == ',' {
					e.state = jsExpectKeyOrEnd
				} else if b == '}' {
					e.state = jsDone
				}
				continue
			}
			if e.skipInStr {
				if e.skipEsc {
					e.skipEsc = false
				} else if b == '\\' {
					e.skipEsc = true
				} else if b == '"' {
					e.skipInStr = false
				}
				continue
			}
			switch b {
			case '"':
				e.skipInStr = true
			case '{', '[':
				e.skipDepth++
			case '}', ']':
				e.skipDepth--
				if e.skipDepth == 0 {
					e.state = jsExpectKeyOrEnd
				}
			}

		case jsDone:
			// Ignore trailing whitespace/tokens after closing brace.
		}
	}

	flushStream()
	return events
}
