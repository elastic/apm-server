FROM python:3.6.1
MAINTAINER Nicolas Ruflin <ruflin@elastic.co>

RUN mkdir -p /app

WORKDIR /app

RUN git clone https://github.com/opbeat/opbeans-python.git .

RUN pip install -Ur requirements.txt
RUN pip uninstall opbeat -y
RUN pip install git+https://github.com/opbeat/opbeat_python.git@feature/sampling#egg=opbeat

RUN bunzip2 -k demo/db.sql.bz2
RUN ./manage.py collectstatic

EXPOSE 8000

ENTRYPOINT DATABASE_URL=sqlite://./demo/db.sql ./manage.py runserver 0.0.0.0:8000
