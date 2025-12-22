SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PYTHON_BIN="${REPO_ROOT}/.venv/bin/python"

if [ ! -x "$PYTHON_BIN" ]; then
	echo "Error: Python binary $PYTHON_BIN not found. Activate or create the repo virtualenv first."
	exit 1
fi

cd "$SCRIPT_DIR"

"$PYTHON_BIN" -m twine upload dist/*