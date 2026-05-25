def sanitize_path(path: str) -> str:
    if path is None:
        return ""
    return path.replace('\x00', '').strip()
