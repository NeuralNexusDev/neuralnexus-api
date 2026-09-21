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

# --- Test environment (Postgres + Redis containers for integration tests) ---

TEST_POSTGRES_URL ?= postgres://neuralnexus:neuralnexus@localhost:55432/neuralnexus_test
TEST_REDIS_URL ?= redis://localhost:56379

# Not real secrets - fixed fixture values the auth package's init() requires
# to be non-empty (see modules/auth/types.go and modules/auth/session.go).
# Overridable, but any non-empty value works since nothing here is deployed.
TEST_JWT_SECRET ?= test-jwt-secret
TEST_PEPPER ?= test-pepper
TEST_NN_SITE_URL ?= https://neuralnexus.test
TEST_NN_API_URL ?= https://api.neuralnexus.test

test-env-up:
	docker compose -f docker-compose.test.yml up -d --wait

test-env-down:
	docker compose -f docker-compose.test.yml down -v

test-env-logs:
	docker compose -f docker-compose.test.yml logs -f

vet:
	go vet ./...

_go-test:
	go test ./...

# Brings up the test containers, vets and tests against them, then tears
# them down regardless of outcome - the full suite, including the
# Postgres-backed auth store tests that otherwise skip themselves without
# TEST_POSTGRES_URL. Use test-env-up/test-env-down directly to keep the
# containers running across multiple runs during development.
test: test-env-up
	TEST_POSTGRES_URL=$(TEST_POSTGRES_URL) TEST_REDIS_URL=$(TEST_REDIS_URL) JWT_SECRET=$(TEST_JWT_SECRET) PEPPER=$(TEST_PEPPER) NN_SITE_URL=$(TEST_NN_SITE_URL) NN_API_URL=$(TEST_NN_API_URL) $(MAKE) vet _go-test; status=$$?; $(MAKE) test-env-down; exit $$status
