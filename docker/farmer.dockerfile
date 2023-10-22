# syntax=docker/dockerfile:1
# Note: this dockerfile must be used from a subfolder
FROM golang:1.21 AS builder
VOLUME /go/src
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download -x

ADD . /app
RUN make farmer


FROM scratch
COPY --from=builder /app/bin/farmer /farmer
ENTRYPOINT ["/farmer"]
