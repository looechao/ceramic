package mastodon

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-text/typesetting/segmenter"
)

// PostLimits is supplied by the authenticated account's instance.
type PostLimits struct {
	MaxCharacters            int `json:"max_characters"`
	CharactersReservedPerURL int `json:"characters_reserved_per_url"`
}

func (l PostLimits) WithDefaults() PostLimits {
	if l.MaxCharacters <= 0 {
		l.MaxCharacters = 500
	}
	if l.CharactersReservedPerURL <= 0 {
		l.CharactersReservedPerURL = 23
	}
	return l
}

func (a *Client) PostLimits(s *Session) (PostLimits, error) {
	var result struct {
		Configuration struct {
			Statuses PostLimits `json:"statuses"`
		} `json:"configuration"`
	}
	err := a.request("GET", s.Instance+"/api/v2/instance", "", nil, &result)
	var response HTTPError
	if errors.As(err, &response) && response.Status == 404 {
		err = a.request("GET", s.Instance+"/api/v1/instance", "", nil, &result)
	}
	if err != nil {
		return PostLimits{}, err
	}
	return result.Configuration.Statuses.WithDefaults(), nil
}

var countableEntity = regexp.MustCompile(`(?i:https?)://[^\s\p{Z}\x{0085}\x{FEFF}<>]+|[@＠][a-zA-Z0-9_]+(?:[.-]+[a-zA-Z0-9_]+)*@[\p{L}\p{N}\p{M}_]+(?:[.-]+[\p{L}\p{N}\p{M}_]+)*`)

// Count joins the content warning and body, counts grapheme clusters, and
// applies Mastodon's URL reservation and remote-mention domain concession.
// Server validation remains authoritative for version-specific extraction rules.
func (l PostLimits) Count(text, spoiler string) int {
	l = l.WithDefaults()
	var b strings.Builder
	b.WriteString(spoiler)
	end := 0
	reserved := 0
	maxCount := int(^uint(0) >> 1)
	for _, at := range countableEntity.FindAllStringIndex(text, -1) {
		entity := text[at[0]:at[1]]
		if at[0] > 0 {
			before, _ := utf8.DecodeLastRuneInString(text[:at[0]])
			if unicode.IsLetter(before) || unicode.IsNumber(before) || unicode.IsMark(before) || strings.ContainsRune("_/=", before) {
				continue
			}
		}
		replacement := ""
		if strings.HasPrefix(strings.ToLower(entity), "http") {
			entity = strings.TrimRight(entity, ".,!?:;\"'，。！？；：")
			for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
				for strings.HasSuffix(entity, pair[1]) && strings.Count(entity, pair[1]) > strings.Count(entity, pair[0]) {
					entity = strings.TrimSuffix(entity, pair[1])
				}
			}
			u, err := url.Parse(entity)
			if err != nil || u.Hostname() == "" {
				continue
			}
			extra := l.CharactersReservedPerURL - 1
			if extra > maxCount-reserved {
				return maxCount
			}
			reserved += extra
			replacement = "x"
		} else {
			entity = strings.TrimRight(entity, ".")
			if at[1] < len(text) {
				after, _ := utf8.DecodeRuneInString(text[at[1]:])
				if unicode.IsLetter(after) || unicode.IsMark(after) || after == '@' || after == '＠' {
					continue
				}
			}
			account := strings.TrimPrefix(strings.TrimPrefix(entity, "@"), "＠")
			name, domain, _ := strings.Cut(account, "@")
			if domain == "" || len(domain) > 253 {
				continue
			}
			replacement = "@" + name
		}
		b.WriteString(text[end:at[0]])
		b.WriteString(replacement)
		end = at[0] + len(entity)
	}
	b.WriteString(text[end:])
	var seg segmenter.Segmenter
	seg.Init([]rune(b.String()))
	iter := seg.GraphemeIterator()
	count := 0
	for iter.Next() {
		count++
	}
	if count > maxCount-reserved {
		return maxCount
	}
	return count + reserved
}
