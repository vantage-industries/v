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
    docker compose run --rm yocto-builder bash -c \
      "./bitbake/bin/bitbake-setup init --non-interactive -L meta-vantageos /home/builder/layers/meta-vantageos /home/builder/vantageos.conf.json vantageos machine/qemuarm64 --setup-dir-name vantageos"
    echo ""
    echo "VantageOS build environment initialized!"
    echo "To build: ./yocto-docker.sh bitbake vantageos-image"
    ;;

  shell)
    docker compose run --rm yocto-builder bash
    ;;

  bitbake)
    shift
    echo "Running bitbake command: bitbake $*"
     docker compose run --rm yocto-builder bash -c "
       source ~/bitbake-builds/vantageos/build/init-build-env
       bitbake $*
     "
    ;;

  setup)
    shift
    echo "Running bitbake-setup command: bitbake-setup $* --setup-dir ~/bitbake-builds/vantageos"
    docker compose run --rm yocto-builder bash -c "
      ./bitbake/bin/bitbake-setup $* --setup-dir ~/bitbake-builds/vantageos
    "
    ;;

   runqemu)
     shift || true
     echo "Running OS in QEMU..."
     docker compose run --rm --service-ports yocto-builder bash -c "
      source ~/bitbake-builds/vantageos/build/init-build-env
      cd ~/bitbake-builds/vantageos/build/tmp/deploy/images/qemuarm64/
      # Start QEMU with serial on telnet port 4444
      runqemu nographic snapshot slirp serialstdio
     "
     ;;

  *)
    echo "Yocto Docker Build Helper"
    echo ""
    echo "Usage: $0 {build|shell|init|bitbake|setup|runqemu|clean}"
    echo ""
    echo "Commands:"
    echo "  build             Build the Docker image"
    echo "  shell             Start an interactive shell in the container"
    echo "  bitbake <target>  Run bitbake command"
    echo "  setup <args>      Run bitbake-setup command (e.g. status, update)"
    echo "  runqemu           Run QEMU emulator"
    echo ""
    echo "Examples:"
    echo "  $0 build"
    echo "  $0 shell"
    echo "  $0 bitbake core-image-sato"
    echo "  $0 setup status"
    echo "  $0 setup update"
    exit 1
    ;;
esac
