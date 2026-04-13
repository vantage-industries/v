# VantageOS

Custom Linux distribution built with Yocto.

## Quick Start

```bash
# 1. Clone with submodules
git clone --recursive <repo-url>
cd vantageos

# 2. Build Docker image
./yocto-docker.sh build

# 3. Initialize VantageOS build environment
./yocto-docker.sh init

# 4. Build (takes 30-60 min first time)
#./yocto-docker.sh bitbake core-image-base
./yocto-docker.sh bitbake vantageos-image

# 5. Run in QEMU
./yocto-docker.sh runqemu
```

## Available Machines

- `qemuarm64` - QEMU ARM64 emulator (default)

To build for a different machine, edit `bitbake-builds/vantageos/build/conf/local.conf` and change the `MACHINE` variable.

## Requirements

- Docker & Docker Compose
- ~50GB free disk space
- ~8GB RAM

## Structure

- `bitbake/` - Yocto build system (git submodule)
- `layers/` - Custom Yocto layers
- `vantageos.conf.json` - VantageOS build configuration (GitHub mirrors, custom setup)
- `bitbake-builds/` - Build output directory

## Updating

```bash
git pull
git submodule update --init
```

## License

See individual layer directories for license information.
