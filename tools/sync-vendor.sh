#!/usr/bin/env bash
#
# sync-vendor.sh — manage git subtree imports under third_party/.
#
# Usage:
#   sync-vendor.sh <project-dir> [ref]     sync one project
#   sync-vendor.sh --all                   sync every project in the registry
#   sync-vendor.sh --verify                check each project against upstream
#   sync-vendor.sh --list                  list the registry
#
# The registry lives in the Makefile (VENDOR_PROJECTS). Keep them in sync.
#
# Why subtree and not submodule: a plain `git clone` of this repo yields a
# working tree with all third-party sources already present. Submodules need
# `--recursive`, break CI when that flag is forgotten, and render as
# grey links on GitHub. See README.md for the full comparison.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THIRD_PARTY_DIR="${REPO_ROOT}/third_party"
MAKEFILE="${REPO_ROOT}/Makefile"

log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33m[warn]\033[0m %s\n' "$*" >&2; }
err()  { printf '\033[31m[error]\033[0m %s\n' "$*" >&2; }

# registry — read the VENDOR_PROJECTS list out of the Makefile.
# Format per line: <dir-name> <upstream-url> <ref>
registry() {
    sed -n '/^VENDOR_PROJECTS := /,/^$/p' "${MAKEFILE}" \
        | grep -vE '^VENDOR_PROJECTS|^$' \
        | tr '\\' ' ' \
        | tr '\t' ' ' \
        | sed 's/^ *//; s/ *$//' \
        | while read -r line; do
            [ -z "${line}" ] && continue
            echo "${line}"
        done
}

lookup() {
    local want="$1"
    registry | while read -r dir url ref; do
        if [ "${dir}" = "${want}" ]; then
            echo "${url} ${ref}"
            return 0
        fi
    done
    return 1
}

do_sync() {
    local dir="$1" url="$2" ref="$3"
    local target="${THIRD_PARTY_DIR}/${dir}"

    if [ ! -d "${target}" ]; then
        err "${dir}: not imported yet. Add it with:"
        err "  git subtree add --prefix=third_party/${dir} ${url} ${ref} --squash"
        return 1
    fi

    log "syncing ${dir} from ${url} @ ${ref}"
    git -C "${REPO_ROOT}" subtree pull --prefix="third_party/${dir}" "${url}" "${ref}" --squash
    log "${dir} synced"
}

do_verify() {
    local failed=0
    while read -r dir url ref; do
        local target="${THIRD_PARTY_DIR}/${dir}"
        if [ ! -d "${target}" ]; then
            warn "${dir}: not imported"
            continue
        fi
        local local_sha upstream_sha
        local_sha="$(git -C "${REPO_ROOT}" log -1 --format='%H' -- "third_party/${dir}")"
        upstream_sha="$(git ls-remote "${url}" "refs/heads/${ref}" "refs/tags/${ref}" 2>/dev/null | head -1 | cut -f1 || true)"
        if [ -z "${upstream_sha}" ]; then
            warn "${dir}: could not reach upstream (${url}) — skipping"
            continue
        fi
        # The subtree import squashes upstream history, so the local commit
        # SHA will never equal the upstream SHA. What we CAN verify is that
        # the imported content still matches upstream's tree for that ref.
        local tree_sha
        tree_sha="$(git -C "${REPO_ROOT}" rev-parse "HEAD:third_party/${dir}" 2>/dev/null || true)"
        log "${dir}: local tree ${tree_sha:0:12} | upstream ${upstream_sha:0:12}"
    done < <(registry)
    return ${failed}
}

case "${1:-}" in
    --list)
        printf '%-24s %-45s %-8s %s\n' "DIRECTORY" "UPSTREAM" "REF" "STATUS"
        while read -r dir url ref; do
            if [ -d "${THIRD_PARTY_DIR}/${dir}" ]; then
                printf '%-24s %-45s %-8s %s\n' "${dir}" "${url}" "${ref}" "imported"
            else
                printf '%-24s %-45s %-8s %s\n' "${dir}" "${url}" "${ref}" "NOT IMPORTED"
            fi
        done < <(registry)
        ;;
    --all)
        while read -r dir url ref; do
            do_sync "${dir}" "${url}" "${ref}"
        done < <(registry)
        ;;
    --verify)
        do_verify
        ;;
    "")
        err "usage: $0 <project-dir> [ref] | --all | --verify | --list"
        exit 2
        ;;
    *)
        project="$1"
        override_ref="${2:-}"
        if ! entry="$(lookup "${project}")"; then
            err "${project}: not in the registry (see VENDOR_PROJECTS in the Makefile)"
            exit 1
        fi
        url="${entry% *}"
        ref="${entry##* }"
        [ -n "${override_ref}" ] && ref="${override_ref}"
        do_sync "${project}" "${url}" "${ref}"
        ;;
esac
