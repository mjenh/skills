#!/usr/bin/env bash
# Creates module tags, GitHub releases, and updates README.md on main.

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=lib/modules.sh
source "${SCRIPT_DIR}/lib/modules.sh"

BEFORE_SHA=${1:-}
AFTER_SHA=${2:-HEAD}
GITHUB_REPOSITORY=${GITHUB_REPOSITORY:-}
SKIP_CI=${SKIP_CI:-1}

if [[ -z "$BEFORE_SHA" ]]; then
  echo "Usage: release-module.sh <before-sha> [after-sha]"
  exit 1
fi

commit_message=$(git log -1 --pretty=%B "$AFTER_SHA" || true)
if [[ "$commit_message" == *"[skip ci]"* ]]; then
  echo "Commit message contains [skip ci]; skipping release workflow."
  exit 0
fi

if [[ "$BEFORE_SHA" == "0000000000000000000000000000000000000000" ]]; then
  echo "Initial push detected; skipping release workflow."
  exit 0
fi

changed_files=$(git diff --name-only "$BEFORE_SHA" "$AFTER_SHA" || true)
if [[ -z "$changed_files" ]]; then
  echo "No changed files between ${BEFORE_SHA} and ${AFTER_SHA}."
  exit 0
fi

released_any=0
readme_updated=0
root_readme=$(repo_root)/README.md

release_module() {
  local module=$1
  local changelog
  changelog=$(module_changelog "$module")

  if ! module_changelog_changed "$module" "${BEFORE_SHA}..${AFTER_SHA}"; then
    echo "Skipping ${module}: CHANGELOG.md not changed in this push."
    return 0
  fi

  local version
  version=$(pending_release_version "$module" || true)
  if [[ -z "$version" ]]; then
    echo "Skipping ${module}: no untagged version in CHANGELOG.md."
    return 0
  fi

  local tag notes_file release_date anchor
  tag=$(module_tag "$module" "$version")
  release_date=$(changelog_release_date "$changelog" "$version")
  anchor=$(changelog_anchor "$version" "$release_date")

  if tag_exists "$tag"; then
    echo "Skipping ${module}: tag ${tag} already exists."
    return 0
  fi

  notes_file=$(mktemp)
  {
    echo "## ${module} v${version}"
    echo ""
    extract_release_notes "$changelog" "$version"
  } > "$notes_file"

  echo "Creating tag ${tag} at ${AFTER_SHA}"
  git tag "$tag" "$AFTER_SHA"
  git push origin "$tag"

  if command -v gh >/dev/null 2>&1 && [[ -n "$GITHUB_REPOSITORY" ]]; then
    if gh release view "$tag" >/dev/null 2>&1; then
      echo "GitHub release ${tag} already exists."
    else
      echo "Creating GitHub release ${tag}"
      gh release create "$tag" \
        --title "${module} v${version}" \
        --notes-file "$notes_file" \
        --target "$AFTER_SHA"
    fi
  else
    echo "gh CLI unavailable or GITHUB_REPOSITORY unset; skipped GitHub release for ${tag}."
  fi

  rm -f "$notes_file"

  if [[ -f "$root_readme" ]]; then
    python3 "${SCRIPT_DIR}/update-readme.py" \
      "$root_readme" \
      "$module" \
      "$version" \
      "$anchor"
    readme_updated=1
  fi

  released_any=1
  echo "Released ${module} v${version} (${tag})."
}

while IFS= read -r module; do
  [[ -z "$module" ]] && continue
  if echo "$changed_files" | grep -q "^${module}/"; then
    release_module "$module"
  fi
done < <(discover_modules)

if [[ "$readme_updated" -eq 1 ]]; then
  if git diff --quiet -- "$root_readme"; then
    echo "README.md already up to date."
    exit 0
  fi

  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

  git add "$root_readme"
  skip_suffix=""
  if [[ "$SKIP_CI" == "1" ]]; then
    skip_suffix=" [skip ci]"
  fi
  git commit -m "chore: update README module versions${skip_suffix}"
  git push origin HEAD:main
  echo "Updated README.md on main."
fi

if [[ "$released_any" -eq 0 ]]; then
  echo "No module releases were created."
fi
