FROM k8s.gcr.io/build-image/debian-base:v2.1.3
RUN echo "deb http://deb.debian.org/debian testing non-free contrib main" >> /etc/apt/sources.list &&\
    echo "deb http://deb.debian.org/debian unstable non-free contrib main" >> /etc/apt/sources.list && \
    apt-get -y update && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN export DEBIAN_FRONTEND=noninteractive && clean-install libc-bin=2.31-9 libgnutls30=3.7.0-5 libudev1=247.2-4 udev=247.2-4 libsqlite3-0=3.34.0-1 && apt-get clean && rm -rf /var/lib/apt/lists/*
RUN export DEBIAN_FRONTEND=noninteractive && clean-install ca-certificates e2fsprogs mount udev util-linux xfsprogs && apt-get clean && rm -rf /var/lib/apt/lists/*
WORKDIR /go/src/github.com/outscale-dev/osc-bsu-csi-driver
COPY ./bin/osc-bsu-csi-driver /bin/osc-bsu-csi-driver

ENTRYPOINT ["/bin/osc-bsu-csi-driver"]
