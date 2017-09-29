FROM python:3.6.1
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN mkdir -p /app

WORKDIR /app

RUN git clone https://github.com/opbeat/opbeans-python.git . 

RUN pip install -Ur requirements.txt

RUN bunzip2 -k demo/db.sql.bz2
RUN ./manage.py collectstatic

ADD flaskapp flaskapp
RUN pip install -Ur flaskapp/requirements.txt

EXPOSE 8000

ENV DATABASE_URL=sqlite://./demo/db.sql

CMD python manage.py runserver 0.0.0.0:8000
