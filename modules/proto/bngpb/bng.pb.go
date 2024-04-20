// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v5.26.1
// source: bng.proto

package bngpb

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

type BeeName struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty" xml:"name,omitempty"`
}

func (x *BeeName) Reset() {
	*x = BeeName{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bng_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BeeName) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeeName) ProtoMessage() {}

func (x *BeeName) ProtoReflect() protoreflect.Message {
	mi := &file_bng_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BeeName.ProtoReflect.Descriptor instead.
func (*BeeName) Descriptor() ([]byte, []int) {
	return file_bng_proto_rawDescGZIP(), []int{0}
}

func (x *BeeName) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type BeeNameSuggestions struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Suggestions []string `protobuf:"bytes,1,rep,name=suggestions,proto3" json:"suggestions,omitempty" xml:"suggestions,omitempty"`
}

func (x *BeeNameSuggestions) Reset() {
	*x = BeeNameSuggestions{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bng_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BeeNameSuggestions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeeNameSuggestions) ProtoMessage() {}

func (x *BeeNameSuggestions) ProtoReflect() protoreflect.Message {
	mi := &file_bng_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BeeNameSuggestions.ProtoReflect.Descriptor instead.
func (*BeeNameSuggestions) Descriptor() ([]byte, []int) {
	return file_bng_proto_rawDescGZIP(), []int{1}
}

func (x *BeeNameSuggestions) GetSuggestions() []string {
	if x != nil {
		return x.Suggestions
	}
	return nil
}

var File_bng_proto protoreflect.FileDescriptor

var file_bng_proto_rawDesc = []byte{
	0x0a, 0x09, 0x62, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x62, 0x6e, 0x67,
	0x70, 0x62, 0x22, 0x1d, 0x0a, 0x07, 0x42, 0x65, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x22, 0x36, 0x0a, 0x12, 0x42, 0x65, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x53, 0x75, 0x67, 0x67,
	0x65, 0x73, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x20, 0x0a, 0x0b, 0x73, 0x75, 0x67, 0x67, 0x65,
	0x73, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x75,
	0x67, 0x67, 0x65, 0x73, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42, 0x09, 0x5a, 0x07, 0x2e, 0x2f, 0x62,
	0x6e, 0x67, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_bng_proto_rawDescOnce sync.Once
	file_bng_proto_rawDescData = file_bng_proto_rawDesc
)

func file_bng_proto_rawDescGZIP() []byte {
	file_bng_proto_rawDescOnce.Do(func() {
		file_bng_proto_rawDescData = protoimpl.X.CompressGZIP(file_bng_proto_rawDescData)
	})
	return file_bng_proto_rawDescData
}

var file_bng_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_bng_proto_goTypes = []interface{}{
	(*BeeName)(nil),            // 0: bngpb.BeeName
	(*BeeNameSuggestions)(nil), // 1: bngpb.BeeNameSuggestions
}
var file_bng_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_bng_proto_init() }
func file_bng_proto_init() {
	if File_bng_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_bng_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BeeName); i {
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
		file_bng_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BeeNameSuggestions); i {
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
			RawDescriptor: file_bng_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_bng_proto_goTypes,
		DependencyIndexes: file_bng_proto_depIdxs,
		MessageInfos:      file_bng_proto_msgTypes,
	}.Build()
	File_bng_proto = out.File
	file_bng_proto_rawDesc = nil
	file_bng_proto_goTypes = nil
	file_bng_proto_depIdxs = nil
}
