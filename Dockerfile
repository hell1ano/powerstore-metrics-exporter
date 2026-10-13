FROM registry.access.redhat.com/ubi9-minimal:latest
ARG PORT=9010
LABEL maintainer="The CGI Riksbank Team De Tre VISe männen"

WORKDIR /powerstore_exporter

COPY build/powerstore-metrics-exporter .

COPY bulk .
COPY https .

RUN chmod +x ./powerstore-metrics-exporter

EXPOSE ${PORT}

ENTRYPOINT ["/powerstore_exporter/powerstore-metrics-exporter"]
CMD ["-c","/powerstore_exporter/config.yml"]
