// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/datastore/admin/v1/index.proto

package admin

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// For an ordered index, specifies whether each of the entity's ancestors
// will be included.
type Index_AncestorMode int32

const (
	// The ancestor mode is unspecified.
	Index_ANCESTOR_MODE_UNSPECIFIED Index_AncestorMode = 0
	// Do not include the entity's ancestors in the index.
	Index_NONE Index_AncestorMode = 1
	// Include all the entity's ancestors in the index.
	Index_ALL_ANCESTORS Index_AncestorMode = 2
)

var Index_AncestorMode_name = map[int32]string{
	0: "ANCESTOR_MODE_UNSPECIFIED",
	1: "NONE",
	2: "ALL_ANCESTORS",
}

var Index_AncestorMode_value = map[string]int32{
	"ANCESTOR_MODE_UNSPECIFIED": 0,
	"NONE":                      1,
	"ALL_ANCESTORS":             2,
}

func (x Index_AncestorMode) String() string {
	return proto.EnumName(Index_AncestorMode_name, int32(x))
}

func (Index_AncestorMode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_809cc5775e1cdeb3, []int{0, 0}
}

// The direction determines how a property is indexed.
type Index_Direction int32

const (
	// The direction is unspecified.
	Index_DIRECTION_UNSPECIFIED Index_Direction = 0
	// The property's values are indexed so as to support sequencing in
	// ascending order and also query by <, >, <=, >=, and =.
	Index_ASCENDING Index_Direction = 1
	// The property's values are indexed so as to support sequencing in
	// descending order and also query by <, >, <=, >=, and =.
	Index_DESCENDING Index_Direction = 2
)

var Index_Direction_name = map[int32]string{
	0: "DIRECTION_UNSPECIFIED",
	1: "ASCENDING",
	2: "DESCENDING",
}

var Index_Direction_value = map[string]int32{
	"DIRECTION_UNSPECIFIED": 0,
	"ASCENDING":             1,
	"DESCENDING":            2,
}

func (x Index_Direction) String() string {
	return proto.EnumName(Index_Direction_name, int32(x))
}

func (Index_Direction) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_809cc5775e1cdeb3, []int{0, 1}
}

// The possible set of states of an index.
type Index_State int32

const (
	// The state is unspecified.
	Index_STATE_UNSPECIFIED Index_State = 0
	// The index is being created, and cannot be used by queries.
	// There is an active long-running operation for the index.
	// The index is updated when writing an entity.
	// Some index data may exist.
	Index_CREATING Index_State = 1
	// The index is ready to be used.
	// The index is updated when writing an entity.
	// The index is fully populated from all stored entities it applies to.
	Index_READY Index_State = 2
	// The index is being deleted, and cannot be used by queries.
	// There is an active long-running operation for the index.
	// The index is not updated when writing an entity.
	// Some index data may exist.
	Index_DELETING Index_State = 3
	// The index was being created or deleted, but something went wrong.
	// The index cannot by used by queries.
	// There is no active long-running operation for the index,
	// and the most recently finished long-running operation failed.
	// The index is not updated when writing an entity.
	// Some index data may exist.
	Index_ERROR Index_State = 4
)

var Index_State_name = map[int32]string{
	0: "STATE_UNSPECIFIED",
	1: "CREATING",
	2: "READY",
	3: "DELETING",
	4: "ERROR",
}

var Index_State_value = map[string]int32{
	"STATE_UNSPECIFIED": 0,
	"CREATING":          1,
	"READY":             2,
	"DELETING":          3,
	"ERROR":             4,
}

func (x Index_State) String() string {
	return proto.EnumName(Index_State_name, int32(x))
}

func (Index_State) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_809cc5775e1cdeb3, []int{0, 2}
}

// A minimal index definition.
// Next tag: 8
type Index struct {
	// Project ID.
	// Output only.
	ProjectId string `protobuf:"bytes,1,opt,name=project_id,json=projectId,proto3" json:"project_id,omitempty"`
	// The resource ID of the index.
	// Output only.
	IndexId string `protobuf:"bytes,3,opt,name=index_id,json=indexId,proto3" json:"index_id,omitempty"`
	// The entity kind to which this index applies.
	// Required.
	Kind string `protobuf:"bytes,4,opt,name=kind,proto3" json:"kind,omitempty"`
	// The index's ancestor mode.  Must not be ANCESTOR_MODE_UNSPECIFIED.
	// Required.
	Ancestor Index_AncestorMode `protobuf:"varint,5,opt,name=ancestor,proto3,enum=google.datastore.admin.v1.Index_AncestorMode" json:"ancestor,omitempty"`
	// An ordered sequence of property names and their index attributes.
	// Required.
	Properties []*Index_IndexedProperty `protobuf:"bytes,6,rep,name=properties,proto3" json:"properties,omitempty"`
	// The state of the index.
	// Output only.
	State                Index_State `protobuf:"varint,7,opt,name=state,proto3,enum=google.datastore.admin.v1.Index_State" json:"state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}

func (m *Index) Reset()         { *m = Index{} }
func (m *Index) String() string { return proto.CompactTextString(m) }
func (*Index) ProtoMessage()    {}
func (*Index) Descriptor() ([]byte, []int) {
	return fileDescriptor_809cc5775e1cdeb3, []int{0}
}

func (m *Index) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Index.Unmarshal(m, b)
}
func (m *Index) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Index.Marshal(b, m, deterministic)
}
func (m *Index) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Index.Merge(m, src)
}
func (m *Index) XXX_Size() int {
	return xxx_messageInfo_Index.Size(m)
}
func (m *Index) XXX_DiscardUnknown() {
	xxx_messageInfo_Index.DiscardUnknown(m)
}

var xxx_messageInfo_Index proto.InternalMessageInfo

func (m *Index) GetProjectId() string {
	if m != nil {
		return m.ProjectId
	}
	return ""
}

func (m *Index) GetIndexId() string {
	if m != nil {
		return m.IndexId
	}
	return ""
}

func (m *Index) GetKind() string {
	if m != nil {
		return m.Kind
	}
	return ""
}

func (m *Index) GetAncestor() Index_AncestorMode {
	if m != nil {
		return m.Ancestor
	}
	return Index_ANCESTOR_MODE_UNSPECIFIED
}

func (m *Index) GetProperties() []*Index_IndexedProperty {
	if m != nil {
		return m.Properties
	}
	return nil
}

func (m *Index) GetState() Index_State {
	if m != nil {
		return m.State
	}
	return Index_STATE_UNSPECIFIED
}

// Next tag: 3
type Index_IndexedProperty struct {
	// The property name to index.
	// Required.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The indexed property's direction.  Must not be DIRECTION_UNSPECIFIED.
	// Required.
	Direction            Index_Direction `protobuf:"varint,2,opt,name=direction,proto3,enum=google.datastore.admin.v1.Index_Direction" json:"direction,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *Index_IndexedProperty) Reset()         { *m = Index_IndexedProperty{} }
func (m *Index_IndexedProperty) String() string { return proto.CompactTextString(m) }
func (*Index_IndexedProperty) ProtoMessage()    {}
func (*Index_IndexedProperty) Descriptor() ([]byte, []int) {
	return fileDescriptor_809cc5775e1cdeb3, []int{0, 0}
}

func (m *Index_IndexedProperty) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Index_IndexedProperty.Unmarshal(m, b)
}
func (m *Index_IndexedProperty) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Index_IndexedProperty.Marshal(b, m, deterministic)
}
func (m *Index_IndexedProperty) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Index_IndexedProperty.Merge(m, src)
}
func (m *Index_IndexedProperty) XXX_Size() int {
	return xxx_messageInfo_Index_IndexedProperty.Size(m)
}
func (m *Index_IndexedProperty) XXX_DiscardUnknown() {
	xxx_messageInfo_Index_IndexedProperty.DiscardUnknown(m)
}

var xxx_messageInfo_Index_IndexedProperty proto.InternalMessageInfo

func (m *Index_IndexedProperty) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *Index_IndexedProperty) GetDirection() Index_Direction {
	if m != nil {
		return m.Direction
	}
	return Index_DIRECTION_UNSPECIFIED
}

func init() {
	proto.RegisterEnum("google.datastore.admin.v1.Index_AncestorMode", Index_AncestorMode_name, Index_AncestorMode_value)
	proto.RegisterEnum("google.datastore.admin.v1.Index_Direction", Index_Direction_name, Index_Direction_value)
	proto.RegisterEnum("google.datastore.admin.v1.Index_State", Index_State_name, Index_State_value)
	proto.RegisterType((*Index)(nil), "google.datastore.admin.v1.Index")
	proto.RegisterType((*Index_IndexedProperty)(nil), "google.datastore.admin.v1.Index.IndexedProperty")
}

func init() {
	proto.RegisterFile("google/datastore/admin/v1/index.proto", fileDescriptor_809cc5775e1cdeb3)
}

var fileDescriptor_809cc5775e1cdeb3 = []byte{
	// 492 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x53, 0x6f, 0x6b, 0xd3, 0x4e,
	0x1c, 0xff, 0xa5, 0x6d, 0xb6, 0xe6, 0xfb, 0xdb, 0x66, 0x76, 0x30, 0x48, 0x87, 0xc5, 0x52, 0x50,
	0x8a, 0x60, 0x62, 0xe7, 0x43, 0x45, 0xc8, 0x92, 0x73, 0x46, 0xba, 0x34, 0x5c, 0xa2, 0xa0, 0x4f,
	0xca, 0xd9, 0x3b, 0x42, 0xb4, 0xbd, 0x0b, 0x49, 0x1c, 0xfa, 0x06, 0x7c, 0xea, 0xfb, 0xf0, 0x55,
	0x4a, 0x2e, 0x59, 0x1c, 0xc5, 0xd1, 0x27, 0xe1, 0x7b, 0xf7, 0xfd, 0xfc, 0xe3, 0xc3, 0x05, 0x1e,
	0xa7, 0x52, 0xa6, 0x1b, 0xee, 0x30, 0x5a, 0xd1, 0xb2, 0x92, 0x05, 0x77, 0x28, 0xdb, 0x66, 0xc2,
	0xb9, 0x99, 0x3b, 0x99, 0x60, 0xfc, 0xbb, 0x9d, 0x17, 0xb2, 0x92, 0x68, 0xd4, 0xc0, 0xec, 0x0e,
	0x66, 0x2b, 0x98, 0x7d, 0x33, 0x3f, 0x7f, 0xd8, 0x2a, 0xd0, 0x3c, 0x73, 0xa8, 0x10, 0xb2, 0xa2,
	0x55, 0x26, 0x45, 0xd9, 0x10, 0xa7, 0x3f, 0x75, 0xd0, 0x83, 0x5a, 0x08, 0x8d, 0x01, 0xf2, 0x42,
	0x7e, 0xe1, 0xeb, 0x6a, 0x95, 0x31, 0x4b, 0x9b, 0x68, 0x33, 0x83, 0x18, 0xed, 0x4d, 0xc0, 0xd0,
	0x08, 0x86, 0xca, 0xb0, 0x5e, 0xf6, 0xd5, 0xf2, 0x50, 0x9d, 0x03, 0x86, 0x10, 0x0c, 0xbe, 0x66,
	0x82, 0x59, 0x03, 0x75, 0xad, 0x66, 0x14, 0xc0, 0x90, 0x8a, 0x35, 0xaf, 0xb3, 0x58, 0xfa, 0x44,
	0x9b, 0x9d, 0x5c, 0x3c, 0xb3, 0xef, 0xcd, 0x68, 0xab, 0x04, 0xb6, 0xdb, 0x12, 0xae, 0x25, 0xe3,
	0xa4, 0xa3, 0xa3, 0x48, 0x05, 0xcb, 0x79, 0x51, 0x65, 0xbc, 0xb4, 0x0e, 0x26, 0xfd, 0xd9, 0xff,
	0x17, 0xcf, 0xf7, 0x8a, 0xa9, 0x2f, 0x67, 0x51, 0xc3, 0xfc, 0x41, 0xee, 0x68, 0xa0, 0x57, 0xa0,
	0x97, 0x15, 0xad, 0xb8, 0x75, 0xa8, 0x92, 0x3d, 0xd9, 0x2b, 0x16, 0xd7, 0x68, 0xd2, 0x90, 0xce,
	0x25, 0x3c, 0xd8, 0x11, 0xaf, 0x1b, 0x10, 0x74, 0xcb, 0xdb, 0xd6, 0xd4, 0x8c, 0xde, 0x82, 0xc1,
	0xb2, 0x82, 0xaf, 0xeb, 0xb6, 0xad, 0x9e, 0x32, 0x7a, 0xba, 0xd7, 0xc8, 0xbf, 0x65, 0x90, 0xbf,
	0xe4, 0xe9, 0x3b, 0x38, 0xba, 0x5b, 0x0d, 0x1a, 0xc3, 0xc8, 0x0d, 0x3d, 0x1c, 0x27, 0x4b, 0xb2,
	0xba, 0x5e, 0xfa, 0x78, 0xf5, 0x3e, 0x8c, 0x23, 0xec, 0x05, 0x6f, 0x02, 0xec, 0x9b, 0xff, 0xa1,
	0x21, 0x0c, 0xc2, 0x65, 0x88, 0x4d, 0x0d, 0x9d, 0xc2, 0xb1, 0xbb, 0x58, 0xac, 0x6e, 0xc1, 0xb1,
	0xd9, 0x9b, 0x62, 0x30, 0x3a, 0x0f, 0x34, 0x82, 0x33, 0x3f, 0x20, 0xd8, 0x4b, 0x82, 0x65, 0xb8,
	0x23, 0x72, 0x0c, 0x86, 0x1b, 0x7b, 0x38, 0xf4, 0x83, 0xf0, 0xca, 0xd4, 0xd0, 0x09, 0x80, 0x8f,
	0xbb, 0x73, 0x6f, 0x1a, 0x81, 0xae, 0x3a, 0x41, 0x67, 0x70, 0x1a, 0x27, 0x6e, 0xb2, 0x9b, 0xe1,
	0x08, 0x86, 0x1e, 0xc1, 0x6e, 0xd2, 0xb0, 0x0d, 0xd0, 0x09, 0x76, 0xfd, 0x8f, 0x66, 0xaf, 0x5e,
	0xf8, 0x78, 0x81, 0xd5, 0xa2, 0x5f, 0x2f, 0x30, 0x21, 0x4b, 0x62, 0x0e, 0x2e, 0x7f, 0x69, 0x30,
	0x5e, 0xcb, 0xed, 0xfd, 0x0d, 0x5d, 0x82, 0xaa, 0x28, 0xaa, 0x9f, 0x6d, 0xa4, 0x7d, 0x7a, 0xdd,
	0x02, 0x53, 0xb9, 0xa1, 0x22, 0xb5, 0x65, 0x91, 0x3a, 0x29, 0x17, 0xea, 0x51, 0x3b, 0xcd, 0x8a,
	0xe6, 0x59, 0xf9, 0x8f, 0xff, 0xe6, 0xa5, 0x1a, 0x7e, 0xf7, 0x1e, 0x5d, 0x35, 0x02, 0xde, 0x46,
	0x7e, 0x63, 0xb6, 0xdf, 0xf9, 0xb9, 0xca, 0xef, 0xc3, 0xfc, 0xf3, 0x81, 0x12, 0x7b, 0xf1, 0x27,
	0x00, 0x00, 0xff, 0xff, 0x3b, 0x8d, 0xf4, 0xff, 0x83, 0x03, 0x00, 0x00,
}
