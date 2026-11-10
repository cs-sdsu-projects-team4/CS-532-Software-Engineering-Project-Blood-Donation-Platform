# syntax=docker/dockerfile:1

# ---- build stage ----------------------------------------------------------
# Compile a static binary in the full Go toolchain image.
FROM golang:1.23-alpine AS build
WORKDIR /src

# Copy module files first and download deps. This layer is cached as long as
# go.mod/go.sum don't change, so rebuilds are fast.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build. CGO disabled -> a fully static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# ---- run stage ------------------------------------------------------------
# Tiny final image: just the binary and a non-root user.
FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=build /out/api /usr/local/bin/api
USER appuser
EXPOSE 8080
ENTRYPOINT ["api"]
