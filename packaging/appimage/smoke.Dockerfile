FROM ubuntu:22.04
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates python3 xvfb xauth xdotool x11-apps dbus-x11 fonts-dejavu-core squashfs-tools \
    hicolor-icon-theme adwaita-icon-theme libgdk-pixbuf-2.0-0 \
    libgl1-mesa-dri libglx-mesa0 libegl-mesa0 libgl1 libegl1 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --create-home smoke
USER 10001:10001
WORKDIR /tmp
ENTRYPOINT ["bash", "/checks/smoke-appimage.sh"]
