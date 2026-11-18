#!/usr/bin/env bash
# Compute the next release version from conventional commits, per commit.md.
#
# Baseline: the highest existing vX.Y.Z git tag; if the repository has no
# tags yet, the version stamped in mygo.json (commit.md §4.1).
# Bump: the highest impact found in the commit subjects since that baseline
# (commit.md §2): feat=minor, fix/perf/revert=patch, any breaking marker
# (`type!:` subject or a `BREAKING CHANGE:` footer)=major, everything else
# (docs, chore, style, refactor, test, build, ci)=no release.
#
# Outputs GITHUB_ACTIONS-style key=value lines (GITHUB_OUTPUT in CI):
#   version        the next version without the v prefix
#   tag            the tag to create (v + version)
#   bump           major | minor | patch | none
#   should_release true | false
#
# Override the repository location with REPO_DIR (defaults to the directory
# containing this script's parent) so tests can run it against scratch clones.
set -euo pipefail

repo="${REPO_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
out="${GITHUB_OUTPUT:-/dev/stdout}"

cd "$repo"

baseline_tag="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n1 || true)"
if [ -n "$baseline_tag" ]; then
	baseline="${baseline_tag#v}"
	range=("$baseline_tag..HEAD")
else
	baseline="$(sed -nE 's/.*"version"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' mygo.json | head -n1)"
	if [ -z "$baseline" ]; then
		echo "semver.sh: no git tag and no version in mygo.json" >&2
		exit 1
	fi
	range=(HEAD)
fi

major="$(cut -d. -f1 <<<"$baseline")"
minor="$(cut -d. -f2 <<<"$baseline")"
patch="$(cut -d. -f3 <<<"$baseline")"

rank() {
	case "$1" in
	none) echo 0 ;;
	patch) echo 1 ;;
	minor) echo 2 ;;
	major) echo 3 ;;
	esac
}

bump=none

# A BREAKING CHANGE: footer anywhere in the range forces a major bump once.
if git log --pretty=%B "${range[@]}" | grep -q '^BREAKING CHANGE:'; then
	bump=major
fi

while IFS= read -r subject; do
	if [[ ! "$subject" =~ ^(feat|fix|perf|revert|docs|chore|style|refactor|test|build|ci)(\([a-zA-Z0-9._/-]+\))?(!)?: ]]; then
		continue
	fi
	if [ -n "${BASH_REMATCH[3]}" ]; then
		bump=major
		continue
	fi
	case "${BASH_REMATCH[1]}" in
	feat) candidate=minor ;;
	fix | perf | revert) candidate=patch ;;
	*) candidate=none ;;
	esac
	if [ "$(rank "$candidate")" -gt "$(rank "$bump")" ]; then
		bump=$candidate
	fi
done < <(git log --pretty=%s "${range[@]}")

if [ "$bump" = none ]; then
	version="$baseline"
	should_release=false
else
	case "$bump" in
	major)
		major=$((major + 1))
		minor=0
		patch=0
		;;
	minor)
		minor=$((minor + 1))
		patch=0
		;;
	patch) patch=$((patch + 1)) ;;
	esac
	version="${major}.${minor}.${patch}"
	should_release=true
fi

{
	printf 'version=%s\n' "$version"
	printf 'tag=v%s\n' "$version"
	printf 'bump=%s\n' "$bump"
	printf 'should_release=%s\n' "$should_release"
} >"$out"
