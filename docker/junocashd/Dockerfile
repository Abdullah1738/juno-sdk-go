FROM ubuntu:24.04

ARG JUNOCASH_VERSION=0.9.13
ARG JUNOCASH_LINUX64_SHA256=38cd45b8766da65cb14a14337279ae9fc06ba37227f03c1eaf2cfbfbc415f06b

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libnuma1 \
    tar \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp
RUN curl -fsSLo "junocash-${JUNOCASH_VERSION}-linux64.tar.gz" "https://github.com/juno-cash/junocash/releases/download/v${JUNOCASH_VERSION}/junocash-${JUNOCASH_VERSION}-linux64.tar.gz" \
 && echo "${JUNOCASH_LINUX64_SHA256}  junocash-${JUNOCASH_VERSION}-linux64.tar.gz" | sha256sum -c - \
 && mkdir -p /tmp/junocash-release \
 && tar -xzf "junocash-${JUNOCASH_VERSION}-linux64.tar.gz" -C /tmp/junocash-release --strip-components=1 \
 && install -m 0755 /tmp/junocash-release/bin/junocashd /usr/local/bin/junocashd \
 && install -m 0755 /tmp/junocash-release/bin/junocash-cli /usr/local/bin/junocash-cli \
 && rm -rf /tmp/*

VOLUME ["/data"]

ENTRYPOINT ["junocashd"]
