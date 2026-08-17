#!/usr/bin/env python3
"""Create Gitee releases for tags, keeping version-descending display order.

Gitee lists releases by creation time (oldest first), so to show the newest
version at the top we must create releases in descending version order. To keep
that invariant after a new release, this script deletes all existing releases
and recreates them in descending order.

Usage: gitee_release.py <gitee_username> <gitee_token> [tag ...]
       If no tag is given, all tags are used.
"""

import json
import re
import subprocess
import sys
import urllib.parse
import urllib.request


def api_request(url, token=None, method="GET", data=None):
    headers = {"User-Agent": "jm-gitee-sync"}
    if data is not None:
        data = urllib.parse.urlencode(data).encode()
        req = urllib.request.Request(url, data=data, method=method, headers=headers)
    else:
        req = urllib.request.Request(url, method=method, headers=headers)
    with urllib.request.urlopen(req, timeout=30) as resp:
        raw = resp.read().decode()
        return json.loads(raw) if raw else {}


def list_releases(username, token):
    url = "https://gitee.com/api/v5/repos/%s/jvmtool/releases" % username
    return api_request(url + "?access_token=" + urllib.parse.quote(token))


def delete_release(username, token, release_id):
    url = "https://gitee.com/api/v5/repos/%s/jvmtool/releases/%s" % (username, release_id)
    api_request(url, token, method="DELETE", data={"access_token": token})


def create_release(username, token, tag, body):
    url = "https://gitee.com/api/v5/repos/%s/jvmtool/releases" % username
    return api_request(url, token, method="POST", data={
        "access_token": token,
        "tag_name": tag,
        "name": tag,
        "body": body,
        "target_commitish": "main",
        "prerelease": "false",
    })


def extract_changelog_section(version):
    with open("CHANGELOG.md", encoding="utf-8") as f:
        text = f.read()
    pattern = re.compile(
        r"## \[%s\] - [^\n]*\n(.*?)(?=\n## \[|\Z)" % re.escape(version),
        re.DOTALL,
    )
    m = pattern.search(text)
    return m.group(1).strip() if m else ""


def version_key(tag):
    """Convert vX.Y.Z to a sortable tuple."""
    parts = tag.lstrip("v").split(".")
    out = []
    for p in parts:
        num = re.match(r"\d+", p)
        out.append(int(num.group()) if num else 0)
    while len(out) < 4:
        out.append(0)
    return tuple(out)


def list_tags():
    tags = subprocess.run(
        ["git", "tag", "-l"], capture_output=True, text=True, check=True
    ).stdout.split()
    return [t for t in tags if re.match(r"^v\d+\.\d+\.\d+$", t)]


def main():
    if len(sys.argv) < 3:
        print("usage: gitee_release.py <username> <token> [tag ...]", file=sys.stderr)
        sys.exit(2)

    username, token = sys.argv[1], sys.argv[2]
    tags = sys.argv[3:] if len(sys.argv) > 3 else list_tags()

    # 按版本号降序排列（新版本先创建，从而显示在顶部）
    tags.sort(key=version_key, reverse=True)

    # 删除全部旧 release
    for rel in list_releases(username, token):
        rel_id = rel.get("id")
        tag_name = rel.get("tag_name", "?")
        delete_release(username, token, rel_id)
        print("deleted release:", tag_name)

    # 按降序重新创建
    for tag in tags:
        body = extract_changelog_section(tag.lstrip("v")) or ("Release " + tag)
        create_release(username, token, tag, body)
        print("created release:", tag)


if __name__ == "__main__":
    main()
