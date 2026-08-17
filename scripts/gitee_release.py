#!/usr/bin/env python3
"""Create Gitee releases for tags, mirroring GitHub release assets.

Gitee lists releases by creation time (oldest first on the web UI), so to show
the newest version at the top we create releases in descending version order.
This script also downloads release assets from GitHub and uploads them to the
corresponding Gitee release.

Usage: gitee_release.py <gitee_username> <gitee_token> [--github-owner OWNER] [--github-token TOKEN] [tag ...]
       If no tag is given, all tags are used.
"""

import argparse
import json
import os
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
    with urllib.request.urlopen(req, timeout=60) as resp:
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


def upload_asset(username, token, release_id, filepath, filename):
    """Upload a file as a release attachment (multipart/form-data)."""
    url = "https://gitee.com/api/v5/repos/%s/jvmtool/releases/%s/attach_files" % (
        username, release_id)
    boundary = "----jm-sync-boundary-7d4a1f"
    with open(filepath, "rb") as f:
        content = f.read()
    body = b""
    body += ("--%s\r\n" % boundary).encode()
    body += ('Content-Disposition: form-data; name="access_token"\r\n\r\n').encode()
    body += (token + "\r\n").encode()
    body += ("--%s\r\n" % boundary).encode()
    body += ('Content-Disposition: form-data; name="file"; filename="%s"\r\n' % filename).encode()
    body += b"Content-Type: application/octet-stream\r\n\r\n"
    body += content
    body += ("\r\n--%s--\r\n" % boundary).encode()

    req = urllib.request.Request(url, data=body, method="POST")
    req.add_header("Content-Type", "multipart/form-data; boundary=%s" % boundary)
    req.add_header("User-Agent", "jm-gitee-sync")
    with urllib.request.urlopen(req, timeout=120) as resp:
        result = json.loads(resp.read().decode())
        return result.get("name", "?")


def list_github_assets(github_owner, github_token, tag):
    """Fetch the asset (name, download_url) list of a GitHub release."""
    url = "https://api.github.com/repos/%s/jvmtool/releases/tags/%s" % (github_owner, tag)
    headers = {"User-Agent": "jm-gitee-sync", "Accept": "application/vnd.github+json"}
    if github_token:
        headers["Authorization"] = "Bearer " + github_token
    req = urllib.request.Request(url, headers=headers)
    with urllib.request.urlopen(req, timeout=60) as resp:
        rel = json.loads(resp.read().decode())
    return [(a["name"], a["browser_download_url"]) for a in rel.get("assets", [])]


def download(url, dest):
    req = urllib.request.Request(url, headers={"User-Agent": "jm-gitee-sync"})
    with urllib.request.urlopen(req, timeout=300) as resp, open(dest, "wb") as f:
        while True:
            chunk = resp.read(1024 * 256)
            if not chunk:
                break
            f.write(chunk)


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
    parser = argparse.ArgumentParser(description="Sync releases to Gitee")
    parser.add_argument("gitee_username")
    parser.add_argument("gitee_token")
    parser.add_argument("--github-owner", default="dowdsa")
    parser.add_argument("--github-token", default="")
    parser.add_argument("tags", nargs="*")
    args = parser.parse_args()

    username = args.gitee_username
    token = args.gitee_token
    tags = args.tags if args.tags else list_tags()

    # 降序：最新版本先创建，使其在 Gitee 网页列表顶部
    tags.sort(key=version_key, reverse=True)

    # 删除全部旧 release
    for rel in list_releases(username, token):
        rel_id = rel.get("id")
        tag_name = rel.get("tag_name", "?")
        delete_release(username, token, rel_id)
        print("deleted release:", tag_name)

    # 重新创建并上传资产
    for tag in tags:
        body = extract_changelog_section(tag.lstrip("v")) or ("Release " + tag)
        rel = create_release(username, token, tag, body)
        release_id = rel.get("id")
        print("created release:", tag, "(id=%s)" % release_id)

        # 从 GitHub 下载资产并上传到 Gitee
        try:
            assets = list_github_assets(args.github_owner, args.github_token, tag)
        except Exception as e:
            print("  skip assets (github release not found):", e)
            assets = []
        for name, url in assets:
            tmp = "/tmp/jm-asset-" + name
            try:
                print("  downloading %s ..." % name)
                download(url, tmp)
                uploaded = upload_asset(username, token, release_id, tmp, name)
                print("  uploaded:", uploaded)
            except Exception as e:
                print("  failed to upload %s: %s" % (name, e))
            finally:
                if os.path.exists(tmp):
                    os.remove(tmp)


if __name__ == "__main__":
    main()
