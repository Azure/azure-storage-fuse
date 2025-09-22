#!/usr/bin/env python3

"""
This script fetches issues and pull requests from a specified GitHub repository,
separates them, and writes each item directly to its respective JSON Lines (.jsonl) file.
This approach is more memory-efficient than storing all data in memory first.

It uses environment variables for both authentication and repository details.

Prerequisites:
- PyGithub library installed (`pip install PyGithub`)
- The following environment variables must be set:
  - GITHUB_TOKEN: A GitHub Personal Access Token with 'repo' scope.
  - GITHUB_REPO_OWNER: The owner of the GitHub repository.
  - GITHUB_REPO_NAME: The name of the GitHub repository.
"""

import os
import json
from github import Github, GithubException, Auth

def get_item_comments(item):
    """
    Fetches all comments for a given GitHub issue or pull request
    and formats them as a list of dictionaries.
    """
    comments_list = []
    try:
        for comment in item.get_comments():
            comments_list.append({
                'author': comment.user.login,
                'comment': comment.body.strip(),
            })
    except Exception as e:
        print(f"Warning: Could not fetch comments for item #{item.number}. Error: {e}")
        
    return comments_list

def main():
    """
    Main function to fetch data and write to JSONL files.
    """
    # 1. Get configuration from environment variables
    github_token = os.environ.get('GITHUB_TOKEN')
    repo_owner = os.environ.get('GITHUB_REPO_OWNER')
    repo_name = os.environ.get('GITHUB_REPO_NAME')

    if not all([github_token, repo_owner, repo_name]):
        print("Error: One or more required environment variables are not set.")
        print("Please set GITHUB_TOKEN, GITHUB_REPO_OWNER, and GITHUB_REPO_NAME and try again.")
        return

    issues_file_path = 'issues.jsonl'
    prs_file_path = 'prs.jsonl'
    issues_count = 0
    prs_count = 0

    try:
        # 2. Authenticate with GitHub using the new Auth class
        auth = Auth.Token(github_token)
        g = Github(auth=auth)
        repo = g.get_repo(f"{repo_owner}/{repo_name}")
        print(f"Connected to repository: {repo.full_name}\n")

        # 3. Open files for writing at the beginning
        with open(issues_file_path, 'w', encoding='utf-8') as issues_file, \
             open(prs_file_path, 'w', encoding='utf-8') as prs_file:
            
            # 4. Fetch all items (issues and pull requests)
            print("Fetching issues and pull requests...")
            all_items = repo.get_issues(state='all')
            
            for item in all_items:
                if item.body == "" or item.body is None:
                    print(f"  Skipping item #{item.number} with empty body...")
                    continue
                
                if item.get_comments().totalCount == 0:
                    print(f"  Skipping item #{item.number} with no comments...")
                    continue
                
                # Common data points for both issues and PRs
                common_data = {
                    'title': item.title.strip(),
                    'body': item.body.strip() if item.body else "",
                    'author': item.user.login,
                    'comments': get_item_comments(item)
                }

                # Differentiate and write to the correct file
                if item.pull_request is None:
                    # It's a standard issue
                    print(f"  Processing issue #{item.number}...")
                    data_point = {
                        'id': f'issue-{item.number}',
                        'type': 'issue',
                        **common_data
                    }
                    issues_file.write(json.dumps(data_point) + '\n')
                    issues_count += 1
                else:
                    # It's a pull request, check if it was merged
                    pr = item.as_pull_request()
                    if pr.merged:
                        print(f"  Processing merged pull request #{item.number}...")
                        data_point = {
                            'id': f'pr-{item.number}',
                            'type': 'pr',
                            **common_data
                        }
                        prs_file.write(json.dumps(data_point) + '\n')
                        prs_count += 1
                    else:
                        print(f"  Skipping unmerged pull request #{item.number}...")
    
    except GithubException as e:
        if e.status == 404:
            print(f"\nError: The repository '{repo_owner}/{repo_name}' was not found.")
            print("Please double-check the repository name and owner.")
            print("If it's a private repository, ensure your GITHUB_TOKEN has the 'repo' scope.")
        else:
            print(f"Error connecting to GitHub or fetching data: {e.data['message']}")
        return
    except Exception as e:
        print(f"An unexpected error occurred: {e}")
        return

    print(f"\nSuccessfully wrote {issues_count} issue records to {issues_file_path}")
    print(f"Successfully wrote {prs_count} merged pull request records to {prs_file_path}")

if __name__ == "__main__":
    main()
