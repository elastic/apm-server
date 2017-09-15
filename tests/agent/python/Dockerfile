FROM python:3.6.1
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN mkdir -p /app

ADD . /app

WORKDIR /app

RUN pip install -r requirements.txt

ENTRYPOINT python -m app run
