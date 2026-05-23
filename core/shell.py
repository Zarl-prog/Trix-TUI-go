import subprocess
import os
from .utils import sanitize_path

def run_command(command: str, cwd: str = None) -> dict:
    try:
        if cwd:
            cwd = sanitize_path(cwd)
        
        # Use shell=True to support pipes and shell built-ins
        res = subprocess.run(
            command,
            shell=True,
            cwd=cwd,
            capture_output=True,
            text=True,
            timeout=30
        )
        
        return {
            "status": "ok",
            "output": res.stdout + res.stderr,
            "exit_code": res.returncode
        }
    except Exception as e:
        return {"status": "error", "message": str(e)}

def run_command_stream(command: str):
    try:
        process = subprocess.Popen(
            command,
            shell=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1
        )
        for line in process.stdout:
            yield line
        process.wait()
    except Exception as e:
        yield f"Error: {str(e)}"
