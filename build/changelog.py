#!/usr/bin/env python3
"""
changelog.py helps generate the CHANGELOG.md message for a particular release.
"""

import argparse
import os
import subprocess
import shlex
import re
import urllib.request, urllib.error, urllib.parse
import sys
import json


def run(cmd, *args, **kwargs):
    return subprocess.check_output(shlex.split(cmd), *args, **kwargs).decode('utf-8')


def get_commit_ids(from_commit, to_commit):
    cmd = "git log --format=%H --no-merges {from_commit}..{to_commit}"
    commit_ids = run(cmd.format(from_commit=from_commit,
                                to_commit=to_commit)).splitlines()
    return commit_ids


def get_commit_message(commit_id):
    cmd = "git log --format=%B --max-count=1 {commit_id}".format(
        commit_id=commit_id)
    return run(cmd)


def fetch(url, token):
    req = urllib.request.Request(url)
    if token:
        req.add_header('Authorization', "token {}".format(token))
    try:
        rsp = urllib.request.urlopen(req)
        result = json.loads(rsp.read())
    except Exception as e:
        if hasattr(e, 'reason'):
            print('Failed to fetch URL {}: {}'.format(url, e.reason), file=sys.stderr)
        elif hasattr(e, 'code'):
            print('Failed to fetch URL {}: Code {}'.format(url, e.code), file=sys.stderr)
        return {}
    else:
        return result

org_members_usernames = []
def get_org_members(token):
    url = "https://api.github.com/orgs/open-policy-agent/members?per_page=100"
    r = fetch(url, token)
    for m in r:
        user_url = m.get('url', '')
        user_info = fetch(user_url, token)
        email = user_info.get('email', '')
        login = user_info.get('login', '')
        if login:
            org_members_usernames.append(str(login))
        if email:
            github_ids[email]=login

def author_email(commit_message):
    match = re.search(r"<(.*@.*)>", commit_message)
    if match:
        author = match.group(1)
        return str(author)
    return ""

github_ids = {}
def get_github_id(commit_message, commit_id, token):
    email = author_email(commit_message)
    if github_ids.get(email, ""):
        return github_ids[email]
    url = "https://api.github.com/repos/open-policy-agent/opa/commits/{}".format(commit_id)
    r = fetch(url, token)
    author = r.get('author', {})
    if author is None:
        return ""
    login = author.get('login', '')
    if login:
        github_ids[email]=login
        return login
    return ""


def mention_author(commit_message, commit_id, token):
    username = get_github_id(commit_message, commit_id, token)
    return "authored by @{author}".format(author=username)

def get_issue_reporter(issue_id, token):
    url = "https://api.github.com/repos/open-policy-agent/opa/issues/{issue_id}".format(issue_id=issue_id)
    issue_data = fetch(url, token)
    username = issue_data.get("user", "").get("login", "")
    if username not in org_members_usernames:
        return "reported by @{reporter}".format(reporter=username)
    return ""

def fixes_issue_id(commit_message):
    match = re.search(r"Fixes:?\s*#(\d+)", commit_message)
    if match:
        return match.group(1)


def get_subject(commit_message):
    return commit_message.splitlines()[0]

def get_changelog_message(commit_message, issue_id, mention, reporter, repo_url):
    subject = get_subject(commit_message)
    if issue_id:
        if mention:
            mention = " "+mention
        if reporter:
            reporter = " "+reporter
        return "Fixes", "{subject} ([#{issue_id}]({repo_url}/issues/{issue_id})){mention}{reporter}".format(subject=subject, issue_id=issue_id, repo_url=repo_url, mention=mention, reporter=reporter)
    if mention:
        mention = " (" + mention + ")"
    return None, "{subject}{mention}".format(subject=subject, mention=mention)


def get_latest_tag():
    cmd = "git describe --tags --first-parent"
    return run(cmd).split('-')[0]


def parse_args():
    if "GITHUB_TOKEN" in os.environ:
        default_token = os.environ["GITHUB_TOKEN"]
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--repo_url", default="https://github.com/open-policy-agent/opa")
    parser.add_argument("--token", default=default_token, help="GitHub API token")
    parser.add_argument("from_version", nargs="?",
                        default=get_latest_tag(), help="start of changes")
    parser.add_argument("to_commit", nargs="?",
                        default="HEAD", help="end of changes")
    return parser.parse_args()


def main():

    args = parse_args()
    changelog = {}
    get_org_members(args.token)
    for commit_id in get_commit_ids(args.from_version, args.to_commit):
        mention = ""
        reporter = ""
        commit_message = get_commit_message(commit_id)
        mention = mention_author(commit_message, commit_id, args.token)
        issue_id = fixes_issue_id(commit_message)
        if issue_id:
            reporter = get_issue_reporter(issue_id, args.token)
        if mention.split("/")[-1] == reporter.split("/")[-1]:
            reporter = ""
        group, line = get_changelog_message(commit_message, issue_id, mention, reporter, args.repo_url)
        changelog.setdefault(group, []).append(line)

    if "Fixes" in changelog:
        print("### Fixes")
        print("")
        for line in sorted(changelog["Fixes"]):
            print("- {}".format(line))
        print("")

    if None in changelog:
        print("### Miscellaneous")
        print("")
        for line in sorted(changelog[None]):
            print("- {}".format(line))


if __name__ == "__main__":
    main()
