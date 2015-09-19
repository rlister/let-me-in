FROM scratch
MAINTAINER Ric Lister <rlister@gmail.com>

ADD let-me-in /

ENTRYPOINT [ "/let-me-in" ]
