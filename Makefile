.PHONY: generate update build dev test-env-up test-env-down test-env-logs vet test-go test

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
	go get -tool github.com/air-verse/air@latest

build:
	go build -o build/apiserver .

# Runs against the containers from `make test-env-up`. Values already set in
# the shell win. Secrets for the external services (Twitch, Discord OAuth,
# Microsoft, Steam, GitHub, the CDN) are left to the shell.
# DISCORD_PUBLIC_KEY and the S3_* values are placeholders the server only
# needs to be non-empty at start. Requests to those services fail.
dev: export NN_API_URL ?= http://localhost:8080
dev: export NN_SITE_URL ?= http://localhost:3000
dev: export ADDRESS ?= 127.0.0.1:8080
dev: export JWT_SECRET ?= test-jwt-secret
dev: export PEPPER ?= test-pepper
dev: export DATABASE_URL ?= postgres://neuralnexus:neuralnexus@localhost:55432
dev: export DATABASE_URL_2 ?= postgres://neuralnexus:neuralnexus@localhost:55432
dev: export REDIS_ADDRESS ?= localhost:56379
dev: export DISCORD_PUBLIC_KEY ?= 0000000000000000000000000000000000000000000000000000000000000000
dev: export S3_API_URL ?= localhost:9000
dev: export S3_ACCESS_KEY_MCA_TEXTURE ?= dev-access-key
dev: export S3_SECRET_KEY_MCA_TEXTURE ?= dev-secret-key
dev:
	go tool air

TEST_POSTGRES_URL ?= postgres://neuralnexus:neuralnexus@localhost:55432/neuralnexus
TEST_REDIS_URL ?= redis://localhost:56379

# Not real secrets - fixed fixture values that the account and session packages
# require to be non-empty (see modules/auth/account/types.go for PEPPER and
# modules/auth/session/session.go for JWT_SECRET, NN_SITE_URL and NN_API_URL).
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
