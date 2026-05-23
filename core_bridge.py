import sys
import json
import os
from core import files, git, shell

# Ensure the project root is in sys.path so core can be imported
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

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
}

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
