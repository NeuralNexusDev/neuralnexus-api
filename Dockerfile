FROM golang:1.24.2-alpine AS build

WORKDIR /app

RUN apk update && apk add --no-cache gcc make
RUN export PROTOC_ZIP=protoc-26.1-linux-x86_64.zip && \
    wget https://github.com/protocolbuffers/protobuf/releases/download/v26.1/$PROTOC_ZIP && \
    unzip -o $PROTOC_ZIP -d /usr/local bin/protoc && \
    unzip -o $PROTOC_ZIP -d /usr/local 'include/*' && \
    rm -f $PROTOC_ZIP

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN make gen

RUN CGO_ENABLED=0 GOOS=linux go build -o apiserver .

FROM alpine:edge AS release-stage

WORKDIR /app

COPY ./public ./public
COPY --from=build /app/apiserver .

CMD ["/app/apiserver"]
