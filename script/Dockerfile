FROM python:2-slim

COPY requirements.txt /

RUN pip install -r requirements.txt

COPY *.py /

CMD python read_from_intake.py --output-file - | python intake_data_to_elasticsearch.py --input-file -