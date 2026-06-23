# syntax=docker/dockerfile:1

FROM eclipse-mosquitto:2.0

COPY mosquitto.conf /mosquitto/config/mosquitto.conf

RUN mkdir -p /mosquitto/data /mosquitto/certs \
    && chown -R mosquitto:mosquitto /mosquitto/data /mosquitto/certs

COPY docker-entrypoint.sh /custom-entrypoint.sh
RUN chmod +x /custom-entrypoint.sh

ENTRYPOINT ["/custom-entrypoint.sh"]
CMD ["/usr/sbin/mosquitto", "-c", "/mosquitto/config/mosquitto.conf"]

EXPOSE 1883 8883
