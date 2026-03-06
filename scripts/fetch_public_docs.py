import requests
from pathlib import Path
import html2text

DOCS = {
    "blobfuse2-what-is.md":
        "https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-what-is",
    "blobfuse2-installation.md":
        "https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-install?tabs=Ubuntu",
    "blobfuse2-mount.md":
        "https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-mount-container",
    "blobfuse2-compare.md":
        "https://learn.microsoft.com/en-us/azure/storage/blobs/blobfuse2-compare-linux-file-system"
}

out_dir = Path("public")
out_dir.mkdir(exist_ok=True)

converter = html2text.HTML2Text()
converter.ignore_links = False

for name, url in DOCS.items():
    print(f"[public-docs] fetching {url}")
    r = requests.get(url, timeout=30)
    r.raise_for_status()

    md = converter.handle(r.text)
    (out_dir / name).write_text(md, encoding="utf-8")

print("[public-docs] done")