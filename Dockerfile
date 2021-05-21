FROM golang:1.7
RUN apt update && \
	 apt install -y gettext-base

COPY server-conf/ /server-conf/
RUN mkdir -p /go/src/github.com/fullstorydev/hauser/ && \
	 cp /server-conf/entry.sh /usr/local/bin/entry.sh

ADD . /go/src/github.com/fullstorydev/hauser/
WORKDIR /go/src/github.com/fullstorydev/hauser/
RUN go build -o /usr/local/bin/hauser .
ENTRYPOINT ["entry.sh"]