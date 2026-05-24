def sanitize_path(path: str) -> str:
    if path is None:
        return ""
    return path.replace('\x00', '').strip()

def is_git_repo(path: str) -> bool:
    import subprocess
    from pathlib import Path
    try:
        res = subprocess.run(
            ["git", "rev-parse", "--is-inside-work-tree"],
            cwd=path, capture_output=True, text=True, timeout=2
        )
        return res.returncode == 0
    except Exception:
        return False
