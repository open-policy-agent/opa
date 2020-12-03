#!/usr/bin/env python
"""
changelog.py helps generate the CHANGELOG.md message for a particular release.
"""

import argparse
import subprocess
import shlex
import re
import urllib2
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
    req = urllib2.Request(url)
    if token:
        req.add_header('Authorization', "token {}".format(token))
    try:
        rsp = urllib2.urlopen(req)
        result = json.loads(rsp.read())
    except Exception as e:
        if hasattr(e, 'reason'):
            print >> sys.stderr, 'Failed to fetch URL {}: {}'.format(url, e.reason)
        elif hasattr(e, 'code'):
            print >> sys.stderr, 'Failed to fetch URL {}: Code {}'.format(url, e.code)
        return {}
    else:
        return result


def get_maintainers():
    with open("MAINTAINERS.md", "r") as f:
        contents = f.read()
    maintainers = re.findall(r"[^\s]+@[^\s]+", contents)
    return maintainers

maintainers = get_maintainers()


def is_maintainer(commit_message):
    author = author_email(commit_message)
    return author in maintainers


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
    if username:
        return "authored by @[{author}](https://github.com/{author})".format(author=username)
    return ""


def fixes_issue_id(commit_message):
    match = re.search(r"Fixes:?\s*#(\d+)", commit_message)
    if match:
        return match.group(1)


def get_subject(commit_message):
    return commit_message.splitlines()[0]


def get_changelog_message(commit_message, mention, repo_url):
    issue_id = fixes_issue_id(commit_message)
    if issue_id:
        subject = get_subject(commit_message)
        return "Fixes", "{subject} ([#{issue_id}]({repo_url}/issues/{issue_id})) {mention}".format(subject=subject, issue_id=issue_id, repo_url=repo_url, mention=mention)
    return None, get_subject(commit_message)


def get_latest_tag():
    cmd = "git describe --tags --first-parent"
    return run(cmd).split('-')[0]


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--repo_url", default="https://github.com/open-policy-agent/opa")
    parser.add_argument("--token", default="", help="GitHub API token")
    parser.add_argument("from_version", nargs="?",
                        default=get_latest_tag(), help="start of changes")
    parser.add_argument("to_commit", nargs="?",
                        default="HEAD", help="end of changes")
    return parser.parse_args()


def main():

    args = parse_args()
    changelog = {}
    for commit_id in get_commit_ids(args.from_version, args.to_commit):
        mention = ""
        commit_message = get_commit_message(commit_id)
        if not is_maintainer(commit_message):
            mention = mention_author(commit_message, commit_id, args.token)
        group, line = get_changelog_message(commit_message, mention, args.repo_url)
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
