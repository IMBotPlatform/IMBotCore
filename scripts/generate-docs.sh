#!/usr/bin/env bash
set -euo pipefail

# generate-docs.sh
# - 目标：将 pkg 下的 Go 导出 API 自动生成 Markdown 文档到 docs/reference/
# - 说明：docs/reference 目录视为“生成产物”，应避免手工改动（会被覆盖）

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="docs/reference"
GOMARKDOC_VERSION="v1.1.0"
CHECK_MODE=false

# 解析参数
while [[ $# -gt 0 ]]; do
	case $1 in
	--check)
		CHECK_MODE=true
		shift
		;;
	*)
		echo "Usage: $0 [--check]" >&2
		echo "  --check  Verify docs are up-to-date without modifying files" >&2
		exit 1
		;;
	esac
done

mkdir -p "$OUT_DIR"

if ! command -v gomarkdoc >/dev/null 2>&1; then
	echo "gomarkdoc not found, installing ${GOMARKDOC_VERSION} ..." >&2
	env -u GOROOT go install "github.com/princjef/gomarkdoc/cmd/gomarkdoc@${GOMARKDOC_VERSION}"
fi

MODULE_PATH="$(env -u GOROOT go list -m)"

# --check 模式：生成到临时目录后对比
if [ "$CHECK_MODE" = true ]; then
	TEMP_DIR="$(mktemp -d)"
	trap "rm -rf '$TEMP_DIR'" EXIT
	GEN_DIR="$TEMP_DIR"
else
	# 正常模式：清理旧的生成文件
	rm -f "${OUT_DIR}"/*.md
	GEN_DIR="$OUT_DIR"
fi

packages="$(env -u GOROOT go list ./pkg/... | sort)"
if [ -z "${packages}" ]; then
	echo "no packages found under ./pkg/..." >&2
	exit 0
fi

index_file="${GEN_DIR}/index.md"
{
	echo "# API Reference"
	echo
	echo "> 本目录为自动生成产物，请勿手动修改。"
	echo
} >"$index_file"

while IFS= read -r pkg; do
	rel="${pkg#${MODULE_PATH}/}"
	rel_no_pkg="${rel#pkg/}"
	file_name="${rel_no_pkg//\//-}.md"
	out_file="${GEN_DIR}/${file_name}"

	gomarkdoc "$pkg" -o "$out_file"
	echo "- [${rel_no_pkg}](${file_name})" >>"$index_file"
done <<<"$packages"

# --check 模式：对比差异
if [ "$CHECK_MODE" = true ]; then
	if diff -rq "$GEN_DIR" "$OUT_DIR" >/dev/null 2>&1; then
		echo "✓ docs/reference is up-to-date" >&2
		exit 0
	else
		echo "✗ docs/reference is out-of-date. Run 'scripts/generate-docs.sh' to update." >&2
		diff -r "$GEN_DIR" "$OUT_DIR" >&2 || true
		exit 1
	fi
fi

echo "docs generated under ${OUT_DIR}" >&2
