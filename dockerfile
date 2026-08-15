# syntax=docker/dockerfile:1.6

FROM golang:1.22.4-bookworm AS builder

ARG TARGETOS=
ARG TARGETARCH=
ARG TARGETVARIANT=
ARG VOSK_VERSION=0.3.45
ARG COMMIT_SHA=unknown

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1

RUN set -eux; \ 
    apt-get update; \ 
    apt-get install -y --no-install-recommends \ 
        build-essential \ 
        ca-certificates \ 
        curl \ 
        git \ 
        libasound2-dev \ 
        libopus-dev \ 
        libopusfile-dev \ 
        libsox-dev \ 
        libsodium-dev \ 
        pkg-config \ 
        unzip \ 
        wget \ 
        gcc-aarch64-linux-gnu \ 
        g++-aarch64-linux-gnu \ 
        gcc-arm-linux-gnueabihf \ 
        g++-arm-linux-gnueabihf; \ 
    if [ "${TARGETARCH:-}" = "arm64" ]; then \
        dpkg --add-architecture arm64; \
        apt-get update; \
        apt-get install -y --no-install-recommends \
            libasound2-dev:arm64 \
            libopus-dev:arm64 \
            libopusfile-dev:arm64 \
            libsox-dev:arm64 \
            libsodium-dev:arm64; \
    elif [ "${TARGETARCH:-}" = "arm" ]; then \
        dpkg --add-architecture armhf; \ 
        apt-get update; \ 
        apt-get install -y --no-install-recommends \ 
            libasound2-dev:armhf \ 
            libopus-dev:armhf \ 
            libopusfile-dev:armhf \ 
            libsox-dev:armhf \ 
            libsodium-dev:armhf; \ 
    fi; \ 
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY chipper/go.mod chipper/go.sum ./chipper/

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd chipper && go mod download

COPY . .

RUN find . -type f -name '*.sh' -exec sed -i 's/\r$//' {} +

RUN set -eux; \
    ARCH="${TARGETARCH:-}"; \
    if [ -z "${ARCH}" ]; then \
        case "$(dpkg --print-architecture)" in \
            armhf|armel) ARCH="arm" ;; \
            *) ARCH="$(dpkg --print-architecture)" ;; \
        esac; \
    fi; \
    case "${ARCH}" in \
        amd64) VOSK_PKG="vosk-linux-x86_64-${VOSK_VERSION}.zip" ;; \
        arm64) VOSK_PKG="vosk-linux-aarch64-${VOSK_VERSION}.zip" ;; \
        arm) VOSK_PKG="vosk-linux-armv7l-${VOSK_VERSION}.zip" ;; \
        *) echo "Unsupported architecture: ${ARCH}" >&2; exit 1 ;; \
    esac; \
    mkdir -p /opt/vosk; \
    curl -fsSL -o /tmp/vosk.zip "https://github.com/alphacep/vosk-api/releases/download/v${VOSK_VERSION}/${VOSK_PKG}"; \
    unzip /tmp/vosk.zip -d /tmp; \
    VOSK_DIR="$(find /tmp -maxdepth 1 -type d -name 'vosk*' | head -n1)"; \
    mv "${VOSK_DIR}" /opt/vosk/libvosk; \
    rm -rf /tmp/vosk.zip /tmp/vosk-*

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    set -eux; \
    BUILD_COMMIT="${COMMIT_SHA}"; \
    if [ "${BUILD_COMMIT}" = "unknown" ] || [ -z "${BUILD_COMMIT}" ]; then \
        BUILD_COMMIT=$(git rev-parse --short HEAD || echo "dev"); \
    fi; \
    GOOS_VALUE="${TARGETOS:-}"; \
    GOARCH_VALUE="${TARGETARCH:-}"; \
    if [ -z "${GOOS_VALUE}" ]; then \
        GOOS_VALUE=$(go env GOOS); \
    fi; \
    if [ -z "${GOARCH_VALUE}" ]; then \
        GOARCH_VALUE=$(go env GOARCH); \
    fi; \
    if [ "${TARGETARCH:-}" = "arm" ]; then \
        GOARM_RAW="${TARGETVARIANT:-}"; \
        GOARM_VALUE="${GOARM_RAW#v}"; \
        if [ -z "${GOARM_VALUE}" ]; then GOARM_VALUE=7; fi; \
        export GOARM=${GOARM_VALUE}; \
    fi; \
    mkdir -p /build; \
    cd /src/chipper; \
    CC_VALUE=""; \
    CXX_VALUE=""; \
    if [ "${GOARCH_VALUE}" = "arm64" ]; then \
        CC_VALUE="aarch64-linux-gnu-gcc"; \
        CXX_VALUE="aarch64-linux-gnu-g++"; \
    elif [ "${GOARCH_VALUE}" = "arm" ]; then \
        CC_VALUE="arm-linux-gnueabihf-gcc"; \
        CXX_VALUE="arm-linux-gnueabihf-g++"; \
    fi; \
    if [ -n "${CC_VALUE}" ]; then \
        export CC=${CC_VALUE}; \
    fi; \
    if [ -n "${CXX_VALUE}" ]; then \
        export CXX=${CXX_VALUE}; \
    fi; \
    LIB_DIR="/usr/lib/x86_64-linux-gnu"; \
    if [ "${GOARCH_VALUE}" = "arm64" ]; then \
        LIB_DIR="/usr/lib/aarch64-linux-gnu"; \
    elif [ "${GOARCH_VALUE}" = "arm" ]; then \
        LIB_DIR="/usr/lib/arm-linux-gnueabihf"; \
    fi; \
    export PKG_CONFIG_PATH="${LIB_DIR}/pkgconfig"; \
    export PKG_CONFIG_LIBDIR="${PKG_CONFIG_PATH}"; \
    export CGO_CFLAGS="-I/opt/vosk/libvosk"; \
    export CGO_LDFLAGS="-L/opt/vosk/libvosk -L${LIB_DIR} -lvosk -lopus -lopusfile -lsodium -lasound -ldl -lpthread"; \
    GOOS=${GOOS_VALUE} GOARCH=${GOARCH_VALUE} \
    go build -tags "nolibopusfile" -ldflags "-s -w -X github.com/kercre123/wire-pod/chipper/pkg/vars.CommitSHA=${BUILD_COMMIT}" \
        -o /build/chipper ./cmd/vosk


FROM ubuntu:22.04 AS runtime

ARG COMMIT_SHA=unknown

ENV DEBIAN_FRONTEND=noninteractive \
    WIREPOD_DATA_DIR=/data \
    WIREPOD_IMAGES_DIR=/images \
    WIREPOD_IN_DOCKER=1 \
    LD_LIBRARY_PATH=/opt/vosk/libvosk

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        avahi-daemon \
        avahi-utils \
        bash \
        ca-certificates \
        curl \
        git \
        iproute2 \
        libasound2 \
        libatomic1 \
        libcap2-bin \
        libopus0 \
        libopusfile0 \
        libsodium23 \
        libsox3 \
        tzdata \
        unzip \
        util-linux \
        wget \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 wirepod \
    && useradd --uid 10001 --gid wirepod --create-home --shell /usr/sbin/nologin wirepod

WORKDIR /opt/wire-pod

COPY --from=builder /opt/vosk/libvosk /opt/vosk/libvosk
COPY --from=builder /src /opt/wire-pod
COPY --from=builder /build/chipper /opt/wire-pod/chipper/chipper

# CAP_NET_BIND_SERVICE lets the unprivileged "wirepod" user (which
# entrypoint.sh drops to at container start, after fixing up bind-mount
# ownership -- see there) bind the privileged 80/443 ports without the
# process needing to run as root.
RUN chmod +x \
        /opt/wire-pod/setup.sh \
        /opt/wire-pod/update.sh \
        /opt/wire-pod/chipper/start.sh \
        /opt/wire-pod/docker/entrypoint.sh \
    && setcap 'cap_net_bind_service=+ep' /opt/wire-pod/chipper/chipper \
    && mkdir -p /data /images \
    && chown -R wirepod:wirepod /opt/wire-pod /data /images /home/wirepod

VOLUME ["/data", "/images"]

EXPOSE 80 443 8080 8084

LABEL org.opencontainers.image.revision="${COMMIT_SHA}"

# Deliberately no USER directive here: the container starts as root so
# entrypoint.sh can fix ownership of bind-mounted host directories
# (docker-compose's ./data, ./images, which Docker creates owned by
# root) before dropping to the unprivileged "wirepod" user itself via
# setpriv. See docker/entrypoint.sh.
ENTRYPOINT ["/opt/wire-pod/docker/entrypoint.sh"]
CMD ["/opt/wire-pod/chipper/start.sh"]
