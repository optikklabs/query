FROM alpine:3.20
WORKDIR /app

COPY query .
COPY config.yml .

RUN chown -R 1000:1000 /app
USER 1000:1000

# Match default config.yml: HTTP server.port (override with -p when running).
EXPOSE 19090
ENTRYPOINT ["./query"]

