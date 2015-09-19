FROM scratch
MAINTAINER Ric Lister <rlister@gmail.com>

ADD certs/ca-certificates.crt /etc/ssl/certs/
ADD let-me-in /

ENTRYPOINT [ "/let-me-in" ]
