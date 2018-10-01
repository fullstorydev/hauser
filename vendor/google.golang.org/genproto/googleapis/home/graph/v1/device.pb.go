// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/home/graph/v1/device.proto

package graph

import (
	fmt "fmt"
	math "math"

	proto "github.com/golang/protobuf/proto"
	_struct "github.com/golang/protobuf/ptypes/struct"
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

// Third-party partner's device definition.
type Device struct {
	// Third-party partner's device ID.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Hardware type of the device (e.g. light, outlet, etc).
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	// Traits supported by the device.
	Traits []string `protobuf:"bytes,3,rep,name=traits,proto3" json:"traits,omitempty"`
	// Name of the device given by the third party. This includes names given to
	// the device via third party device manufacturer's app, model names for the
	// device, etc.
	Name *DeviceNames `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	// Indicates whether the state of this device is being reported to Google
	// through ReportStateAndNotification call.
	WillReportState bool `protobuf:"varint,5,opt,name=will_report_state,json=willReportState,proto3" json:"will_report_state,omitempty"`
	// If the third-party partner's cloud configuration includes placing devices
	// in rooms, the name of the room can be provided here.
	RoomHint string `protobuf:"bytes,6,opt,name=room_hint,json=roomHint,proto3" json:"room_hint,omitempty"`
	// As in roomHint, for structures that users set up in the partner's system.
	StructureHint string `protobuf:"bytes,7,opt,name=structure_hint,json=structureHint,proto3" json:"structure_hint,omitempty"`
	// Device manufacturer, model, hardware version, and software version.
	DeviceInfo *DeviceInfo `protobuf:"bytes,8,opt,name=device_info,json=deviceInfo,proto3" json:"device_info,omitempty"`
	// Attributes for the traits supported by the device.
	Attributes *_struct.Struct `protobuf:"bytes,9,opt,name=attributes,proto3" json:"attributes,omitempty"`
	// Custom JSON data provided by the manufacturer and attached to QUERY and
	// EXECUTE requests in AoG.
	CustomData           string   `protobuf:"bytes,10,opt,name=custom_data,json=customData,proto3" json:"custom_data,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Device) Reset()         { *m = Device{} }
func (m *Device) String() string { return proto.CompactTextString(m) }
func (*Device) ProtoMessage()    {}
func (*Device) Descriptor() ([]byte, []int) {
	return fileDescriptor_1729f8e53993f499, []int{0}
}

func (m *Device) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Device.Unmarshal(m, b)
}
func (m *Device) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Device.Marshal(b, m, deterministic)
}
func (m *Device) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Device.Merge(m, src)
}
func (m *Device) XXX_Size() int {
	return xxx_messageInfo_Device.Size(m)
}
func (m *Device) XXX_DiscardUnknown() {
	xxx_messageInfo_Device.DiscardUnknown(m)
}

var xxx_messageInfo_Device proto.InternalMessageInfo

func (m *Device) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *Device) GetType() string {
	if m != nil {
		return m.Type
	}
	return ""
}

func (m *Device) GetTraits() []string {
	if m != nil {
		return m.Traits
	}
	return nil
}

func (m *Device) GetName() *DeviceNames {
	if m != nil {
		return m.Name
	}
	return nil
}

func (m *Device) GetWillReportState() bool {
	if m != nil {
		return m.WillReportState
	}
	return false
}

func (m *Device) GetRoomHint() string {
	if m != nil {
		return m.RoomHint
	}
	return ""
}

func (m *Device) GetStructureHint() string {
	if m != nil {
		return m.StructureHint
	}
	return ""
}

func (m *Device) GetDeviceInfo() *DeviceInfo {
	if m != nil {
		return m.DeviceInfo
	}
	return nil
}

func (m *Device) GetAttributes() *_struct.Struct {
	if m != nil {
		return m.Attributes
	}
	return nil
}

func (m *Device) GetCustomData() string {
	if m != nil {
		return m.CustomData
	}
	return ""
}

// Different names for the device.
type DeviceNames struct {
	// Primary name of the device, generally provided by the user.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Additional names provided by the user for the device.
	Nicknames []string `protobuf:"bytes,2,rep,name=nicknames,proto3" json:"nicknames,omitempty"`
	// List of names provided by the partner rather than the user, often
	// manufacturer names, SKUs, etc.
	DefaultNames         []string `protobuf:"bytes,3,rep,name=default_names,json=defaultNames,proto3" json:"default_names,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeviceNames) Reset()         { *m = DeviceNames{} }
func (m *DeviceNames) String() string { return proto.CompactTextString(m) }
func (*DeviceNames) ProtoMessage()    {}
func (*DeviceNames) Descriptor() ([]byte, []int) {
	return fileDescriptor_1729f8e53993f499, []int{1}
}

func (m *DeviceNames) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeviceNames.Unmarshal(m, b)
}
func (m *DeviceNames) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeviceNames.Marshal(b, m, deterministic)
}
func (m *DeviceNames) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeviceNames.Merge(m, src)
}
func (m *DeviceNames) XXX_Size() int {
	return xxx_messageInfo_DeviceNames.Size(m)
}
func (m *DeviceNames) XXX_DiscardUnknown() {
	xxx_messageInfo_DeviceNames.DiscardUnknown(m)
}

var xxx_messageInfo_DeviceNames proto.InternalMessageInfo

func (m *DeviceNames) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *DeviceNames) GetNicknames() []string {
	if m != nil {
		return m.Nicknames
	}
	return nil
}

func (m *DeviceNames) GetDefaultNames() []string {
	if m != nil {
		return m.DefaultNames
	}
	return nil
}

// Device information.
type DeviceInfo struct {
	// Device manufacturer.
	Manufacturer string `protobuf:"bytes,1,opt,name=manufacturer,proto3" json:"manufacturer,omitempty"`
	// Device model.
	Model string `protobuf:"bytes,2,opt,name=model,proto3" json:"model,omitempty"`
	// Device hardware version.
	HwVersion string `protobuf:"bytes,3,opt,name=hw_version,json=hwVersion,proto3" json:"hw_version,omitempty"`
	// Device software version.
	SwVersion            string   `protobuf:"bytes,4,opt,name=sw_version,json=swVersion,proto3" json:"sw_version,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeviceInfo) Reset()         { *m = DeviceInfo{} }
func (m *DeviceInfo) String() string { return proto.CompactTextString(m) }
func (*DeviceInfo) ProtoMessage()    {}
func (*DeviceInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_1729f8e53993f499, []int{2}
}

func (m *DeviceInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeviceInfo.Unmarshal(m, b)
}
func (m *DeviceInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeviceInfo.Marshal(b, m, deterministic)
}
func (m *DeviceInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeviceInfo.Merge(m, src)
}
func (m *DeviceInfo) XXX_Size() int {
	return xxx_messageInfo_DeviceInfo.Size(m)
}
func (m *DeviceInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_DeviceInfo.DiscardUnknown(m)
}

var xxx_messageInfo_DeviceInfo proto.InternalMessageInfo

func (m *DeviceInfo) GetManufacturer() string {
	if m != nil {
		return m.Manufacturer
	}
	return ""
}

func (m *DeviceInfo) GetModel() string {
	if m != nil {
		return m.Model
	}
	return ""
}

func (m *DeviceInfo) GetHwVersion() string {
	if m != nil {
		return m.HwVersion
	}
	return ""
}

func (m *DeviceInfo) GetSwVersion() string {
	if m != nil {
		return m.SwVersion
	}
	return ""
}

func init() {
	proto.RegisterType((*Device)(nil), "google.home.graph.v1.Device")
	proto.RegisterType((*DeviceNames)(nil), "google.home.graph.v1.DeviceNames")
	proto.RegisterType((*DeviceInfo)(nil), "google.home.graph.v1.DeviceInfo")
}

func init() { proto.RegisterFile("google/home/graph/v1/device.proto", fileDescriptor_1729f8e53993f499) }

var fileDescriptor_1729f8e53993f499 = []byte{
	// 470 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x93, 0xc1, 0x6f, 0xd4, 0x3c,
	0x10, 0xc5, 0x95, 0xdd, 0xed, 0x7e, 0x9b, 0xd9, 0xb6, 0x9f, 0xb0, 0x2a, 0xb0, 0xa0, 0x88, 0x74,
	0x11, 0xd2, 0x8a, 0x43, 0xa2, 0x82, 0x10, 0x42, 0x9c, 0xa8, 0x7a, 0x80, 0x0b, 0x42, 0xa9, 0xc4,
	0x81, 0x4b, 0xe4, 0x4d, 0x9c, 0xc4, 0x22, 0xb1, 0x23, 0x7b, 0xb2, 0x2b, 0xee, 0x1c, 0xf8, 0xb3,
	0x51, 0xc6, 0xd9, 0x6e, 0x2b, 0x55, 0xdc, 0xc6, 0xef, 0xfd, 0x3c, 0x1e, 0x3f, 0x27, 0x70, 0x51,
	0x19, 0x53, 0x35, 0x32, 0xa9, 0x4d, 0x2b, 0x93, 0xca, 0x8a, 0xae, 0x4e, 0xb6, 0x97, 0x49, 0x21,
	0xb7, 0x2a, 0x97, 0x71, 0x67, 0x0d, 0x1a, 0x76, 0xe6, 0x91, 0x78, 0x40, 0x62, 0x42, 0xe2, 0xed,
	0xe5, 0xd3, 0xf3, 0x71, 0x23, 0x31, 0x9b, 0xbe, 0x4c, 0x1c, 0xda, 0x3e, 0x47, 0xbf, 0x67, 0xf5,
	0x67, 0x0a, 0xf3, 0x6b, 0x6a, 0xc2, 0x4e, 0x61, 0xa2, 0x0a, 0x1e, 0x44, 0xc1, 0x3a, 0x4c, 0x27,
	0xaa, 0x60, 0x0c, 0x66, 0xf8, 0xab, 0x93, 0x7c, 0x42, 0x0a, 0xd5, 0xec, 0x31, 0xcc, 0xd1, 0x0a,
	0x85, 0x8e, 0x4f, 0xa3, 0xe9, 0x3a, 0x4c, 0xc7, 0x15, 0x7b, 0x07, 0x33, 0x2d, 0x5a, 0xc9, 0x67,
	0x51, 0xb0, 0x5e, 0xbe, 0xb9, 0x88, 0x1f, 0x9a, 0x24, 0xf6, 0xe7, 0x7c, 0x15, 0xad, 0x74, 0x29,
	0xe1, 0xec, 0x35, 0x3c, 0xda, 0xa9, 0xa6, 0xc9, 0xac, 0xec, 0x8c, 0xc5, 0xcc, 0xa1, 0x40, 0xc9,
	0x8f, 0xa2, 0x60, 0xbd, 0x48, 0xff, 0x1f, 0x8c, 0x94, 0xf4, 0x9b, 0x41, 0x66, 0xcf, 0x20, 0xb4,
	0xc6, 0xb4, 0x59, 0xad, 0x34, 0xf2, 0x39, 0xcd, 0xb4, 0x18, 0x84, 0xcf, 0x4a, 0x23, 0x7b, 0x05,
	0xa7, 0xfe, 0x5a, 0xbd, 0x95, 0x9e, 0xf8, 0x8f, 0x88, 0x93, 0x5b, 0x95, 0xb0, 0x4f, 0xb0, 0xf4,
	0x89, 0x65, 0x4a, 0x97, 0x86, 0x2f, 0x68, 0xda, 0xe8, 0x5f, 0xd3, 0x7e, 0xd1, 0xa5, 0x49, 0xa1,
	0xb8, 0xad, 0xd9, 0x7b, 0x00, 0x81, 0x68, 0xd5, 0xa6, 0x47, 0xe9, 0x78, 0x48, 0x1d, 0x9e, 0xec,
	0x3b, 0xec, 0x33, 0x8e, 0x6f, 0xe8, 0xd8, 0xf4, 0x0e, 0xca, 0x5e, 0xc0, 0x32, 0xef, 0x1d, 0x9a,
	0x36, 0x2b, 0x04, 0x0a, 0x0e, 0x34, 0x1f, 0x78, 0xe9, 0x5a, 0xa0, 0x58, 0x15, 0xb0, 0xbc, 0x93,
	0xd0, 0x10, 0x3f, 0x45, 0xea, 0x1f, 0xc4, 0xe7, 0x75, 0x0e, 0xa1, 0x56, 0xf9, 0xcf, 0xa1, 0x76,
	0x7c, 0x42, 0x2f, 0x70, 0x10, 0xd8, 0x4b, 0x38, 0x29, 0x64, 0x29, 0xfa, 0x06, 0x33, 0x4f, 0xf8,
	0x37, 0x3a, 0x1e, 0x45, 0x6a, 0xbb, 0xfa, 0x1d, 0x00, 0x1c, 0xae, 0xc6, 0x56, 0x70, 0xdc, 0x0a,
	0xdd, 0x97, 0x82, 0x42, 0xb2, 0xe3, 0x69, 0xf7, 0x34, 0x76, 0x06, 0x47, 0xad, 0x29, 0x64, 0x33,
	0x7e, 0x09, 0x7e, 0xc1, 0x9e, 0x03, 0xd4, 0xbb, 0x6c, 0x2b, 0xad, 0x53, 0x46, 0xf3, 0x29, 0x59,
	0x61, 0xbd, 0xfb, 0xee, 0x85, 0xc1, 0x76, 0x07, 0x7b, 0xe6, 0x6d, 0xb7, 0xb7, 0xaf, 0x36, 0xc0,
	0x73, 0xd3, 0x3e, 0x98, 0xfc, 0xd5, 0x18, 0xc3, 0xb7, 0x21, 0xcc, 0x1f, 0x1f, 0x46, 0xa4, 0x32,
	0x8d, 0xd0, 0x55, 0x6c, 0x6c, 0x95, 0x54, 0x52, 0x53, 0xd0, 0x89, 0xb7, 0x44, 0xa7, 0xdc, 0xfd,
	0xdf, 0xe2, 0x23, 0x15, 0x9b, 0x39, 0x51, 0x6f, 0xff, 0x06, 0x00, 0x00, 0xff, 0xff, 0xb7, 0x2a,
	0xc2, 0xaf, 0x3b, 0x03, 0x00, 0x00,
}
