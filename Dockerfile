from alpine:3

ARG BIN

COPY ${BIN} /shimdns
RUN mkdir /config

CMD [ "/shimdns", "-c", "/config/config.yaml" ]