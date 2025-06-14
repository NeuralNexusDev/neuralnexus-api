generate:
	protoc -I=./proto --go_out=./modules/proto ./proto/bng.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1,omitempty" xml:"\1,omitempty"/g' ./modules/proto/bngpb/bng.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/problem.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1,omitempty" xml:"\1,omitempty"/g' ./modules/proto/problempb/problem.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/mcstatus.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1" xml:"\1"/g' ./modules/proto/mcstatuspb/mcstatus.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/gss.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1" xml:"\1"/g' ./modules/proto/gsspb/gss.pb.go

	protoc -I=./proto --go_out=./modules/proto ./proto/session.proto
	sed -i 's/json:"\(.*\),omitempty"/json:"\1" xml:"\1" db:"\1"/g' ./modules/proto/sessionpb/session.pb.go

update:
	#go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
