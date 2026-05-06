FROM registry.access.redhat.com/ubi9/ubi-micro:latest

COPY catalog /configs

LABEL operators.operatorframework.io.index.configs.v1=/configs
