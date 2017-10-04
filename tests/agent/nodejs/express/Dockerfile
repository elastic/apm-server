FROM node:8.1.2
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN mkdir -p /app

WORKDIR /app

COPY . /app

RUN make setup

EXPOSE 8081

ENTRYPOINT cd /app && node app.js
