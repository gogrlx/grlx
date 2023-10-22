# syntax=docker/dockerfile:1
# Note: this dockerfile must be used from a subfolder
FROM golang:1.21 AS builder
VOLUME /go/src
WORKDIR /app
COPY go.mod .
COPY go.sum .

RUN go mod download

ADD . /app
RUN make sprout


FROM busybox
COPY --from=builder /app/bin/sprout sprout
ENTRYPOINT ["/sprout"]
