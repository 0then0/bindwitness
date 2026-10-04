# Disposable development environment. No packages or services on the host are modified.
ARG BASE=golang:1.25-bookworm@sha256:3b4a11519ad929d1e1d261a12cff056f0c85b735253d7d861346b9c6f8b36437
FROM python:3.11-slim-bookworm@sha256:2333bd330d12de02514770b3585cad313644316047cdee24a7acfdece6de6efb AS cpython
FROM ${BASE}
# Debian's /usr/bin/python3 may have a built-in zlib module. This is the actual
# upstream CPython shared extension from the official Python image instead.
COPY --from=cpython /usr/local /opt/cpython
WORKDIR /work
