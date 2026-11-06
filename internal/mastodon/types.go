// Package mastodon reads the Mastodon API and handles login. It knows nothing
// about the interface.
package mastodon

type AccountField struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	VerifiedAt string `json:"verified_at"`
}

type CustomEmoji struct {
	Shortcode string `json:"shortcode"`
	URL       string `json:"url"`
	StaticURL string `json:"static_url"`
}

type Account struct {
	Emojis       []CustomEmoji  `json:"emojis"`
	Fields       []AccountField `json:"fields"`
	ID           string         `json:"id"`
	Username     string         `json:"username"`
	DisplayName  string         `json:"display_name"`
	Acct         string         `json:"acct"`
	Avatar       string         `json:"avatar"`
	AvatarStatic string         `json:"avatar_static"`
	Note         string         `json:"note"`
	URL          string         `json:"url"`
	Followers    int            `json:"followers_count"`
	Following    int            `json:"following_count"`
	Statuses     int            `json:"statuses_count"`
}

func (a Account) Name() string {
	if a.DisplayName != "" {
		return a.DisplayName
	}
	return a.Username
}

type MediaDimensions struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type MediaMeta struct {
	Original MediaDimensions `json:"original"`
	Small    MediaDimensions `json:"small"`
}

type Media struct {
	Meta        MediaMeta `json:"meta"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	Preview     string    `json:"preview_url"`
	Description string    `json:"description"`
}

type LinkCard struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Provider    string `json:"provider_name"`
}

type PollOption struct {
	Title      string `json:"title"`
	VotesCount *int   `json:"votes_count"`
}

type Poll struct {
	Emojis      []CustomEmoji `json:"emojis"`
	ID          string        `json:"id"`
	ExpiresAt   string        `json:"expires_at"`
	Expired     bool          `json:"expired"`
	Multiple    bool          `json:"multiple"`
	VotesCount  int           `json:"votes_count"`
	VotersCount *int          `json:"voters_count"`
	Voted       bool          `json:"voted"`
	OwnVotes    []int         `json:"own_votes"`
	Options     []PollOption  `json:"options"`
}

type Status struct {
	Mentions   []Account     `json:"mentions"`
	Emojis     []CustomEmoji `json:"emojis"`
	Poll       *Poll         `json:"poll"`
	Favourited bool          `json:"favourited"`
	Reblogged  bool          `json:"reblogged"`
	Bookmarked bool          `json:"bookmarked"`
	Visibility string        `json:"visibility"`
	Card       *LinkCard     `json:"card"`

	ID         string  `json:"id"`
	Content    string  `json:"content"`
	URL        string  `json:"url"`
	CreatedAt  string  `json:"created_at"`
	Account    Account `json:"account"`
	Reblog     *Status `json:"reblog"`
	Spoiler    string  `json:"spoiler_text"`
	Sensitive  bool    `json:"sensitive"`
	Media      []Media `json:"media_attachments"`
	Replies    int     `json:"replies_count"`
	Reblogs    int     `json:"reblogs_count"`
	Favourites int     `json:"favourites_count"`
	ReplyTo    string  `json:"in_reply_to_id"`
	Filtered   []struct {
		Filter struct {
			Action string `json:"filter_action"`
		} `json:"filter"`
	} `json:"filtered"`
}

// Notification is one entry from /api/v1/notifications. Status is nil for
// follow and admin events, so the interface must not assume a post.
type Notification struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	CreatedAt string  `json:"created_at"`
	Account   Account `json:"account"`
	Status    *Status `json:"status"`
}

type SearchResult struct {
	Accounts []Account `json:"accounts"`
	Statuses []Status  `json:"statuses"`
}
