#!/usr/bin/env bash
# Validates that module PRs include an appropriate CHANGELOG version bump.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib/modules.sh
source "${SCRIPT_DIR}/lib/modules.sh"

BASE_REF=${1:-origin/main}
COMPARE_RANGE="${BASE_REF}...HEAD"

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  echo "Base ref ${BASE_REF} not found; fetching origin/main"
  git fetch origin main --tags
  BASE_REF=origin/main
  COMPARE_RANGE="${BASE_REF}...HEAD"
fi

changed_files=$(git diff --name-only $COMPARE_RANGE || true)
if [[ -z "$changed_files" ]]; then
  echo "No changed files in ${COMPARE_RANGE}; skipping changelog check."
  exit 0
fi

errors=0

check_module() {
  local module=$1
  local code_changed=0
  local changelog_changed=0
  local changelog
  changelog=$(module_changelog "$module")

  if module_code_changed "$module" "$COMPARE_RANGE"; then
    code_changed=1
  fi

  if module_changelog_changed "$module" "$COMPARE_RANGE"; then
    changelog_changed=1
  fi

  if [[ "$code_changed" -eq 0 && "$changelog_changed" -eq 0 ]]; then
    return 0
  fi

  if [[ ! -f "$changelog" ]]; then
    echo "ERROR: ${module} is missing CHANGELOG.md"
    errors=$((errors + 1))
    return 0
  fi

  if [[ "$code_changed" -eq 1 && "$changelog_changed" -eq 0 ]]; then
    echo "ERROR: ${module} code changed but ${module}/CHANGELOG.md was not updated."
    echo "       Add a new ## [X.Y.Z] - YYYY-MM-DD section describing the release."
    errors=$((errors + 1))
    return 0
  fi

  local latest_tag pending_version
  latest_tag=$(latest_tag_version "$module" || true)
  pending_version=$(pending_release_version "$module" || true)

  if [[ "$code_changed" -eq 1 ]]; then
    if [[ -z "$pending_version" ]]; then
      echo "ERROR: ${module} code changed but no untagged version was found in CHANGELOG.md."
      echo "       Add a new version section greater than the latest tag (${latest_tag:-none})."
      errors=$((errors + 1))
      return 0
    fi

    if [[ -n "$latest_tag" ]] && ! version_gt "$pending_version" "$latest_tag"; then
      echo "ERROR: ${module} pending version v${pending_version} must be greater than latest tag v${latest_tag}."
      errors=$((errors + 1))
      return 0
    fi

    echo "OK: ${module} will release v${pending_version} (latest tag: v${latest_tag:-none})."
  elif [[ "$changelog_changed" -eq 1 ]]; then
    if [[ -n "$pending_version" ]]; then
      if [[ -n "$latest_tag" ]] && ! version_gt "$pending_version" "$latest_tag"; then
        echo "ERROR: ${module} pending version v${pending_version} must be greater than latest tag v${latest_tag}."
        errors=$((errors + 1))
        return 0
      fi
      echo "OK: ${module} changelog-only update prepares v${pending_version}."
    else
      echo "OK: ${module} changelog updated without a pending release."
    fi
  fi
}

while IFS= read -r module; do
  [[ -z "$module" ]] && continue
  if echo "$changed_files" | grep -q "^${module}/"; then
    check_module "$module"
  fi
done < <(discover_modules)

if [[ "$errors" -gt 0 ]]; then
  echo ""
  echo "${errors} changelog validation error(s)."
  exit 1
fi

echo "Changelog validation passed."
