#!/bin/sh
set -eu

PERSIST_DIR="/data/suricata"
CONFIG_DIR="/etc/suricata"

mkdir -p "$PERSIST_DIR"

if [ -L "$CONFIG_DIR" ]; then
    exit 0
fi

if [ -d "$CONFIG_DIR" ] && [ -z "$(ls -A "$PERSIST_DIR")" ]; then
    cp -a "$CONFIG_DIR"/. "$PERSIST_DIR"/
fi

if [ -d "$CONFIG_DIR" ]; then
    rm -rf "$CONFIG_DIR"
fi

ln -s "$PERSIST_DIR" "$CONFIG_DIR"
