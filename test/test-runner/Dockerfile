FROM alpine:3.21

RUN apk add --no-cache curl git openssh-client

COPY run-tests.sh /run-tests.sh
RUN chmod +x /run-tests.sh

CMD ["/run-tests.sh"]
