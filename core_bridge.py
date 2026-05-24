import sys
import json
import os
from core import files, git, shell

# Ensure the project root is in sys.path so core can be imported
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import subprocess
from pathlib import Path
import threading

# --- Event sending ---
_send_lock = threading.Lock()

def send_event(event_type: str, data: dict):
    """Send an async event to the Go frontend."""
    msg = json.dumps({"event": event_type, "data": data})
    with _send_lock:
        sys.stdout.write(msg + "\n")
        sys.stdout.flush()

# --- Persistent Shell ---
_shell_process = None

def start_terminal(params):
    """Start a persistent shell process."""
    global _shell_process
    if _shell_process:
        _shell_process.stop()
    _shell_process = shell.PersistentShell(send_event)
    return {"status": "ok", "message": "Terminal started"}

def terminal_write(params):
    """Write data to the running shell process."""
    global _shell_process
    if not _shell_process:
        return {"status": "error", "message": "No terminal running"}
    _shell_process.write(params.get("data", ""))
    return {"status": "ok"}

def stop_terminal(params):
    """Stop the persistent shell process."""
    global _shell_process
    if _shell_process:
        _shell_process.stop()
        _shell_process = None
    return {"status": "ok"}


HANDLERS = {
    "list_dir":    lambda r: files.list_dir(r.get("path", ".")),
    "read_file":   lambda r: files.read_file(r.get("path")),
    "write_file":  lambda r: files.write_file(r.get("path"), r.get("content")),
    "create_file": lambda r: files.create_file(r.get("path")),
    "delete_file": lambda r: files.delete_file(r.get("path")),
    "rename_file": lambda r: files.rename_file(r.get("old_path"), r.get("new_path")),
    "get_cwd":     lambda r: files.get_cwd(),
    "change_dir":  lambda r: files.change_dir(r.get("path")),
    "git_status":  lambda r: git.git_status(),
    "git_log":     lambda r: git.git_log(r.get("limit", 50)),
    "git_show":    lambda r: git.git_show(r.get("hash")),
    "git_diff":    lambda r: git.git_diff(r.get("hash")),
    "run_command": lambda r: shell.run_command(r.get("command"), r.get("cwd")),
    "get_git_branch": lambda r: get_git_branch(r.get("path", ".")),
    "search_files":   lambda r: search_files(r.get("root", "."), r.get("query", "")),
    "start_terminal": lambda r: start_terminal(r),
    "terminal_write": lambda r: terminal_write(r),
    "stop_terminal":  lambda r: stop_terminal(r),
    "quit":           lambda r: sys.exit(0),
}


def get_git_branch(path):
    try:
        r = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            cwd=path, capture_output=True, text=True, timeout=2
        )
        branch = r.stdout.strip()
        dirty_r = subprocess.run(
            ["git", "status", "--short"],
            cwd=path, capture_output=True, text=True, timeout=2
        )
        dirty = bool(dirty_r.stdout.strip())
        return {"status": "ok", "branch": branch, "dirty": dirty}
    except Exception as e:
        return {"status": "error", "message": str(e)}


def search_files(root, query):
    results = []
    try:
        for fpath in Path(root).rglob("*"):
            if not fpath.is_file():
                continue
            if any(p in str(fpath) for p in [".git", "__pycache__", "node_modules"]):
                continue
            try:
                lines = fpath.read_text(encoding="utf-8", errors="ignore").splitlines()
            except Exception:
                continue
            for i, line in enumerate(lines):
                if query.lower() in line.lower():
                    results.append({
                        "file": str(fpath),
                        "line": i + 1,
                        "text": line.strip()[:120]
                    })
                    if len(results) >= 200:
                        return {"status": "ok", "results": results}
    except Exception as e:
        return {"status": "error", "message": str(e)}
    return {"status": "ok", "results": results}

def handle_request(request: dict) -> dict:
    # Support both "method" (JSON-RPC style) and "action" (direct style)
    action = request.get("action") or request.get("method")
    
    if action not in HANDLERS:
        return {"status": "error", "message": f"Unknown action: {action}"}
    
    params = request.get("params")
    if params and isinstance(params, dict):
        # Flatten params for the handler
        data = params
    else:
        data = request

    try:
        return HANDLERS[action](data)
    except KeyError as e:
        return {"status": "error", "message": f"Missing field: {e}"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            request = json.loads(line)
            req_id = request.get("id")
            
            result = handle_request(request)
            
            if req_id is not None:
                # Send JSON-RPC response
                print(json.dumps({"id": req_id, "result": result}), flush=True)
            else:
                # Send direct response (backward compatibility)
                print(json.dumps(result), flush=True)
                
        except json.JSONDecodeError as e:
            print(json.dumps({"status": "error", "message": f"Invalid JSON: {e}"}), flush=True)
        except Exception as e:
            print(json.dumps({"status": "error", "message": str(e)}), flush=True)

if __name__ == "__main__":
    main()
