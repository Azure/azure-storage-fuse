#!/usr/bin/env python3
"""
Export GitHub repo issues + PRs + comments into JSONL files suitable for Azure AI Search blob indexer
using parsingMode: jsonLines (each line = one JSON document).

Repo targeted: Azure/azure-storage-fuse (can be overridden by env vars OWNER/REPO).

Outputs (in OUT_DIR, default: out_github_export):
  - threads_issues_prs.jsonl          (one doc per issue or PR thread)
  - issue_pr_comments.jsonl           (one doc per issue/PR timeline comment)
  - pr_reviews.jsonl                  (one doc per PR review)
  - pr_review_comments.jsonl          (one doc per PR diff review comment)

Incremental:
  Stores latest updated_at in out_github_export/state.json and uses it as "since" next run.
"""

import os
import sys
import json
import time
import hashlib
from urllib.parse import urlencode

import requests

OWNER = os.getenv("OWNER", "Azure")
REPO = os.getenv("REPO", "azure-storage-fuse")
BASE = "https://api.github.com"

# Optional: set GITHUB_TOKEN to increase rate limits
GITHUB_TOKEN = os.getenv("GITHUB_TOKEN", "").strip()

OUT_DIR = os.getenv("OUT_DIR", "out_github_export")
STATE_FILE = os.path.join(OUT_DIR, "state.json")


def _headers():
    h = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "blobfuse-support-exporter",
    }
    if GITHUB_TOKEN:
        h["Authorization"] = f"Bearer {GITHUB_TOKEN}"
    return h


def _ensure_out_dir():
    os.makedirs(OUT_DIR, exist_ok=True)


def _load_state():
    if os.path.exists(STATE_FILE):
        with open(STATE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {"since": None}


def _save_state(state):
    with open(STATE_FILE, "w", encoding="utf-8") as f:
        json.dump(state, f, indent=2)


def _request_json(url, params=None):
    if params:
        url = f"{url}?{urlencode(params)}"

    while True:
        r = requests.get(url, headers=_headers(), timeout=60)

        # Basic rate limit handling
        if r.status_code == 403 and "rate limit" in r.text.lower():
            reset = r.headers.get("X-RateLimit-Reset")
            if reset:
                sleep_for = max(1, int(reset) - int(time.time()) + 5)
                print(f"[rate-limit] sleeping {sleep_for}s", file=sys.stderr)
                time.sleep(sleep_for)
                continue

        r.raise_for_status()
        return r.json(), r.headers


def _paginate(url, params=None):
    params = params or {}
    page = 1
    while True:
        p = dict(params)
        p["per_page"] = 100
        p["page"] = page
        data, _hdrs = _request_json(url, p)
        if not data:
            break
        for item in data:
            yield item
        page += 1


def _stable_id(*parts):
    raw = "|".join(str(p) for p in parts)
    return hashlib.sha1(raw.encode("utf-8")).hexdigest()


def export():
    _ensure_out_dir()
    state = _load_state()
    since = state.get("since")  # ISO8601 timestamp or None

    # GitHub "issues" list includes PRs; PRs have "pull_request" key.
    issues_url = f"{BASE}/repos/{OWNER}/{REPO}/issues"
    params = {"state": "all", "sort": "updated", "direction": "desc"}
    if since:
        params["since"] = since

    print("Fetching issues from:", issues_url, params)

    # Open output files once for streaming writes.
    # Use append mode on incremental runs so previously exported records are
    # preserved; use write mode on a full (first) run to start clean.
    # Azure AI Search uses the stable document "id" for upserts, so duplicate
    # records in append mode are safely overwritten by the indexer.
    file_mode = "a" if since else "w"
    threads_path = os.path.join(OUT_DIR, "threads_issues_prs.jsonl")
    comments_path = os.path.join(OUT_DIR, "issue_pr_comments.jsonl")
    reviews_path = os.path.join(OUT_DIR, "pr_reviews.jsonl")
    review_comments_path = os.path.join(OUT_DIR, "pr_review_comments.jsonl")

    newest = None
    thread_count = 0

    with open(threads_path, file_mode, encoding="utf-8") as f_threads, \
         open(comments_path, file_mode, encoding="utf-8") as f_comments, \
         open(reviews_path, file_mode, encoding="utf-8") as f_reviews, \
         open(review_comments_path, file_mode, encoding="utf-8") as f_review_comments:

        for it in _paginate(issues_url, params):
            thread_count += 1
            number = it["number"]
            is_pr = "pull_request" in it
            print("Fetching id: ", number, " PR: ", is_pr)

            labels = []
            if isinstance(it.get("labels"), list):
                labels = [l.get("name") for l in it.get("labels", []) if isinstance(l, dict) and l.get("name")]

            # 1) thread document (issue or PR) — written immediately
            f_threads.write(json.dumps({
                "id": _stable_id("thread", f"{OWNER}/{REPO}#{number}"),
                "content_type": "github_pr" if is_pr else "github_issue",
                "repo": f"{OWNER}/{REPO}",
                "github_number": number,
                "title": it.get("title", ""),
                "content": it.get("body") or "",
                "state": it.get("state"),
                "labels": labels,
                "author": (it.get("user") or {}).get("login"),
                "created_at": it.get("created_at"),
                "updated_at": it.get("updated_at"),
                "closed_at": it.get("closed_at"),
                "source_url": it.get("html_url"),
            }, ensure_ascii=False) + "\n")

            # Track newest timestamp for incremental state update
            ts = it.get("updated_at")
            if ts and (newest is None or ts > newest):
                newest = ts

            # 2) issue comments (for issues AND PR timeline comments)
            ic_url = f"{BASE}/repos/{OWNER}/{REPO}/issues/{number}/comments"
            for c in _paginate(ic_url):
                f_comments.write(json.dumps({
                    "id": _stable_id("issue_comment", number, c["id"]),
                    "content_type": "github_pr_issue_comment" if is_pr else "github_issue_comment",
                    "repo": f"{OWNER}/{REPO}",
                    "github_number": number,
                    "comment_id": c["id"],
                    "author": (c.get("user") or {}).get("login"),
                    "created_at": c.get("created_at"),
                    "updated_at": c.get("updated_at"),
                    "source_url": c.get("html_url"),
                    "content": c.get("body") or "",
                }, ensure_ascii=False) + "\n")

            if is_pr:
                # 3) PR reviews (top-level review bodies)
                reviews_url = f"{BASE}/repos/{OWNER}/{REPO}/pulls/{number}/reviews"
                for rv in _paginate(reviews_url):
                    f_reviews.write(json.dumps({
                        "id": _stable_id("pr_review", number, rv["id"]),
                        "content_type": "github_pr_review",
                        "repo": f"{OWNER}/{REPO}",
                        "github_number": number,
                        "review_id": rv["id"],
                        "state": rv.get("state"),
                        "author": (rv.get("user") or {}).get("login"),
                        "submitted_at": rv.get("submitted_at"),
                        "source_url": rv.get("html_url"),
                        "content": rv.get("body") or "",
                    }, ensure_ascii=False) + "\n")

                # 4) PR review comments (diff comments)
                prc_url = f"{BASE}/repos/{OWNER}/{REPO}/pulls/{number}/comments"
                for rc in _paginate(prc_url):
                    f_review_comments.write(json.dumps({
                        "id": _stable_id("pr_review_comment", number, rc["id"]),
                        "content_type": "github_pr_review_comment",
                        "repo": f"{OWNER}/{REPO}",
                        "github_number": number,
                        "comment_id": rc["id"],
                        "pull_request_review_id": rc.get("pull_request_review_id"),
                        "path": rc.get("path"),
                        "line": rc.get("line"),
                        "side": rc.get("side"),
                        "author": (rc.get("user") or {}).get("login"),
                        "created_at": rc.get("created_at"),
                        "updated_at": rc.get("updated_at"),
                        "source_url": rc.get("html_url"),
                        "content": rc.get("body") or "",
                    }, ensure_ascii=False) + "\n")

            # Flush after each thread so progress is preserved on interruption
            f_threads.flush()
            f_comments.flush()
            f_reviews.flush()
            f_review_comments.flush()

    print("Threads fetched:", thread_count)

    # Update incremental marker: latest updated_at seen
    if newest:
        state["since"] = newest
        _save_state(state)

    print(f"[ok] wrote exports to: {OUT_DIR}")
    print(f"[ok] next incremental since: {state.get('since')}")


if __name__ == "__main__":
    export()