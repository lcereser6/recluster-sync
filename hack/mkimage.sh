#!/usr/bin/env bash
set -euo pipefail
REG=ghcr.io/lcereser6     # or “docker.io/$USER”

make -C autoscaler container REGISTRY=ghcr.io/lcereser6 IMAGE_TAG=v1.30.0-recluster
