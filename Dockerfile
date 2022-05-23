# The base go-image
FROM golang:alpine AS builder
RUN mkdir /app
COPY . /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux go build -o /server .

# production image
FROM busybox
RUN mkdir /app
EXPOSE 8000
WORKDIR /app

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /server /app/
COPY *.toml /app/


CMD [ "/app/server" ]
