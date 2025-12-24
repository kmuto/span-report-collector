FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY . .

WORKDIR /app/dist
RUN CGO_ENABLED=0 GOOS=linux go build -o span-report-collector .

FROM alpine:3.20

WORKDIR /

COPY --from=builder /app/dist/span-report-collector /span-report-collector

EXPOSE 4317 4318
ENV SPAN_REPORT_OTLP_ENDPOINT_GRPC=0.0.0.0:4317
ENV SPAN_REPORT_OTLP_ENDPOINT_HTTP=0.0.0.0:4318

ENTRYPOINT ["/span-report-collector"]
