VERSION="${1:-v0.0.0}"

LATEST_TAG_REV=$(git rev-list --tags --max-count=1 2>/dev/null || true)
LATEST_TAG=$(git describe --tags "$LATEST_TAG_REV" 2>/dev/null || echo "v0.0.0")

if [[  "${LATEST_TAG}" == "${VERSION}"* ]]; then
    if [[ -n "$CHECK" ]]; then
        echo "Update .version and add changelog"
        exit 1
    fi
    git rev-list HEAD --max-count=1 --abbrev-commit | sed "s/^/${VERSION}-/"
else
    echo "$VERSION"
fi