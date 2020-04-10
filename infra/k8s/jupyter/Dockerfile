ARG BASE_CONTAINER=jupyter/datascience-notebook

FROM $BASE_CONTAINER

ADD requirements.txt /

RUN pip install jupyter-client --upgrade

RUN pip install -r /requirements.txt
