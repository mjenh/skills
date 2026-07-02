#!/usr/bin/env bash
# Shared helpers for Go module discovery and CHANGELOG parsing.

set -euo pipefail

repo_root() {
  git rev-parse --show-toplevel
}

discover_modules() {
  find "$(repo_root)" -maxdepth 2 -name go.mod -not -path '*/.*/*' \
    | sed -E 's|.*/([^/]+)/go\.mod|\1|' \
    | sort -u
}

module_changelog() {
  echo "$(repo_root)/$1/CHANGELOG.md"
}

module_tag() {
  local module=$1
  local version=$2
  echo "${module}/v${version}"
}

latest_tag_version() {
  local module=$1
  git tag -l "${module}/v*" 2>/dev/null \
    | sed "s|^${module}/v||" \
    | sort -V \
    | tail -1
}

changelog_versions() {
  local changelog=$1
  grep -E '^## \[[0-9]+\.[0-9]+\.[0-9]+\]' "$changelog" \
    | sed -E 's/^## \[([0-9]+\.[0-9]+\.[0-9]+)\].*/\1/' \
    | sort -V
}

highest_changelog_version() {
  changelog_versions "$1" | tail -1
}

version_gt() {
  local left=$1
  local right=$2
  [[ "$(printf '%s\n' "$left" "$right" | sort -V | tail -1)" == "$left" && "$left" != "$right" ]]
}

tag_exists() {
  git rev-parse --verify "refs/tags/$1" >/dev/null 2>&1
}

pending_release_version() {
  local module=$1
  local changelog
  changelog=$(module_changelog "$module")

  if [[ ! -f "$changelog" ]]; then
    return 1
  fi

  local version
  while IFS= read -r version; do
    [[ -z "$version" ]] && continue
    if ! tag_exists "$(module_tag "$module" "$version")"; then
      echo "$version"
      return 0
    fi
  done < <(changelog_versions "$changelog" | sort -V)

  return 1
}

changelog_release_date() {
  local changelog=$1
  local version=$2
  grep -E "^## \\[${version}\\]" "$changelog" \
    | sed -E 's/.* - ([0-9]{4}-[0-9]{2}-[0-9]{2}).*/\1/' \
    | head -1
}

changelog_anchor() {
  local version=$1
  local date=$2
  local compact=${version//./}
  echo "${compact}---${date}"
}

extract_release_notes() {
  local changelog=$1
  local version=$2
  awk -v ver="$version" '
    BEGIN { capture = 0 }
    /^## \[/ {
      if (capture) exit
      if ($0 ~ "^## \\[" ver "\\]") {
        capture = 1
        next
      }
    }
    capture { print }
  ' "$changelog"
}

module_code_changed() {
  local module=$1
  local range=$2
  git diff --name-only $range -- "$module" \
    | grep -vE "^${module}/CHANGELOG\.md$" \
    | grep -q .
}

module_changelog_changed() {
  local module=$1
  local range=$2
  git diff --name-only $range -- "${module}/CHANGELOG.md" | grep -q .
}
