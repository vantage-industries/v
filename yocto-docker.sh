#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

HOST_UID=$(id -u)
HOST_GID=$(id -g)
HOST_USER=$(id -un)

export HOST_UID HOST_GID HOST_USER

case "${1:-}" in
  build)
    echo "Building Yocto Docker image..."
    docker compose build --build-arg USER_ID=$HOST_UID --build-arg GROUP_ID=$HOST_GID --build-arg USER_NAME=$HOST_USER
    ;;
  
  init)
    echo "Initializing VantageOS build environment..."
    docker compose run --rm -v "$SCRIPT_DIR/vantageos.conf.json:/home/builder/vantageos.conf.json:ro" yocto-builder bash -c \
      "./bitbake/bin/bitbake-setup init --non-interactive -L meta-vantageos /home/builder/layers/meta-vantageos /home/builder/vantageos.conf.json vantageos machine/qemuarm64"
    echo ""
    echo "VantageOS build environment initialized!"
    echo "To build: ./yocto-docker.sh bitbake core-image-base"
    ;;
  
  shell)
    docker compose run --rm yocto-builder bash
    ;;
  
  bitbake)
    shift
    echo "Running bitbake command: bitbake $*"
     docker compose run --rm yocto-builder bash -c "
       source ~/bitbake-builds/vantageos-vantageos-machine_qemuarm64/build/init-build-env 2>/dev/null || \
       source ~/bitbake-builds/vantageos/build/init-build-env 2>/dev/null || true
       bitbake "$@"
     "
    ;;
  
     runqemu)
       shift || true
       echo "Running OS in QEMU..."
       docker compose run --rm yocto-builder bash -c "
         source ~/bitbake-builds/vantageos-vantageos-machine_qemuarm64/build/init-build-env 2>/dev/null || \
         source ~/bitbake-builds/vantageos/build/init-build-env 2>/dev/null || true
         # Remove zst file and update symlinks to use uncompressed ext4 image
         cd ~/bitbake-builds/vantageos-vantageos-machine_qemuarm64/build/tmp/deploy/images/qemuarm64/
         rm -f vantageos-image-qemuarm64.rootfs-20260407102403.ext4.zst
         rm -f vantageos-image-qemuarm64.rootfs.ext4.zst
         ln -sf vantageos-image-qemuarm64.rootfs.ext4 vantageos-image-qemuarm64.rootfs.ext4.zst
         runqemu snapshot slirp "$@"
       "
       ;;
  
  *)
    echo "Yocto Docker Build Helper"
    echo ""
    echo "Usage: $0 {build|shell|init|bitbake|runqemu|clean}"
    echo ""
    echo "Commands:"
    echo "  build             Build the Docker image"
    echo "  shell             Start an interactive shell in the container"
    echo "  bitbake <target>  Run bitbake command"
    echo "  runqemu           Run QEMU emulator"
    echo ""
    echo "Examples:"
    echo "  $0 build"
    echo "  $0 shell"
    echo "  $0 bitbake core-image-sato"
    exit 1
    ;;
esac
