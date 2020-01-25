FROM python:3.7.6-slim-buster

COPY pyproject.toml /var/poetry/pyproject.toml
COPY poetry.lock /var/poetry/poetry.lock

RUN apt-get update \
 && apt-get install -y curl \
 && curl -sSL https://raw.githubusercontent.com/python-poetry/poetry/master/get-poetry.py | python \
 && cd /var/poetry \
 && ${HOME}/.poetry/bin/poetry export --dev -f requirements.txt >/var/poetry/requirements.txt \
 && pip install -r /var/poetry/requirements.txt
