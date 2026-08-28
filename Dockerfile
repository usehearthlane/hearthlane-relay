FROM golang:1.25-bookworm AS go

FROM opencode-base

USER root

ENV GOPATH=/home/opencode/go

ENV PATH="${PATH}:/usr/local/go/bin:${GOPATH}/bin"

COPY --from=go /usr/local/go /usr/local/go

RUN mkdir -p "${GOPATH}" \
    && chown -R opencode:opencode "${GOPATH}"

RUN apt-get update && apt-get install -y gh

USER opencode

WORKDIR /workspace

CMD ["opencode"]