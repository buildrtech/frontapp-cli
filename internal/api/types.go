package api

import "time"

// Conversation represents a Front conversation.
type Conversation struct {
	ID           string     `json:"id"`
	Subject      string     `json:"subject"`
	Status       string     `json:"status"` // open, archived, snoozed, trashed
	Assignee     *Teammate  `json:"assignee,omitempty"`
	Recipient    *Recipient `json:"recipient,omitempty"`
	Tags         []Tag      `json:"tags,omitempty"`
	Inboxes      []Inbox    `json:"inboxes,omitempty"`
	CreatedAt    float64    `json:"created_at"` // Unix timestamp
	WaitingSince float64    `json:"waiting_since,omitempty"`
	Links        Links      `json:"_links,omitempty"` //nolint:tagliatelle // Front API //nolint:tagliatelle // Front API uses underscore prefix
}

// Message represents a message in a conversation.
type Message struct {
	ID          string       `json:"id"`
	Type        string       `json:"type"` // email, sms, intercom, custom, etc.
	IsInbound   bool         `json:"is_inbound"`
	CreatedAt   float64      `json:"created_at"`
	Blurb       string       `json:"blurb"`
	Author      *Author      `json:"author,omitempty"`
	Recipients  []Recipient  `json:"recipients,omitempty"`
	Body        string       `json:"body"`
	Text        string       `json:"text"`
	Subject     string       `json:"subject,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Links       Links        `json:"_links,omitempty"` //nolint:tagliatelle // Front API //nolint:tagliatelle // Front API uses underscore prefix
}

// DraftMessage represents a Front draft message using the docs-aligned message shape.
type DraftMessage struct {
	ID          string       `json:"id"`
	MessageUID  string       `json:"message_uid,omitempty"`
	Type        string       `json:"type,omitempty"`
	IsInbound   bool         `json:"is_inbound,omitempty"`
	DraftMode   string       `json:"draft_mode,omitempty"`
	ErrorType   string       `json:"error_type,omitempty"`
	Version     string       `json:"version"`
	CreatedAt   float64      `json:"created_at"`
	UpdatedAt   float64      `json:"updated_at,omitempty"`
	Blurb       string       `json:"blurb,omitempty"`
	Author      *Author      `json:"author,omitempty"`
	Recipients  []Recipient  `json:"recipients,omitempty"`
	Body        string       `json:"body"`
	Text        string       `json:"text"`
	Subject     string       `json:"subject,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Links       Links        `json:"_links,omitempty"` //nolint:tagliatelle // Front API //nolint:tagliatelle // Front API uses underscore prefix
}

// Signature represents a Front message signature.
type Signature struct {
	ID        string  `json:"id"`
	Name      string  `json:"name,omitempty"`
	Body      string  `json:"body,omitempty"`
	IsDefault bool    `json:"is_default,omitempty"`
	CreatedAt float64 `json:"created_at,omitempty"`
	UpdatedAt float64 `json:"updated_at,omitempty"`
	Links     Links   `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Tag represents a Front tag.
type Tag struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Highlight   string  `json:"highlight,omitempty"` // color
	IsPrivate   bool    `json:"is_private,omitempty"`
	ParentTagID string  `json:"parent_tag_id,omitempty"`
	CreatedAt   float64 `json:"created_at,omitempty"`
	UpdatedAt   float64 `json:"updated_at,omitempty"`
	Links       Links   `json:"_links,omitempty"` //nolint:tagliatelle // Front API //nolint:tagliatelle // Front API uses underscore prefix
}

// Inbox represents a Front inbox.
type Inbox struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	IsPrivate    bool                   `json:"is_private,omitempty"`
	IsPublic     bool                   `json:"is_public,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	Links        Links                  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Teammate represents a Front teammate.
type Teammate struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	IsAdmin     bool   `json:"is_admin,omitempty"`
	IsAvailable bool   `json:"is_available,omitempty"`
	IsBlocked   bool   `json:"is_blocked,omitempty"`
	Links       Links  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Contact represents a Front contact.
type Contact struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name,omitempty"`
	Description  string                 `json:"description,omitempty"`
	AvatarURL    string                 `json:"avatar_url,omitempty"`
	IsSpammer    bool                   `json:"is_spammer,omitempty"`
	Links        Links                  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
	Handles      []Handle               `json:"handles,omitempty"`
	Groups       []Group                `json:"groups,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
	CreatedAt    float64                `json:"created_at,omitempty"`
	UpdatedAt    float64                `json:"updated_at,omitempty"`
}

// ContactNote represents a note on a contact.
type ContactNote struct {
	ID        string  `json:"id"`
	Body      string  `json:"body"`
	Author    *Author `json:"author,omitempty"`
	CreatedAt float64 `json:"created_at,omitempty"`
	Links     Links   `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Handle represents a contact handle (email, phone, etc).
type Handle struct {
	Handle string `json:"handle"`
	Source string `json:"source"` // email, phone, twitter, etc.
}

// Group represents a contact group.
type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Channel represents a Front channel.
type Channel struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	Type      string `json:"type"` // email, sms, intercom, custom, etc.
	Address   string `json:"address,omitempty"`
	SendAs    string `json:"send_as,omitempty"`
	IsPrivate bool   `json:"is_private,omitempty"`
	IsValid   bool   `json:"is_valid,omitempty"`
	Links     Links  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Comment represents an internal comment on a conversation.
type Comment struct {
	ID       string  `json:"id"`
	Author   *Author `json:"author,omitempty"`
	Body     string  `json:"body"`
	PostedAt float64 `json:"posted_at"`
	Links    Links   `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Template represents a canned response template.
type Template struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Subject           string       `json:"subject,omitempty"`
	Body              string       `json:"body"`
	IsAvailableForAll bool         `json:"is_available_for_all,omitempty"`
	Attachments       []Attachment `json:"attachments,omitempty"`
	Links             Links        `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Attachment represents a message attachment.
type Attachment struct {
	ID          string `json:"id,omitempty"`
	Filename    string `json:"filename"`
	URL         string `json:"url,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

// Recipient represents a message recipient.
type Recipient struct {
	Handle string `json:"handle,omitempty"`
	Role   string `json:"role,omitempty"`   // to, cc, bcc, from
	Links  Links  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Author represents the author of a message or comment.
type Author struct {
	ID        string `json:"id,omitempty"`
	Email     string `json:"email,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	IsBot     bool   `json:"is_bot,omitempty"`
	Links     Links  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Links contains HATEOAS links.
type Links struct {
	Self    string            `json:"self,omitempty"`
	Related map[string]string `json:"related,omitempty"`
}

// Pagination represents cursor-based pagination.
type Pagination struct {
	Next string `json:"next,omitempty"`
	Prev string `json:"prev,omitempty"`
}

// ListResponse is the generic response for list endpoints.
type ListResponse[T any] struct {
	Results    []T        `json:"_results"`              //nolint:tagliatelle // Front API
	Pagination Pagination `json:"_pagination,omitempty"` //nolint:tagliatelle // Front API
	Links      Links      `json:"_links,omitempty"`      //nolint:tagliatelle // Front API
}

// Me represents the authenticated user.
type Me struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username,omitempty"`
	FirstName   string `json:"first_name,omitempty"`
	LastName    string `json:"last_name,omitempty"`
	IsAdmin     bool   `json:"is_admin,omitempty"`
	IsAvailable bool   `json:"is_available,omitempty"`
	Links       Links  `json:"_links,omitempty"` //nolint:tagliatelle // Front API
}

// Helper to convert Unix timestamp to time.Time
func UnixToTime(ts float64) time.Time {
	if ts == 0 {
		return time.Time{}
	}

	return time.Unix(int64(ts), 0)
}

// Helper to format Unix timestamp as string
func FormatTimestamp(ts float64) string {
	if ts == 0 {
		return ""
	}

	return time.Unix(int64(ts), 0).Format(time.RFC3339)
}
