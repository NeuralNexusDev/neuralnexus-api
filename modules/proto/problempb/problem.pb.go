// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        v5.26.1
// source: problem.proto

package problempb

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

type Problem struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type     string `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty" xml:"type,omitempty"`
	Status   int32  `protobuf:"varint,2,opt,name=status,proto3" json:"status,omitempty" xml:"status,omitempty"`
	Title    string `protobuf:"bytes,3,opt,name=title,proto3" json:"title,omitempty" xml:"title,omitempty"`
	Detail   string `protobuf:"bytes,4,opt,name=detail,proto3" json:"detail,omitempty" xml:"detail,omitempty"`
	Instance string `protobuf:"bytes,5,opt,name=instance,proto3" json:"instance,omitempty" xml:"instance,omitempty"`
}

func (x *Problem) Reset() {
	*x = Problem{}
	if protoimpl.UnsafeEnabled {
		mi := &file_problem_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Problem) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Problem) ProtoMessage() {}

func (x *Problem) ProtoReflect() protoreflect.Message {
	mi := &file_problem_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Problem.ProtoReflect.Descriptor instead.
func (*Problem) Descriptor() ([]byte, []int) {
	return file_problem_proto_rawDescGZIP(), []int{0}
}

func (x *Problem) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Problem) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Problem) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *Problem) GetDetail() string {
	if x != nil {
		return x.Detail
	}
	return ""
}

func (x *Problem) GetInstance() string {
	if x != nil {
		return x.Instance
	}
	return ""
}

var File_problem_proto protoreflect.FileDescriptor

var file_problem_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x70, 0x72, 0x6f, 0x62, 0x6c, 0x65, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x09, 0x70, 0x72, 0x6f, 0x62, 0x6c, 0x65, 0x6d, 0x70, 0x62, 0x22, 0x7f, 0x0a, 0x07, 0x50, 0x72,
	0x6f, 0x62, 0x6c, 0x65, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61,
	0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x64, 0x65, 0x74, 0x61, 0x69,
	0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x12,
	0x1a, 0x0a, 0x08, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x42, 0x0d, 0x5a, 0x0b, 0x2e,
	0x2f, 0x70, 0x72, 0x6f, 0x62, 0x6c, 0x65, 0x6d, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_problem_proto_rawDescOnce sync.Once
	file_problem_proto_rawDescData = file_problem_proto_rawDesc
)

func file_problem_proto_rawDescGZIP() []byte {
	file_problem_proto_rawDescOnce.Do(func() {
		file_problem_proto_rawDescData = protoimpl.X.CompressGZIP(file_problem_proto_rawDescData)
	})
	return file_problem_proto_rawDescData
}

var file_problem_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_problem_proto_goTypes = []interface{}{
	(*Problem)(nil), // 0: problempb.Problem
}
var file_problem_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_problem_proto_init() }
func file_problem_proto_init() {
	if File_problem_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_problem_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Problem); i {
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
			RawDescriptor: file_problem_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_problem_proto_goTypes,
		DependencyIndexes: file_problem_proto_depIdxs,
		MessageInfos:      file_problem_proto_msgTypes,
	}.Build()
	File_problem_proto = out.File
	file_problem_proto_rawDesc = nil
	file_problem_proto_goTypes = nil
	file_problem_proto_depIdxs = nil
}
