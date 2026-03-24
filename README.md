# VantageOS

Custom Linux distribution built with Yocto.

## Quick Start

```bash
# 1. Clone with submodules
git clone --recursive <repo-url>
cd vantageos

# 2. Build Docker image
./yocto-docker.sh build

# 3. Initialize bitbake
docker compose run --rm yocto-builder bash -c \
  "./bitbake/bin/bitbake-setup init --non-interactive oe-nodistro-whinlatter nodistro machine/qemuarm64"

# 4. Build (takes 30-60 min first time)
docker compose run --rm yocto-builder bash -c \
  "source bitbake-builds/oe-nodistro-whinlatter/build/init-build-env && bitbake core-image-base"

# 5. Run in QEMU
./yocto-docker.sh runqemu
```

## Requirements

- Docker & Docker Compose
- ~50GB free disk space
- ~8GB RAM

## Structure

- `bitbake/` - Yocto build system (git submodule)
- `layers/` - Custom Yocto layers
- `config/` - Build configurations
- `bitbake-builds/` - Build output directory

## Updating

```bash
git pull
git submodule update --init
```

## License

See individual layer directories for license information.
