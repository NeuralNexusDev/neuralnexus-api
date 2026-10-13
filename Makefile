.PHONY: generate update test-env-up test-env-down test-env-logs vet test-go test

PROTO_FILES := bng problem mcstatus gss session

PROTO_TAGS_bng := json:"\1,omitempty" xml:"\1,omitempty"
PROTO_TAGS_problem := json:"\1,omitempty" xml:"\1,omitempty"
PROTO_TAGS_mcstatus := json:"\1" xml:"\1"
PROTO_TAGS_gss := json:"\1" xml:"\1"
PROTO_TAGS_session := json:"\1" xml:"\1" db:"\1"

generate:
	$(foreach p,$(PROTO_FILES),protoc -I=./proto --plugin=protoc-gen-go=$$(go tool -n protoc-gen-go) --go_out=./modules/proto ./proto/$(p).proto && sed -i 's/json:"\(.*\),omitempty"/$(PROTO_TAGS_$(p))/g' ./modules/proto/$(p)pb/$(p).pb.go &&) true

update:
	go get -tool google.golang.org/protobuf/cmd/protoc-gen-go@$$(go list -m -f '{{.Version}}' google.golang.org/protobuf)

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
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "$$out"; exit 1; fi
	go vet ./...

test-go: export TEST_POSTGRES_URL := $(TEST_POSTGRES_URL)
test-go: export TEST_REDIS_URL := $(TEST_REDIS_URL)
test-go: export JWT_SECRET := $(TEST_JWT_SECRET)
test-go: export PEPPER := $(TEST_PEPPER)
test-go: export NN_SITE_URL := $(TEST_NN_SITE_URL)
test-go: export NN_API_URL := $(TEST_NN_API_URL)
test-go:
	go test -race ./...

test:
	$(MAKE) test-env-up && $(MAKE) vet test-go; status=$$?; $(MAKE) test-env-down; exit $$status
