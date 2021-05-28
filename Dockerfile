FROM golang:1.13-alpine as builder
MAINTAINER FullStory Engineering

# create non-priveleged group and user
RUN addgroup -S hauser && adduser -S hauser -G hauser

WORKDIR /tmp/fullstorydev/hauser
COPY VERSION *.go go.* /tmp/fullstorydev/hauser/
COPY client /tmp/fullstorydev/hauser/client
COPY config /tmp/fullstorydev/hauser/config
COPY core /tmp/fullstorydev/hauser/core
COPY internal /tmp/fullstorydev/hauser/internal
COPY warehouse /tmp/fullstorydev/hauser/warehouse

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GO111MODULE=on
RUN go build -o /hauser \
    -ldflags "-w -extldflags \"-static\" -X \"main.version=$(cat VERSION)\"" \
    .

FROM alpine:3.13.5
WORKDIR /hauser
RUN addgroup -S hauser && adduser -S hauser -G hauser && chown -R hauser:hauser /hauser
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /hauser /bin/hauser
USER hauser

ENTRYPOINT ["/bin/hauser"]
