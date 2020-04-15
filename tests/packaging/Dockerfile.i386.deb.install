FROM debian:jessie

RUN dpkg --add-architecture i386 && apt update && apt install libc6-i386

ARG apm_server_pkg
COPY $apm_server_pkg $apm_server_pkg
RUN dpkg -i $apm_server_pkg

COPY test.sh test.sh

CMD ./test.sh
