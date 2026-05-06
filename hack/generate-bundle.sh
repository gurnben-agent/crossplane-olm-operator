#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

CRD_SOURCE="${PROJECT_ROOT}/config/crd/bases/crossplane.crossplane.io_crossplaneconfigs.yaml"
VERSIONS=("v2.0" "v2.1" "v2.2")

usage() {
    echo "Usage: $0 [--version VERSION]"
    echo ""
    echo "Regenerates OLM bundle manifests from controller-gen output."
    echo "If --version is given, only regenerates that version's bundle."
    echo "Otherwise regenerates all versions: ${VERSIONS[*]}"
    exit 1
}

TARGET_VERSION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) TARGET_VERSION="$2"; shift 2 ;;
        -h|--help) usage ;;
        *) echo "Unknown argument: $1"; usage ;;
    esac
done

if [[ ! -f "${CRD_SOURCE}" ]]; then
    echo "ERROR: CRD source not found at ${CRD_SOURCE}"
    echo "Run 'make manifests' first to generate CRD YAML."
    exit 1
fi

generate_bundle() {
    local version="$1"
    local bundle_dir="${PROJECT_ROOT}/bundle/${version}"

    echo "Generating bundle for ${version}..."

    mkdir -p "${bundle_dir}/manifests" "${bundle_dir}/metadata"

    cp "${CRD_SOURCE}" "${bundle_dir}/manifests/crossplaneconfigs.crossplane.io.crd.yaml"

    if [[ ! -f "${bundle_dir}/manifests/crossplane-olm-operator.clusterserviceversion.yaml" ]]; then
        echo "WARNING: CSV template not found for ${version} — skipping CSV generation."
        echo "  Create it at: ${bundle_dir}/manifests/crossplane-olm-operator.clusterserviceversion.yaml"
    else
        echo "  CSV template exists — CRD snapshot updated."
    fi

    if [[ ! -f "${bundle_dir}/metadata/annotations.yaml" ]]; then
        echo "WARNING: annotations.yaml not found for ${version} — skipping."
    fi

    if [[ ! -f "${bundle_dir}/bundle.Dockerfile" ]]; then
        echo "WARNING: bundle.Dockerfile not found for ${version} — skipping."
    fi

    echo "  Done: ${bundle_dir}"
}

if [[ -n "${TARGET_VERSION}" ]]; then
    generate_bundle "${TARGET_VERSION}"
else
    for v in "${VERSIONS[@]}"; do
        generate_bundle "${v}"
    done
fi

echo ""
echo "Bundle generation complete."
echo "Next steps:"
echo "  - Review CSV templates and update Helm-dependent sections"
echo "  - Run 'operator-sdk bundle validate bundle/<version>' to validate"
echo "  - Run 'opm validate catalog/' to validate the FBC catalog"
