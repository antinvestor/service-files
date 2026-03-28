package models

import (
	"time"

	"github.com/pitabwire/frame/data"
	"gorm.io/datatypes"
)

// LinkState represents the lifecycle state of a redirect link.
type LinkState int32

const (
	LinkStateUnspecified LinkState = 0
	LinkStateActive      LinkState = 1
	LinkStatePaused      LinkState = 2
	LinkStateExpired     LinkState = 3
	LinkStateDeleted     LinkState = 4
)

func (s LinkState) String() string {
	switch s {
	case LinkStateActive:
		return "active"
	case LinkStatePaused:
		return "paused"
	case LinkStateExpired:
		return "expired"
	case LinkStateDeleted:
		return "deleted"
	default:
		return "unspecified"
	}
}

// DeviceType classifies the device from user agent parsing.
type DeviceType int32

const (
	DeviceTypeUnspecified DeviceType = 0
	DeviceTypeDesktop     DeviceType = 1
	DeviceTypeMobile      DeviceType = 2
	DeviceTypeTablet      DeviceType = 3
	DeviceTypeBot         DeviceType = 4
)

func (d DeviceType) String() string {
	switch d {
	case DeviceTypeDesktop:
		return "desktop"
	case DeviceTypeMobile:
		return "mobile"
	case DeviceTypeTablet:
		return "tablet"
	case DeviceTypeBot:
		return "bot"
	default:
		return "unknown"
	}
}

// Link represents a redirect link with affiliate tracking metadata.
type Link struct {
	data.BaseModel `gorm:"embedded"`

	Slug           string `gorm:"column:slug;type:varchar(50);uniqueIndex;not null"`
	DestinationURL string `gorm:"column:destination_url;type:text;not null"`
	Title          string `gorm:"column:title;type:varchar(500)"`
	AffiliateID    string `gorm:"column:affiliate_id;type:varchar(50);index"`

	// UTM-style campaign tracking.
	Campaign string `gorm:"column:campaign;type:varchar(250);index"`
	Source   string `gorm:"column:source;type:varchar(250)"`
	Medium   string `gorm:"column:medium;type:varchar(250)"`
	Content  string `gorm:"column:content;type:varchar(250)"`
	Term     string `gorm:"column:term;type:varchar(250)"`

	Tags      datatypes.JSONMap `gorm:"column:tags;type:jsonb"`
	MaxClicks int64             `gorm:"column:max_clicks;default:0"`
	ExpiresAt time.Time         `gorm:"column:expires_at"`

	State            LinkState `gorm:"column:state;type:smallint;default:1;index"`
	ClickCount       int64     `gorm:"column:click_count;default:0"`
	UniqueClickCount int64     `gorm:"column:unique_click_count;default:0"`
}

func (Link) TableName() string { return "links" }

func (l *Link) IsActive() bool {
	if l.State != LinkStateActive {
		return false
	}
	if !l.ExpiresAt.IsZero() && time.Now().After(l.ExpiresAt) {
		return false
	}
	if l.MaxClicks > 0 && l.ClickCount >= l.MaxClicks {
		return false
	}
	return true
}

// Click records a single redirect event with telemetry data.
type Click struct {
	data.BaseModel `gorm:"embedded"`

	LinkID      string `gorm:"column:link_id;type:varchar(50);index:idx_click_link_created,priority:1;not null"`
	AffiliateID string `gorm:"column:affiliate_id;type:varchar(50);index"`
	Slug        string `gorm:"column:slug;type:varchar(50);index"`

	IPAddress      string `gorm:"column:ip_address;type:varchar(45)"`
	UserAgent      string `gorm:"column:user_agent;type:text"`
	Referer        string `gorm:"column:referer;type:text"`
	AcceptLanguage string `gorm:"column:accept_language;type:varchar(250)"`

	// Derived fields.
	Country    string     `gorm:"column:country;type:varchar(10)"`
	City       string     `gorm:"column:city;type:varchar(100)"`
	DeviceType DeviceType `gorm:"column:device_type;type:smallint"`
	Browser    string     `gorm:"column:browser;type:varchar(100)"`
	OS         string     `gorm:"column:os;type:varchar(100)"`
}

func (Click) TableName() string { return "clicks" }
