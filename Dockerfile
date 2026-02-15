FROM ubuntu:24.04 AS base

ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=Europe/Warsaw

RUN apt-get update && apt-get install -y \
    build-essential clang-18 lld-18 cmake ninja-build meson gperf pkg-config \
    python3 ruby-dev unifdef wget xz-utils dpkg-dev \
    libegl1-mesa-dev libxkbcommon-dev libdrm-dev libinput-dev \
    libglib2.0-dev libepoxy-dev libwayland-dev libwayland-egl1 libyaml-dev \
    libavif-dev libunibreak-dev libcairo2-dev liblcms2-dev libgbm-dev \
    libgcrypt20-dev libgnutls28-dev libgstreamer-plugins-base1.0-dev \
    libgstreamer-plugins-bad1.0-dev libharfbuzz-dev libicu-dev libjpeg8-dev \
    libopenjp2-7-dev libsoup-3.0-dev libsqlite3-dev libsystemd-dev \
    libtasn1-6-dev libwebp-dev libwoff-dev libxml2-dev libxslt1-dev \
    wayland-protocols zlib1g-dev libatk1.0-dev libatk-bridge2.0-dev \
    libwayland-bin \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# --- Stage: compile WebKit (cached unless build.mk changes) ---
FROM base AS webkit

WORKDIR /build
COPY build.mk /build/Makefile

RUN make download && make /build/src/.stamp-libwpe && make /build/src/.stamp-webkit

# --- Final image: compile launcher + package at run time ---
FROM webkit

WORKDIR /build

CMD ["make", "/build/src/.stamp-launcher", "package"]
