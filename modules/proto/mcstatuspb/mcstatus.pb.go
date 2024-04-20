// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v5.26.1
// source: mcstatus.proto

package mcstatuspb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ServerType int32

const (
	ServerType_JAVA    ServerType = 0
	ServerType_BEDROCK ServerType = 1
)

// Enum value maps for ServerType.
var (
	ServerType_name = map[int32]string{
		0: "JAVA",
		1: "BEDROCK",
	}
	ServerType_value = map[string]int32{
		"JAVA":    0,
		"BEDROCK": 1,
	}
)

func (x ServerType) Enum() *ServerType {
	p := new(ServerType)
	*p = x
	return p
}

func (x ServerType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ServerType) Descriptor() protoreflect.EnumDescriptor {
	return file_mcstatus_proto_enumTypes[0].Descriptor()
}

func (ServerType) Type() protoreflect.EnumType {
	return &file_mcstatus_proto_enumTypes[0]
}

func (x ServerType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ServerType.Descriptor instead.
func (ServerType) EnumDescriptor() ([]byte, []int) {
	return file_mcstatus_proto_rawDescGZIP(), []int{0}
}

type ServerStatus struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Host       string     `protobuf:"bytes,1,opt,name=host,proto3" json:"host" xml:"host"`
	Port       int32      `protobuf:"varint,2,opt,name=port,proto3" json:"port" xml:"port"`
	Name       string     `protobuf:"bytes,3,opt,name=name,proto3" json:"name" xml:"name"`
	Motd       string     `protobuf:"bytes,4,opt,name=motd,proto3" json:"motd" xml:"motd"`
	Map        string     `protobuf:"bytes,5,opt,name=map,proto3" json:"map" xml:"map"`
	MaxPlayers int32      `protobuf:"varint,6,opt,name=max_players,json=maxPlayers,proto3" json:"max_players" xml:"max_players"`
	NumPlayers int32      `protobuf:"varint,7,opt,name=num_players,json=numPlayers,proto3" json:"num_players" xml:"num_players"`
	Players    []*Player  `protobuf:"bytes,8,rep,name=players,proto3" json:"players" xml:"players"`
	Version    string     `protobuf:"bytes,9,opt,name=version,proto3" json:"version" xml:"version"`
	Favicon    string     `protobuf:"bytes,10,opt,name=favicon,proto3" json:"favicon" xml:"favicon"`
	ServerType ServerType `protobuf:"varint,11,opt,name=server_type,json=serverType,proto3,enum=mcstatuspb.ServerType" json:"server_type" xml:"server_type"`
}

func (x *ServerStatus) Reset() {
	*x = ServerStatus{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mcstatus_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ServerStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ServerStatus) ProtoMessage() {}

func (x *ServerStatus) ProtoReflect() protoreflect.Message {
	mi := &file_mcstatus_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ServerStatus.ProtoReflect.Descriptor instead.
func (*ServerStatus) Descriptor() ([]byte, []int) {
	return file_mcstatus_proto_rawDescGZIP(), []int{0}
}

func (x *ServerStatus) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *ServerStatus) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *ServerStatus) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ServerStatus) GetMotd() string {
	if x != nil {
		return x.Motd
	}
	return ""
}

func (x *ServerStatus) GetMap() string {
	if x != nil {
		return x.Map
	}
	return ""
}

func (x *ServerStatus) GetMaxPlayers() int32 {
	if x != nil {
		return x.MaxPlayers
	}
	return 0
}

func (x *ServerStatus) GetNumPlayers() int32 {
	if x != nil {
		return x.NumPlayers
	}
	return 0
}

func (x *ServerStatus) GetPlayers() []*Player {
	if x != nil {
		return x.Players
	}
	return nil
}

func (x *ServerStatus) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *ServerStatus) GetFavicon() string {
	if x != nil {
		return x.Favicon
	}
	return ""
}

func (x *ServerStatus) GetServerType() ServerType {
	if x != nil {
		return x.ServerType
	}
	return ServerType_JAVA
}

type Player struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name" xml:"name"`
	Uuid string `protobuf:"bytes,2,opt,name=uuid,proto3" json:"uuid" xml:"uuid"`
}

func (x *Player) Reset() {
	*x = Player{}
	if protoimpl.UnsafeEnabled {
		mi := &file_mcstatus_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Player) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Player) ProtoMessage() {}

func (x *Player) ProtoReflect() protoreflect.Message {
	mi := &file_mcstatus_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Player.ProtoReflect.Descriptor instead.
func (*Player) Descriptor() ([]byte, []int) {
	return file_mcstatus_proto_rawDescGZIP(), []int{1}
}

func (x *Player) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Player) GetUuid() string {
	if x != nil {
		return x.Uuid
	}
	return ""
}

var File_mcstatus_proto protoreflect.FileDescriptor

var file_mcstatus_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x6d, 0x63, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0a, 0x6d, 0x63, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x70, 0x62, 0x22, 0xcd, 0x02, 0x0a,
	0x0c, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x6f, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x6f, 0x73,
	0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x04, 0x70, 0x6f, 0x72, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f, 0x74,
	0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6d, 0x6f, 0x74, 0x64, 0x12, 0x10, 0x0a,
	0x03, 0x6d, 0x61, 0x70, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6d, 0x61, 0x70, 0x12,
	0x1f, 0x0a, 0x0b, 0x6d, 0x61, 0x78, 0x5f, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x18, 0x06,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x6d, 0x61, 0x78, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73,
	0x12, 0x1f, 0x0a, 0x0b, 0x6e, 0x75, 0x6d, 0x5f, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x18,
	0x07, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0a, 0x6e, 0x75, 0x6d, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72,
	0x73, 0x12, 0x2c, 0x0a, 0x07, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x18, 0x08, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x12, 0x2e, 0x6d, 0x63, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x70, 0x62, 0x2e,
	0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x52, 0x07, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x12,
	0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x09, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x66, 0x61, 0x76,
	0x69, 0x63, 0x6f, 0x6e, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x66, 0x61, 0x76, 0x69,
	0x63, 0x6f, 0x6e, 0x12, 0x37, 0x0a, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x5f, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x16, 0x2e, 0x6d, 0x63, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x0a, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x22, 0x30, 0x0a, 0x06,
	0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x75, 0x75,
	0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x75, 0x75, 0x69, 0x64, 0x2a, 0x23,
	0x0a, 0x0a, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x54, 0x79, 0x70, 0x65, 0x12, 0x08, 0x0a, 0x04,
	0x4a, 0x41, 0x56, 0x41, 0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x42, 0x45, 0x44, 0x52, 0x4f, 0x43,
	0x4b, 0x10, 0x01, 0x42, 0x0e, 0x5a, 0x0c, 0x2e, 0x2f, 0x6d, 0x63, 0x73, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_mcstatus_proto_rawDescOnce sync.Once
	file_mcstatus_proto_rawDescData = file_mcstatus_proto_rawDesc
)

func file_mcstatus_proto_rawDescGZIP() []byte {
	file_mcstatus_proto_rawDescOnce.Do(func() {
		file_mcstatus_proto_rawDescData = protoimpl.X.CompressGZIP(file_mcstatus_proto_rawDescData)
	})
	return file_mcstatus_proto_rawDescData
}

var file_mcstatus_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_mcstatus_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_mcstatus_proto_goTypes = []interface{}{
	(ServerType)(0),      // 0: mcstatuspb.ServerType
	(*ServerStatus)(nil), // 1: mcstatuspb.ServerStatus
	(*Player)(nil),       // 2: mcstatuspb.Player
}
var file_mcstatus_proto_depIdxs = []int32{
	2, // 0: mcstatuspb.ServerStatus.players:type_name -> mcstatuspb.Player
	0, // 1: mcstatuspb.ServerStatus.server_type:type_name -> mcstatuspb.ServerType
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_mcstatus_proto_init() }
func file_mcstatus_proto_init() {
	if File_mcstatus_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_mcstatus_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ServerStatus); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_mcstatus_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Player); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_mcstatus_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_mcstatus_proto_goTypes,
		DependencyIndexes: file_mcstatus_proto_depIdxs,
		EnumInfos:         file_mcstatus_proto_enumTypes,
		MessageInfos:      file_mcstatus_proto_msgTypes,
	}.Build()
	File_mcstatus_proto = out.File
	file_mcstatus_proto_rawDesc = nil
	file_mcstatus_proto_goTypes = nil
	file_mcstatus_proto_depIdxs = nil
}
