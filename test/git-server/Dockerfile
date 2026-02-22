FROM alpine:3.21

RUN apk add --no-cache openssh-server git netcat-openbsd && \
    adduser -D -s /bin/sh git && \
    echo "git:git" | chpasswd && \
    mkdir -p /home/git/.ssh && \
    chown git:git /home/git/.ssh && \
    chmod 700 /home/git/.ssh && \
    ssh-keygen -A && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config

COPY init-repo.sh /init-repo.sh
RUN chmod +x /init-repo.sh

EXPOSE 22

CMD ["/init-repo.sh"]
