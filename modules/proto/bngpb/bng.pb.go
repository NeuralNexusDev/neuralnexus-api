// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v5.26.1
// source: bng.proto

package bngpb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type BeeName struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Name          string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty" xml:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BeeName) Reset() {
	*x = BeeName{}
	mi := &file_bng_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BeeName) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeeName) ProtoMessage() {}

func (x *BeeName) ProtoReflect() protoreflect.Message {
	mi := &file_bng_proto_msgTypes[0]
	if x != nil {
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
	state         protoimpl.MessageState `protogen:"open.v1"`
	Suggestions   []string               `protobuf:"bytes,1,rep,name=suggestions,proto3" json:"suggestions,omitempty" xml:"suggestions,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *BeeNameSuggestions) Reset() {
	*x = BeeNameSuggestions{}
	mi := &file_bng_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *BeeNameSuggestions) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeeNameSuggestions) ProtoMessage() {}

func (x *BeeNameSuggestions) ProtoReflect() protoreflect.Message {
	mi := &file_bng_proto_msgTypes[1]
	if x != nil {
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

const file_bng_proto_rawDesc = "" +
	"\n" +
	"\tbng.proto\x12\x05bngpb\"\x1d\n" +
	"\aBeeName\x12\x12\n" +
	"\x04name\x18\x01 \x01(\tR\x04name\"6\n" +
	"\x12BeeNameSuggestions\x12 \n" +
	"\vsuggestions\x18\x01 \x03(\tR\vsuggestionsB\tZ\a./bngpbb\x06proto3"

var (
	file_bng_proto_rawDescOnce sync.Once
	file_bng_proto_rawDescData []byte
)

func file_bng_proto_rawDescGZIP() []byte {
	file_bng_proto_rawDescOnce.Do(func() {
		file_bng_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_bng_proto_rawDesc), len(file_bng_proto_rawDesc)))
	})
	return file_bng_proto_rawDescData
}

var file_bng_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_bng_proto_goTypes = []any{
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_bng_proto_rawDesc), len(file_bng_proto_rawDesc)),
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
	file_bng_proto_goTypes = nil
	file_bng_proto_depIdxs = nil
}
