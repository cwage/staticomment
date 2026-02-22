FROM alpine:3.21

RUN apk add --no-cache openssh-server git netcat-openbsd && \
    adduser -D -s /bin/sh git && \
    echo "git:$(head -c 32 /dev/urandom | base64)" | chpasswd && \
    mkdir -p /home/git/.ssh && \
    chown git:git /home/git/.ssh && \
    chmod 700 /home/git/.ssh && \
    ssh-keygen -A && \
    printf '\nPasswordAuthentication no\nPermitRootLogin no\n' >> /etc/ssh/sshd_config

COPY init-repo.sh /init-repo.sh
RUN chmod +x /init-repo.sh

EXPOSE 22

CMD ["/init-repo.sh"]
