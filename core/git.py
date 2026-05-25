import subprocess

def git_status() -> dict:
    try:
        res = subprocess.run(
            ["git", "status", "--short", "--branch"],
            capture_output=True, text=True, timeout=5
        )
        if res.returncode != 0:
            return {"status": "error", "message": "Not a git repository"}
        
        lines = res.stdout.strip().split("\n")
        branch = lines[0].replace("## ", "").split("...")[0] if lines else "unknown"
        
        modified = []
        untracked = []
        staged = []
        
        for line in lines[1:]:
            if not line.strip(): continue
            status = line[:2]
            file = line[3:]
            
            if status[0] in "MADR": staged.append(file)
            if status[1] == "M": modified.append(file)
            if status == "??": untracked.append(file)
            
        return {
            "status": "ok",
            "branch": branch,
            "modified": modified,
            "untracked": untracked,
            "staged": staged
        }
    except Exception as e:
        return {"status": "error", "message": str(e)}

def git_log(limit: int = 50) -> dict:
    try:
        res = subprocess.run(
            ["git", "log", f"--format=%h|%s|%an|%ar|%H", "-n", str(limit)],
            capture_output=True, text=True, timeout=5
        )
        if res.returncode != 0:
            return {"status": "error", "message": "Not a git repository"}
        
        commits = []
        for line in res.stdout.strip().split("\n"):
            if not line: continue
            parts = line.split("|")
            if len(parts) == 5:
                commits.append({
                    "hash": parts[0],
                    "message": parts[1],
                    "author": parts[2],
                    "time": parts[3],
                    "full_hash": parts[4]
                })
        return {"status": "ok", "commits": commits}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def git_show(commit_hash: str) -> dict:
    try:
        res = subprocess.run(
            ["git", "show", "--stat", commit_hash],
            capture_output=True, text=True, timeout=5
        )
        if res.returncode != 0:
            return {"status": "error", "message": "Failed to show commit"}
        return {"status": "ok", "output": res.stdout}
    except Exception as e:
        return {"status": "error", "message": str(e)}

def git_diff(commit_hash: str) -> dict:
    try:
        res = subprocess.run(
            ["git", "diff", commit_hash + "^!"],
            capture_output=True, text=True, timeout=5
        )
        if res.returncode != 0:
            return {"status": "error", "message": "Failed to get diff"}
        return {"status": "ok", "diff": res.stdout}
    except Exception as e:
        return {"status": "error", "message": str(e)}
