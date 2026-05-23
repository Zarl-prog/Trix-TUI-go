import os
from pathlib import Path
from .utils import sanitize_path

def list_dir(path: str) -> dict:
    try:
        path = sanitize_path(path)
        p = Path(path)
        if not p.exists():
            return {"status": "error", "message": "Path does not exist"}
        
        entries = []
        for entry in p.iterdir():
            entries.append({
                "name": entry.name,
                "is_dir": entry.is_dir(),
                "path": str(entry)
            })
        
        # Sort: directories first, then files
        entries.sort(key=lambda x: (not x["is_dir"], x["name"].lower()))
        
        return {"status": "ok", "entries": entries}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def read_file(path: str) -> dict:
    try:
        path = sanitize_path(path)
        content = Path(path).read_text(encoding="utf-8", errors="replace")
        return {"status": "ok", "content": content}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def write_file(path: str, content: str) -> dict:
    try:
        path = sanitize_path(path)
        Path(path).write_text(content, encoding="utf-8")
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def create_file(path: str) -> dict:
    try:
        path = sanitize_path(path)
        p = Path(path)
        p.parent.mkdir(parents=True, exist_ok=True)
        p.touch()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def delete_file(path: str) -> dict:
    try:
        path = sanitize_path(path)
        p = Path(path)
        if p.is_dir():
            p.rmdir()
        else:
            p.unlink()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def rename_file(old_path: str, new_path: str) -> dict:
    try:
        old_path = sanitize_path(old_path)
        new_path = sanitize_path(new_path)
        Path(old_path).rename(new_path)
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def get_cwd() -> dict:
    try:
        return {"status": "ok", "path": os.getcwd()}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def change_dir(path: str) -> dict:
    try:
        path = sanitize_path(path)
        os.chdir(path)
        return {"status": "ok", "path": os.getcwd()}
    except Exception as e:
        return {"status": "error", "message": str(e)}
