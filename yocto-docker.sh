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
  
  shell)
    docker compose run --rm yocto-builder bash
    ;;
  
  bitbake)
    shift
    echo "Running bitbake command: bitbake $*"
     docker compose run --rm yocto-builder bash -c "
       source ~/bitbake-builds/*/build/init-build-env 2>/dev/null || true
       bitbake \$@
     " -- "$@"
    ;;
  
   runqemu)
     shift || true
     echo "Running OS in QEMU..."
     docker compose run --rm yocto-builder bash -c "
       source ~/bitbake-builds/*/build/init-build-env 2>/dev/null || true
       runqemu \$@
     " -- "$@"
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
