#!/usr/bin/env bash
#
# check-notes.sh — verify every learning note follows the required structure.
#
# The point is to keep notes from silently rotting: a note without a
# "last updated" date or without a "related experiment" link is one that
# will not be maintained.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NOTES_DIR="${REPO_ROOT}/docs/notes"

required_sections=(
    "## 一句话结论"
    "## 问题背景"
    "## 实测数据"
    "## 陷阱与坑"
    "## 引用"
)

failed=0
checked=0

# The notes README is an index, not a note — skip it.
notes=$(find "${NOTES_DIR}" -name '*.md' ! -name 'README.md' | sort)

if [ -z "${notes}" ]; then
    echo "no notes found in ${NOTES_DIR}"
    exit 0
fi

for note in ${notes}; do
    rel="${note#${REPO_ROOT}/}"
    checked=$((checked + 1))
    problems=()

    if ! grep -q '^> \*\*状态\*\*' "${note}"; then
        problems+=("missing '> **状态**' marker")
    fi
    if ! grep -q '^> \*\*最后更新\*\*' "${note}"; then
        problems+=("missing '> **最后更新**' marker")
    fi
    if ! grep -q '^> \*\*相关实验\*\*' "${note}"; then
        problems+=("missing '> **相关实验**' marker")
    fi

    for section in "${required_sections[@]}"; do
        if ! grep -qF "${section}" "${note}"; then
            problems+=("missing section: ${section}")
        fi
    done

    if [ ${#problems[@]} -gt 0 ]; then
        echo ""
        echo "✗ ${rel}"
        for p in "${problems[@]}"; do
            echo "    - ${p}"
        done
        failed=$((failed + 1))
    else
        echo "✓ ${rel}"
    fi
done

echo ""
if [ ${failed} -gt 0 ]; then
    echo "${failed} of ${checked} notes have problems"
    exit 1
fi
echo "all ${checked} notes OK"
