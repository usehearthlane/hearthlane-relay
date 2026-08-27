FROM golang:1.25-bookworm AS go

FROM opencode-base

USER root

ENV JAVA_HOME=/opt/java/openjdk
ENV ANDROID_HOME=/opt/android-sdk
ENV ANDROID_NDK_HOME=/opt/android-sdk/ndk/${ANDROID_NDK_VERSION}

ENV GOPATH=/home/opencode/go

ENV PATH="${PATH}:/usr/local/go/bin:${GOPATH}/bin"

COPY --from=go /usr/local/go /usr/local/go

RUN mkdir -p "${GOPATH}" \
    && chown -R opencode:opencode "${GOPATH}"

RUN apt-get update && apt-get install -y gh

USER opencode

WORKDIR /workspace

CMD ["opencode"]