#!/bin/bash
#
# Update all git submodules to the latest commit on their default remote branch.
# Automatically discovers submodules via `git submodule status`.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"

detect_default_branch() {
    local submodule_path="$1"
    local remote_head
    remote_head=$(git -C "$submodule_path" symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's|refs/remotes/origin/||')
    if [ -n "$remote_head" ]; then
        echo "$remote_head"
        return
    fi

    for candidate in main master; do
        if git -C "$submodule_path" show-ref --verify --quiet "refs/remotes/origin/$candidate" 2>/dev/null; then
            echo "$candidate"
            return
        fi
    done

    echo "main"
}

echo "==> Initializing and updating all submodules..."
git submodule update --init --recursive

submodule_paths=$(git submodule status --recursive | awk '{print $2}')

if [ -z "$submodule_paths" ]; then
    echo "No submodules found."
    exit 0
fi

if command -v gitc-hanxi >/dev/null 2>&1; then
    has_gitc_hanxi=1
else
    has_gitc_hanxi=0
    echo "[WARN] gitc-hanxi not found in PATH, skipping git user config step."
fi

failed_modules=()

for submodule_path in $submodule_paths; do
    full_path="$REPO_ROOT/$submodule_path"
    if [ ! -d "$full_path" ]; then
        echo "[WARN] Submodule path not found: $submodule_path, skipping."
        continue
    fi

    default_branch=$(detect_default_branch "$full_path")
    echo "==> Updating $submodule_path (branch: $default_branch)..."

    if git -C "$full_path" checkout "$default_branch" && git -C "$full_path" pull; then
        echo "    ✓ $submodule_path updated successfully."
    else
        echo "    ✗ $submodule_path failed to update."
        failed_modules+=("$submodule_path")
        continue
    fi

    if [ "$has_gitc_hanxi" -eq 1 ]; then
        echo "    → Applying gitc-hanxi in $submodule_path..."
        if ! (cd "$full_path" && gitc-hanxi); then
            echo "    ✗ gitc-hanxi failed in $submodule_path."
            failed_modules+=("$submodule_path (gitc-hanxi)")
        fi
    fi
done

echo ""
if [ ${#failed_modules[@]} -eq 0 ]; then
    echo "All submodules updated successfully."
else
    echo "The following submodules failed to update:"
    for module in "${failed_modules[@]}"; do
        echo "  - $module"
    done
    exit 1
fi
