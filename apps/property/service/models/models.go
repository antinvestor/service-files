package models

import (
	"time"

	commonv1 "buf.build/gen/go/antinvestor/common/protocolbuffers/go/common/v1"
	propertyv1 "buf.build/gen/go/antinvestor/property/protocolbuffers/go/property/v1"
	"github.com/pitabwire/frame/data"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/datatypes"
)

type Locality struct {
	data.BaseModel

	ParentID    string `gorm:"type:varchar(50)"`
	Name        string `gorm:"type:varchar(50)"`
	Description string `gorm:"type:text"`
	Point       datatypes.JSON
	Boundary    datatypes.JSON
	Extra       datatypes.JSONMap
}

func (l *Locality) ToApi() *propertyv1.Locality {
	locality := &propertyv1.Locality{
		Id:          l.GetID(),
		ParentId:    l.ParentID,
		Name:        l.Name,
		Description: l.Description,
		Extras:      JsonMapToStruct(l.Extra),
		CreatedAt:   timestamppb.New(l.CreatedAt),
	}

	if l.Boundary.String() != "{}" {
		locality.Feature = &propertyv1.Locality_Boundary{Boundary: l.Boundary.String()}
	} else {
		locality.Feature = &propertyv1.Locality_Point{Point: l.Point.String()}
	}

	return locality
}

type PropertyType struct {
	data.BaseModel

	Name        string `gorm:"type:varchar(250)"`
	Description string `gorm:"type:text"`
	Extra       datatypes.JSONMap
}

func (pt *PropertyType) ToApi() *propertyv1.PropertyType {
	return &propertyv1.PropertyType{
		Id:          pt.GetID(),
		Name:        pt.Name,
		Description: pt.Description,
		Extra:       JsonMapToStruct(pt.Extra),
		CreatedAt:   timestamppb.New(pt.CreatedAt),
	}
}

type Subscription struct {
	data.BaseModel

	PropertyID  string `gorm:"type:varchar(50)"`
	ProfileID   string `gorm:"type:varchar(50)"`
	Role        string `gorm:"type:varchar(250)"`
	Description string `gorm:"type:text"`
	Extra       datatypes.JSONMap
	ExpiresAt   time.Time
}

func (s *Subscription) ToApi() *propertyv1.Subscription {
	return &propertyv1.Subscription{
		Id:         s.GetID(),
		ProfileId:  s.ProfileID,
		PropertyId: s.PropertyID,
		Role:       s.Role,
		Extra:      JsonMapToStruct(s.Extra),
		ExpiresAt:  timestamppb.New(s.ExpiresAt),
		CreatedAt:  timestamppb.New(s.CreatedAt),
	}
}

type Property struct {
	data.BaseModel

	ParentID string `gorm:"type:varchar(50)"`

	PropertyTypeID string `gorm:"type:varchar(50)"`
	LocalityID     string `gorm:"type:varchar(50)"`

	Name        string `gorm:"type:varchar(250)"`
	Description string `gorm:"type:text"`
	Extra       datatypes.JSONMap

	StartedAt time.Time
	StateID   string `gorm:"type:varchar(50)"`
}

type PropertyState struct {
	data.BaseModel
	PropertyID string `gorm:"type:varchar(50)"`

	Name        string `gorm:"type:varchar(250)"`
	Description string `gorm:"type:text"`
	Extra       datatypes.JSONMap
	State       int32
	Status      int32
}

func (ps *PropertyState) ToApi() *propertyv1.PropertyState {
	return &propertyv1.PropertyState{
		Id:          ps.GetID(),
		Propertyid:  ps.PropertyID,
		Name:        ps.Name,
		Description: ps.Description,
		Extras:      JsonMapToStruct(ps.Extra),
		Status:      commonv1.STATUS(ps.Status),
		State:       commonv1.STATE(ps.State),
		CreatedAt:   timestamppb.New(ps.CreatedAt),
	}
}

func JsonMapToStruct(m datatypes.JSONMap) *structpb.Struct {
	if m == nil {
		return nil
	}
	s, _ := structpb.NewStruct(map[string]any(m))
	return s
}

func StructToJSONMap(s *structpb.Struct) datatypes.JSONMap {
	if s == nil {
		return make(datatypes.JSONMap)
	}
	return datatypes.JSONMap(s.AsMap())
}
