gen:
	protoc -I=./proto --go_out=./modules/proto ./proto/bng.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1,omitempty" xml:"\1,omitempty"/g' ./modules/proto/bngpb/bng.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/problem.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1,omitempty" xml:"\1,omitempty"/g' ./modules/proto/problempb/problem.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/mcstatus.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1" xml:"\1"/g' ./modules/proto/mcstatuspb/mcstatus.pb.go
