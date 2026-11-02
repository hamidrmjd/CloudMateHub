ARG ALPINE_VERSION

FROM alpine:${ALPINE_VERSION}

RUN --mount=type=cache,id=cloudmatehub-e2e-agent-test,target=/cache,sharing=shared \
    touch /cache/probe
