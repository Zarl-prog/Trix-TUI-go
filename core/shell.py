import subprocess
import os
import sys
import threading
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


class PersistentShell:
    """A persistent shell process that streams output via events."""
    
    def __init__(self, event_callback):
        self.event_callback = event_callback
        self.running = True
        self._lock = threading.Lock()
        
        # Determine shell command per platform
        if sys.platform == "win32":
            shell_cmd = ["powershell.exe", "-NoLogo", "-NoProfile", "-Command", "-"]
        else:
            shell_cmd = ["bash", "--norc", "--noediting"]
        
        self.process = subprocess.Popen(
            shell_cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
        self.thread = threading.Thread(target=self._reader, daemon=True)
        self.thread.start()
    
    def _reader(self):
        """Read output from the shell process in a background thread."""
        try:
            for line in iter(self.process.stdout.readline, ""):
                if not self.running:
                    break
                self.event_callback("terminal_data", {"data": line})
        except Exception:
            pass
        finally:
            self.running = False
    
    def write(self, data: str):
        """Write data to the shell's stdin."""
        with self._lock:
            if self.process.stdin and self.running:
                self.process.stdin.write(data)
                self.process.stdin.flush()
    
    def stop(self):
        """Stop the shell process."""
        self.running = False
        try:
            if self.process.stdin:
                self.process.stdin.close()
            self.process.terminate()
            self.process.wait(timeout=3)
        except Exception:
            try:
                self.process.kill()
            except Exception:
                pass
