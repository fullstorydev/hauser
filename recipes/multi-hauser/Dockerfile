FROM alpine:latest
RUN apk update && apk add --no-cache \
  supervisor \
  curl \
  gettext

ARG HAUSER_VERSION=1.0.0
RUN curl -L >hauser.tar.gz https://github.com/fullstorydev/hauser/releases/download/v${HAUSER_VERSION}/hauser_${HAUSER_VERSION}_linux_x86_64.tar.gz \
 && tar -xzvf hauser.tar.gz -C /usr/bin \
 && rm hauser.tar.gz

RUN mkdir -p /var/log/supervisor && \
  mkdir -p /etc/supervisor/conf.d && \
  mkdir -p /conf && \
  mkdir -p /tmpl

COPY supervisord.conf.tmpl supervisor-program.conf.tmpl hauser-config.toml.tmpl /tmpl/

COPY start.sh /

ENTRYPOINT ["/start.sh"]
