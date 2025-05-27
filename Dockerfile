FROM golang:1.24.2-alpine AS build

WORKDIR /app
ENV CGO_ENABLED=0

RUN apk update && apk add --no-cache gcc make

ENV PROTOC_VER=26.1
ENV PROTOC_ZIP=protoc-$PROTOC_VER-linux-x86_64.zip
RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VER/$PROTOC_ZIP && \
    unzip -o $PROTOC_ZIP -d /usr/local bin/protoc && \
    unzip -o $PROTOC_ZIP -d /usr/local 'include/*' && \
    rm -f $PROTOC_ZIP

COPY go.mod go.sum ./
RUN go mod download

COPY Makefile ./
RUN make update

COPY . .

RUN make generate

RUN go build -o apiserver .

FROM alpine:edge AS release-stage

WORKDIR /app

COPY ./public ./public
COPY --from=build /app/apiserver .

CMD ["/app/apiserver"]
