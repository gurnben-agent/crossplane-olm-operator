#!/usr/bin/env bash
set -euo pipefail

CHART_REPO="https://charts.crossplane.io/stable"
CHART_NAME="crossplane"
VERSIONS=("v2.0" "v2.1" "v2.2")
CHARTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../charts" && pwd)"

changed=0

for ver in "${VERSIONS[@]}"; do
    minor="${ver#v}"
    chart_dir="${CHARTS_DIR}/${ver}"

    echo "==> Checking ${ver}..."

    index_url="${CHART_REPO}/index.yaml"
    upstream_version=$(curl -sL "${index_url}" | \
        grep -A5 "name: ${CHART_NAME}" | \
        grep "appVersion:" | \
        head -1 | \
        awk '{print $2}' | \
        tr -d '"')

    if [[ -z "${upstream_version}" ]]; then
        echo "    WARNING: could not determine upstream version for ${ver}, skipping"
        continue
    fi

    local_version=""
    if [[ -f "${chart_dir}/Chart.yaml" ]]; then
        local_version=$(grep "^appVersion:" "${chart_dir}/Chart.yaml" | awk '{print $2}' | tr -d '"')
    fi

    if [[ "${upstream_version}" == "${local_version}" ]]; then
        echo "    ${ver} is up to date (${local_version})"
        continue
    fi

    echo "    Updating ${ver}: ${local_version} -> ${upstream_version}"

    tmpdir=$(mktemp -d)
    trap "rm -rf ${tmpdir}" EXIT

    curl -sL "${CHART_REPO}/${CHART_NAME}-${minor}.*.tgz" -o "${tmpdir}/chart.tgz" || {
        echo "    WARNING: could not download chart for ${ver}, skipping"
        continue
    }

    rm -rf "${chart_dir:?}/templates" "${chart_dir}/Chart.yaml" "${chart_dir}/values.yaml"
    tar xzf "${tmpdir}/chart.tgz" -C "${chart_dir}" --strip-components=1

    rm -rf "${tmpdir}"
    trap - EXIT

    changed=1
    echo "    Updated ${ver} to ${upstream_version}"
done

if [[ "${changed}" -eq 1 ]]; then
    echo ""
    echo "Charts updated. Review changes and commit."
else
    echo ""
    echo "All charts are up to date."
fi
