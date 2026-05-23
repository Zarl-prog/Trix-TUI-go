import sys
import json
import subprocess
import os
import threading
from pathlib import Path
from datetime import datetime, timezone
import winpty

# ==============================================================================
# Git Logic
# ==============================================================================

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

def get_git_history(repo_path, count=50):
    try:
        result = subprocess.run(
            ["git", "log", "--format=%H|%h|%s|%an|%ae|%ai", "-n", str(count)],
            cwd=repo_path, capture_output=True, text=True, timeout=5
        )
        commits = []
        for line in result.stdout.strip().split("\n"):
            if not line.strip(): continue
            parts = line.split("|")
            if len(parts) >= 6:
                full_hash, hash7, message, author, email, date = parts[:6]
                commits.append({
                    "full_hash": full_hash,
                    "hash7": hash7,
                    "message": message,
                    "author": author,
                    "email": email,
                    "date": date
                })
        return {"status": "ok", "commits": commits}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def get_git_show(repo_path, commit_hash):
    try:
        result = subprocess.run(
            ["git", "show", "--numstat", "--format=", commit_hash],
            cwd=repo_path, capture_output=True, text=True, timeout=5
        )
        files = []
        for line in result.stdout.strip().split("\n"):
            parts = line.split("\t")
            if len(parts) == 3:
                try:
                    adds = int(parts[0]) if parts[0] != "-" else 0
                    dels = int(parts[1]) if parts[1] != "-" else 0
                except ValueError:
                    adds, dels = 0, 0
                files.append({"name": parts[2].strip(), "adds": adds, "dels": dels})
        return {"status": "ok", "files": files}
    except Exception as e:
        return {"status": "error", "message": str(e)}

# ==============================================================================
# File System Logic
# ==============================================================================

def get_file_tree(root_path):
    try:
        root = Path(root_path)
        def build_tree(path):
            name = path.name if path.name else str(path)
            node = {"name": name, "path": str(path), "is_dir": path.is_dir()}
            if path.is_dir():
                try:
                    node["children"] = [build_tree(p) for p in path.iterdir() if not p.name.startswith(".")]
                except PermissionError:
                    node["children"] = []
            return node
        return {"status": "ok", "tree": build_tree(root)}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def read_file(path):
    try:
        content = Path(path).read_text(encoding="utf-8")
        return {"status": "ok", "content": content}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def save_file(path, content):
    try:
        Path(path).write_text(content, encoding="utf-8")
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def create_file(path):
    try:
        Path(path).touch()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def delete_file(path):
    try:
        p = Path(path)
        if p.is_dir():
            p.rmdir()
        else:
            p.unlink()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def rename_file(old, new):
    try:
        Path(old).rename(new)
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def search_files(root_path, query):
    try:
        results = []
        root = Path(root_path)
        for p in root.rglob("*"):
            if not p.is_file(): continue
            if any(junk in str(p) for junk in [".git", "__pycache__", "node_modules", ".venv", "venv"]):
                continue
            try:
                # Basic binary check
                with open(p, "rb") as f:
                    if b"\x00" in f.read(1024): continue
                
                content = p.read_text(encoding="utf-8", errors="ignore")
                for i, line in enumerate(content.splitlines()):
                    if query.lower() in line.lower():
                        results.append({
                            "file": str(p),
                            "line": i + 1,
                            "text": line.strip()[:120]
                        })
                        if len(results) >= 200:
                            return {"status": "ok", "results": results}
            except Exception:
                continue
        return {"status": "ok", "results": results}
    except Exception as e:
        return {"status": "error", "message": str(e)}

# ==============================================================================
# Terminal / PTY Logic
# ==============================================================================

_pty = None

def terminal_spawn(rows=24, cols=80):
    global _pty
    try:
        if _pty: _pty.terminate()
        _pty = winpty.PtyProcess.spawn("powershell.exe", dimensions=(rows, cols))
        
        # Start a background thread to read from PTY and push to stdout
        def read_loop():
            while _pty and _pty.isalive():
                try:
                    data = _pty.read(4096)
                    if data:
                        send_event("terminal_data", {"data": data})
                except Exception:
                    break
        
        threading.Thread(target=read_loop, daemon=True).start()
        return {"status": "ok"}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def terminal_write(data):
    global _pty
    if _pty and _pty.isalive():
        try:
            _pty.write(data)
            return {"status": "ok"}
        except Exception as e:
            return {"status": "error", "message": str(e)}
    return {"status": "error", "message": "PTY not running"}

# ==============================================================================
# Bridge Infrastructure
# ==============================================================================

def send_response(id, result):
    print(json.dumps({"id": id, "result": result}), flush=True)

def send_event(event, data):
    print(json.dumps({"event": event, "data": data}), flush=True)

def sanitize_path(path: str) -> str:
    if path is None:
        return ""
    return path.replace('\x00', '').strip()

def main():
    for line in sys.stdin:
        try:
            req = json.loads(line)
            method = req.get("method")
            params = req.get("params", {})
            req_id = req.get("id")

            # Sanitize paths in params
            for k in list(params.keys()):
                if isinstance(params[k], str) and (k.endswith("path") or k == "root"):
                    params[k] = sanitize_path(params[k])

            if method == "read_file":
                res = read_file(params.get("path"))
            elif method == "write_file":
                res = save_file(params.get("path"), params.get("content"))
            elif method == "list_dir":
                path = params.get("path", ".")
                try:
                    entries = []
                    for p in Path(path).iterdir():
                        entries.append({"name": p.name, "is_dir": p.is_dir(), "path": str(p)})
                    res = {"status": "ok", "entries": entries}
                except Exception as e:
                    res = {"status": "error", "message": str(e)}
            elif method == "create_file":
                res = create_file(params.get("path"))
            elif method == "delete_file":
                res = delete_file(params.get("path"))
            elif method == "rename_file":
                res = rename_file(params.get("old_path"), params.get("new_path"))
            elif method == "git_status":
                res = get_git_branch(params.get("path", "."))
            elif method == "git_log":
                res = get_git_history(params.get("path", "."), params.get("count", 50))
            elif method == "run_command":
                try:
                    cmd = params.get("command")
                    r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
                    res = {"status": "ok", "output": r.stdout + r.stderr}
                except Exception as e:
                    res = {"status": "error", "message": str(e)}
            elif method == "terminal_spawn":
                res = terminal_spawn(params.get("rows", 24), params.get("cols", 80))
            elif method == "terminal_write":
                res = terminal_write(params.get("data"))
            elif method == "quit":
                break
            else:
                # Support "action" as an alias for "method" if sent directly
                action = req.get("action")
                if action == "read_file":
                    res = read_file(params.get("path"))
                elif action == "write_file":
                    res = save_file(params.get("path"), params.get("content"))
                elif action == "list_dir":
                    path = params.get("path", ".")
                    try:
                        entries = []
                        for p in Path(path).iterdir():
                            entries.append({"name": p.name, "is_dir": p.is_dir(), "path": str(p)})
                        res = {"status": "ok", "entries": entries}
                    except Exception as e:
                        res = {"status": "error", "message": str(e)}
                elif action == "git_status":
                    res = get_git_branch(params.get("path", "."))
                elif action == "run_command":
                    try:
                        cmd = params.get("command")
                        r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
                        res = {"status": "ok", "output": r.stdout + r.stderr}
                    except Exception as e:
                        res = {"status": "error", "message": str(e)}
                else:
                    res = {"status": "error", "message": f"Unknown method: {method or action}"}
            
            if req_id is not None:
                send_response(req_id, res)
        except Exception as e:
            print(json.dumps({"error": str(e)}), flush=True)

if __name__ == "__main__":
    main()
