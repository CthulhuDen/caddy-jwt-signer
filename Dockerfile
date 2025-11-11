FROM --platform=$BUILDPLATFORM caddy:builder AS builder

WORKDIR /app
COPY go.* signer.go /app/

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} xcaddy build --with github.com/CthulhuDen/caddy-jwt-signer=. --output /usr/bin/caddy


FROM caddy

COPY --from=builder /usr/bin/caddy /usr/bin/caddy
RUN if [[ -z "$(caddy list-modules -s | grep http.handlers.jwt_signer)" ]]; then echo "Module not listed in produced caddy binary" && exit 1; fi
