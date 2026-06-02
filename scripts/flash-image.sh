#!/usr/bin/env bash

set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"

default_deploy_root="${repo_root}/bitbake-builds/vantageos/build/tmp/deploy/images"
deploy_root="${DEPLOY_ROOT:-${default_deploy_root}}"
machine="${MACHINE:-raspberrypi5}"
image_path=""
device=""
allow_small_device=false

usage() {
  printf 'Usage: %s [--deploy-root DIR] [--machine NAME] [--image FILE] [--allow-small-device] DEVICE\n' "${0##*/}"
  printf 'Flashes a Yocto .wic.bz2 image with bmaptool.\n'
  printf 'Run this from `nix develop` at the repo root; do not wrap the script in `sudo`.\n'
  printf '\n'
  printf 'Defaults:\n'
  printf '  deploy root: %s\n' "${default_deploy_root}"
  printf '  machine:     %s\n' "${machine}"
}

while (($#)); do
  case "$1" in
    --deploy-root)
      deploy_root="${2:?missing value for --deploy-root}"
      shift 2
      ;;
    --machine)
      machine="${2:?missing value for --machine}"
      shift 2
      ;;
    --image)
      image_path="${2:?missing value for --image}"
      shift 2
      ;;
    --allow-small-device)
      allow_small_device=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*)
      printf 'Unknown option: %s\n' "$1" >&2
      usage >&2
      exit 2
      ;;
    *)
      if [[ -n ${device} ]]; then
        printf 'Unexpected argument: %s\n' "$1" >&2
        usage >&2
        exit 2
      fi
      device="$1"
      shift
      ;;
  esac
done

if [[ -z ${device} ]]; then
  printf 'Missing target device.\n' >&2
  usage >&2
  exit 2
fi

if [[ ${EUID} -eq 0 && -z ${SUDO_USER:-} ]]; then
  printf 'Do not run this script as root. Enter `nix develop` at the repo root and rerun it as your normal user.\n' >&2
  exit 1
fi

if [[ -z ${image_path} ]]; then
  deploy_dir="${deploy_root}/${machine}"
  if [[ ! -d ${deploy_dir} ]]; then
    printf 'Deploy directory not found: %s\n' "${deploy_dir}" >&2
    exit 1
  fi

  shopt -s nullglob
  images=("${deploy_dir}"/*.wic.bz2)
  shopt -u nullglob

  if (( ${#images[@]} == 0 )); then
    printf 'No .wic.bz2 images found in %s\n' "${deploy_dir}" >&2
    exit 1
  fi

  mapfile -t sorted_images < <(printf '%s\n' "${images[@]}" | sort)
  image_path="${sorted_images[$((${#sorted_images[@]} - 1))]}"
fi

if [[ ! -f ${image_path} ]]; then
  printf 'Image not found: %s\n' "${image_path}" >&2
  exit 1
fi

bmap_path="${image_path%.bz2}.bmap"
if [[ ! -f ${bmap_path} ]]; then
  printf 'Bmap file not found: %s\n' "${bmap_path}" >&2
  exit 1
fi

if [[ ! -b ${device} ]]; then
  printf 'Target is not a block device: %s\n' "${device}" >&2
  exit 1
fi

device_size_bytes=""
if command -v blockdev >/dev/null 2>&1; then
  device_size_bytes="$(blockdev --getsize64 "${device}" 2>/dev/null || true)"
fi
if [[ -z ${device_size_bytes} ]]; then
  device_size_bytes="$(lsblk -dnbo SIZE "${device}" 2>/dev/null | tr -d '[:space:]' || true)"
fi

if [[ -n ${device_size_bytes} ]]; then
  device_size_mib=$((device_size_bytes / 1024 / 1024))
  printf 'Target size: %s bytes (%s MiB)\n' "${device_size_bytes}" "${device_size_mib}"

  if (( device_size_bytes < 1073741824 )) && [[ ${allow_small_device} != true ]]; then
    printf 'Refusing to flash a device smaller than 1 GiB. This one reports only %s MiB.\n' "${device_size_mib}" >&2
    printf 'If you are absolutely sure this is intentional, rerun with --allow-small-device.\n' >&2
    exit 1
  fi
fi

if ! command -v bmaptool >/dev/null 2>&1; then
  printf 'bmaptool is not installed or not on PATH. Enter `nix develop` at the repo root first.\n' >&2
  exit 1
fi

printf 'Flashing image: %s\n' "${image_path}"
printf 'Using bmap: %s\n' "${bmap_path}"
printf 'Target device: %s\n' "${device}"

bmaptool_bin="$(command -v bmaptool)"

if [[ ${EUID} -eq 0 ]]; then
  exec "${bmaptool_bin}" copy --bmap "${bmap_path}" "${image_path}" "${device}"
fi

exec sudo -- "${bmaptool_bin}" copy --bmap "${bmap_path}" "${image_path}" "${device}"
