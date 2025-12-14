#!/usr/bin/env bash
set -euo pipefail

# generate-docs.sh
# - 目标：将 pkg 下的 Go 导出 API 自动生成 Markdown 文档到 docs/reference/
# - 说明：docs/reference 目录视为“生成产物”，应避免手工改动（会被覆盖）

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="docs/reference"
GOMARKDOC_VERSION="v1.1.0"

mkdir -p "$OUT_DIR"

if ! command -v gomarkdoc >/dev/null 2>&1; then
	echo "gomarkdoc not found, installing ${GOMARKDOC_VERSION} ..." >&2
	# 避免环境中错误的 GOROOT 导致 toolchain 不一致
	env -u GOROOT go install "github.com/princjef/gomarkdoc/cmd/gomarkdoc@${GOMARKDOC_VERSION}"
fi

MODULE_PATH="$(env -u GOROOT go list -m)"

# 清理旧的生成文件，避免包被删除时留下陈旧文档。
rm -f "${OUT_DIR}"/*.md

packages="$(env -u GOROOT go list ./pkg/... | sort)"
if [ -z "${packages}" ]; then
	echo "no packages found under ./pkg/..." >&2
	exit 0
fi

index_file="${OUT_DIR}/index.md"
{
	echo "# API Reference"
	echo
	echo "> 本目录为自动生成产物，请勿手动修改。"
	echo
} >"$index_file"

while IFS= read -r pkg; do
	# pkg: github.com/IMBotPlatform/IMBotCore/pkg/command
	rel="${pkg#${MODULE_PATH}/}"      # pkg/command
	rel_no_pkg="${rel#pkg/}"         # command
	file_name="${rel_no_pkg//\//-}.md" # platform-wecom.md
	out_file="${OUT_DIR}/${file_name}"

	# 生成单个 package 文档
	gomarkdoc "$pkg" -o "$out_file"

	# 追加到索引
	echo "- [${rel_no_pkg}](${file_name})" >>"$index_file"
done <<<"$packages"

echo "docs generated under ${OUT_DIR}" >&2

