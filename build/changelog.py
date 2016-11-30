#!/usr/bin/env python
"""
changelog.py helps generate the CHANGELOG.md message for a particular release.
"""

import argparse
import subprocess
import shlex
import re


def run(cmd, *args, **kwargs):
    return subprocess.check_output(shlex.split(cmd), *args, **kwargs)


def get_commit_ids(from_commit, to_commit):
    cmd = "git log --format=%H --no-merges {from_commit}..{to_commit}"
    commit_ids = run(cmd.format(from_commit=from_commit,
                                to_commit=to_commit)).splitlines()
    return commit_ids


def get_commit_message(commit_id):
    cmd = "git log --format=%B --max-count=1 {commit_id}".format(
        commit_id=commit_id)
    return run(cmd)


def fixes_issue_id(commit_message):
    match = re.search(r"Fixes #(\d+)", commit_message)
    if match:
        return match.group(1)


def get_subject(commit_message):
    return commit_message.splitlines()[0]


def get_changelog_message(commit_message, repo_url):
    issue_id = fixes_issue_id(commit_message)
    if issue_id:
        subject = get_subject(commit_message)
        return "Fixes", "{subject} ([#{issue_id}]({repo_url}/issues/{issue_id}))".format(subject=subject, issue_id=issue_id, repo_url=repo_url)
    return None, get_subject(commit_message)


def get_latest_tag():
    cmd = "git describe --tags --first-parent"
    return run(cmd).split('-')[0]


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--repo_url", default="https://github.com/open-policy-agent/opa")
    parser.add_argument("from_version", nargs="?",
                        default=get_latest_tag(), help="start of changes")
    parser.add_argument("to_commit", nargs="?",
                        default="HEAD", help="end of changes")
    return parser.parse_args()


def main():

    args = parse_args()
    changelog = {}

    for commit_id in get_commit_ids(args.from_version, args.to_commit):
        commit_message = get_commit_message(commit_id)
        group, line = get_changelog_message(commit_message, args.repo_url)
        changelog.setdefault(group, []).append(line)

    if "Fixes" in changelog:
        print "### Fixes"
        print ""
        for line in sorted(changelog["Fixes"]):
            print "- {}".format(line)
        print ""

    if None in changelog:
        print "### Miscellaneous"
        print ""
        for line in sorted(changelog[None]):
            print "- {}".format(line)


if __name__ == "__main__":
    main()
