import datetime
import argparse
from collections import Counter
import os
import statistics
import subprocess
import sys

from github import Auth, Github
from github.GithubException import BadCredentialsException, GithubException


DEFAULT_DAYS_LOOKBACK = 90


def get_env_var(*names: str) -> str:
    for name in names:
        value = os.getenv(name, "").strip()
        if value:
            return value
    return ""


def get_gh_cli_token() -> str:
    try:
        result = subprocess.run(
            ["gh", "auth", "token"],
            check=True,
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        return ""
    except subprocess.CalledProcessError:
        result = None

    if result is not None:
        token = result.stdout.strip()
        if token:
            return token

    try:
        result = subprocess.run(
            ["gh", "auth", "status", "--show-token"],
            check=True,
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        return ""
    except subprocess.CalledProcessError:
        return ""

    token_output = "\n".join([result.stdout, result.stderr])
    for line in token_output.splitlines():
        marker = "Token:"
        if marker in line:
            token = line.split(marker, 1)[1].strip()
            if token:
                return token
    return ""


def get_gh_cli_repo() -> str:
    try:
        result = subprocess.run(
            ["gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner"],
            check=True,
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        return ""
    except subprocess.CalledProcessError:
        return ""
    return result.stdout.strip()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Generate GitHub issue metrics report.")
    parser.add_argument(
        "--days",
        type=int,
        default=int(os.getenv("DAYS_LOOKBACK", str(DEFAULT_DAYS_LOOKBACK))),
        help=f"Number of days to look back (default: {DEFAULT_DAYS_LOOKBACK}).",
    )
    parser.add_argument(
        "--only-copilot-tagged",
        action="store_true",
        help="Show only copilot-tagged entries in issue and PR details tables.",
    )
    parser.add_argument(
        "--summary",
        action="store_true",
        help="Show only summary tables (hide issue and PR details).",
    )
    return parser.parse_args()


def print_table(title: str, headers: list[str], rows: list[list[str]]) -> None:
    table_rows = [headers] + rows
    widths = [max(len(str(row[index])) for row in table_rows) for index in range(len(headers))]

    def format_row(row: list[str]) -> str:
        return "| " + " | ".join(str(value).ljust(widths[index]) for index, value in enumerate(row)) + " |"

    separator = "+-" + "-+-".join("-" * width for width in widths) + "-+"

    print(title)
    print(separator)
    print(format_row(headers))
    print(separator)
    for row in rows:
        print(format_row(row))
    print(separator)


def format_duration(hours: float) -> str:
    if hours >= 24:
        return f"{hours / 24:.2f} days"
    return f"{hours:.2f} hrs"


def format_days(hours: float) -> str:
    return f"{hours / 24:.2f}"


def truncate_text(text: str, max_length: int = 60) -> str:
    if len(text) <= max_length:
        return text
    return text[: max_length - 3] + "..."


def is_copilot_tagged(labels) -> bool:
    for label in labels:
        if hasattr(label, "name") and "copilot" in label.name.lower():
            return True
    return False


def get_first_comment_time_display_for_pr(pr) -> str:
    first_comment_time = None

    try:
        issue_comment = next(iter(pr.get_issue_comments()), None)
        if issue_comment is not None and issue_comment.created_at is not None:
            first_comment_time = issue_comment.created_at
    except GithubException:
        return "Unavailable"

    try:
        review_comment = next(iter(pr.get_comments()), None)
        if review_comment is not None and review_comment.created_at is not None:
            if first_comment_time is None or review_comment.created_at < first_comment_time:
                first_comment_time = review_comment.created_at
    except GithubException:
        return "Unavailable"

    if first_comment_time is None:
        return "No comments"

    first_comment_hours = (first_comment_time - pr.created_at).total_seconds() / 3600
    return format_duration(first_comment_hours)


def main() -> int:
    args = parse_args()

    access_token = get_env_var("GITHUB_TOKEN", "GH_TOKEN") or get_gh_cli_token()
    repo_name = get_env_var("REPO_NAME", "GITHUB_REPOSITORY") or get_gh_cli_repo()
    days_lookback = args.days

    if days_lookback <= 0:
        print("Error: --days must be a positive integer.", file=sys.stderr)
        return 1

    if not access_token:
        print(
            "Error: Missing token. Set GITHUB_TOKEN/GH_TOKEN or run 'gh auth login'.",
            file=sys.stderr,
        )
        return 1

    if not repo_name or "/" not in repo_name:
        print("Error: Missing repo. Set REPO_NAME or GITHUB_REPOSITORY as 'owner/repo'.", file=sys.stderr)
        return 1

    try:
        g = Github(auth=Auth.Token(access_token))
        repo = g.get_repo(repo_name)
    except BadCredentialsException:
        print("Error: GitHub authentication failed (bad token).", file=sys.stderr)
        return 1
    except GithubException as ex:
        print(f"Error: Unable to access repository '{repo_name}': {ex}", file=sys.stderr)
        return 1

    since = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=days_lookback)

    issues = repo.get_issues(state="all", since=since)

    new_count = 0
    open_count = 0
    resolved_count = 0
    issue_copilot_yes_count = 0
    resolution_times = []
    issue_rows = []

    for issue in issues:
        if issue.pull_request:
            continue

        new_count += 1
        close_time_display = "Open"
        first_comment_display = "No comments"

        if issue.comments and issue.comments > 0:
            try:
                first_comment = next(iter(issue.get_comments()), None)
                if first_comment is not None and first_comment.created_at is not None:
                    first_comment_hours = (first_comment.created_at - issue.created_at).total_seconds() / 3600
                    first_comment_display = format_duration(first_comment_hours)
            except GithubException:
                first_comment_display = "Unavailable"

        if issue.state == "open":
            open_count += 1
        elif issue.state == "closed" and issue.closed_at is not None:
            resolved_count += 1
            duration = issue.closed_at - issue.created_at
            resolution_hours = duration.total_seconds() / 3600
            resolution_times.append(resolution_hours)
            close_time_display = format_days(resolution_hours)

        issue_copilot_tag = "Yes" if is_copilot_tagged(issue.labels) else "No"
        if issue_copilot_tag == "Yes":
            issue_copilot_yes_count += 1

        if args.only_copilot_tagged and issue_copilot_tag != "Yes":
            continue

        issue_rows.append(
            [
                f"#{issue.number}",
                truncate_text(issue.title),
                close_time_display,
                first_comment_display,
                issue_copilot_tag,
            ]
        )

    if resolution_times:
        mean_time = statistics.mean(resolution_times)
        min_time = min(resolution_times)
        max_time = max(resolution_times)
    else:
        mean_time = min_time = max_time = 0

    print_table(
        f"Issue Report (Last {days_lookback} Days)",
        ["Repository", "Total New", "Still Open", "Resolved", "Copilot"],
        [[repo_name, str(new_count), str(open_count), str(resolved_count), str(issue_copilot_yes_count)]],
    )
    print()
    print_table(
        "Resolution Metrics",
        ["Mean", "Min", "Max"],
        [[format_duration(mean_time), format_duration(min_time), format_duration(max_time)]],
    )
    if not args.summary:
        print()
        print_table(
            "Issue Details",
            ["Issue", "Title", "Days to Close", "Time to First Comment", "Copilot"],
            issue_rows,
        )

    pull_requests = repo.get_pulls(state="all", sort="created", direction="desc")

    pr_created_count = 0
    pr_merged_count = 0
    pr_abandoned_count = 0
    pr_copilot_yes_count = 0
    pr_owner_counts = Counter()
    pr_rows = []

    for pr in pull_requests:
        if pr.created_at < since:
            break

        if pr.base is None or pr.base.ref != "main":
            continue

        pr_created_count += 1
        time_to_close_display = "Open"

        if pr.state == "closed":
            if pr.merged_at is not None:
                pr_merged_count += 1
            else:
                pr_abandoned_count += 1

            if pr.closed_at is not None:
                close_hours = (pr.closed_at - pr.created_at).total_seconds() / 3600
                time_to_close_display = format_duration(close_hours)

        pr_author = pr.user.login if pr.user else "Unknown"
        pr_owner_counts[pr_author] += 1
        pr_author_normalized = pr_author.lower()
        pr_copilot_tag = "Yes" if pr_author_normalized in {"dependabot[bot]", "copilot"} else "No"
        if pr_copilot_tag != "Yes":
            try:
                pr_issue = repo.get_issue(pr.number)
                pr_copilot_tag = "Yes" if is_copilot_tagged(pr_issue.labels) else "No"
            except GithubException:
                pr_copilot_tag = "No"
        if pr_copilot_tag == "Yes":
            pr_copilot_yes_count += 1

        if args.only_copilot_tagged and pr_copilot_tag != "Yes":
            continue

        pr_rows.append(
            [
                f"#{pr.number}",
                truncate_text(pr.title),
                pr_author,
                get_first_comment_time_display_for_pr(pr),
                time_to_close_display,
                pr_copilot_tag,
            ]
        )

    print()
    print_table(
        f"PR Report (Last {days_lookback} Days)",
        ["Repository", "PRs Created", "PRs Merged", "PRs Abandoned", "Copilot"],
        [[repo_name, str(pr_created_count), str(pr_merged_count), str(pr_abandoned_count), str(pr_copilot_yes_count)]],
    )

    pr_owner_rows = [[owner, str(count)] for owner, count in pr_owner_counts.most_common()]
    print()
    print_table(
        "PR Owner Summary",
        ["Author", "PRs"],
        pr_owner_rows,
    )

    if not args.summary:
        print()
        print_table(
            "PR Details",
            ["PR", "Title", "Author", "Time to First Comment", "Time to Merge/Close", "Copilot"],
            pr_rows,
        )

    return 0


if __name__ == "__main__":
    raise SystemExit(main())