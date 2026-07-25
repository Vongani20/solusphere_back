"""Allow /api/cv/import through CloudFront WAF (same carve-out as other uploads)."""
import base64
import json
import subprocess
import tempfile
from pathlib import Path

SCOPE = "CLOUDFRONT"
REGION = "us-east-1"
ACL_ID = "4e93a634-174a-4bbb-971d-599b942b36b8"
ACL_NAME = "CreatedByCloudFront-a2f93c77"

NEW_PATHS = [
    "/api/cv/import",
]


def aws_json(args):
    return json.loads(subprocess.check_output(args, text=True))


def path_match_statement(path: str) -> dict:
    return {
        "ByteMatchStatement": {
            "SearchString": base64.b64encode(path.encode("utf-8")).decode("ascii"),
            "FieldToMatch": {"UriPath": {}},
            "TextTransformations": [{"Priority": 0, "Type": "LOWERCASE"}],
            "PositionalConstraint": "STARTS_WITH",
        }
    }


def decode_search(b64: str) -> str:
    pad = "=" * (-len(b64) % 4)
    return base64.b64decode(b64 + pad).decode("utf-8", errors="replace")


def main():
    data = aws_json([
        "aws", "wafv2", "get-web-acl",
        "--scope", SCOPE,
        "--id", ACL_ID,
        "--name", ACL_NAME,
        "--region", REGION,
        "--output", "json",
    ])
    lock = data["LockToken"]
    acl = data["WebACL"]
    rules = acl["Rules"]
    changed = False

    for rule in rules:
        stmt = rule.get("Statement", {}).get("ManagedRuleGroupStatement")
        if not stmt or stmt.get("Name") != "AWSManagedRulesCommonRuleSet":
            continue

        # 1) Scope-down: skip CRS for known multipart upload endpoints
        or_stmts = (
            stmt.get("ScopeDownStatement", {})
            .get("NotStatement", {})
            .get("Statement", {})
            .get("OrStatement", {})
            .get("Statements", [])
        )
        existing_paths = set()
        for s in or_stmts:
            b64 = s.get("ByteMatchStatement", {}).get("SearchString")
            if b64:
                existing_paths.add(decode_search(b64))
        print("Existing upload carve-outs:", sorted(existing_paths))

        for path in NEW_PATHS:
            if path in existing_paths:
                print("Already present:", path)
                continue
            or_stmts.append(path_match_statement(path))
            changed = True
            print("Added carve-out:", path)

        stmt.setdefault("ScopeDownStatement", {}).setdefault("NotStatement", {}).setdefault(
            "Statement", {}
        )["OrStatement"] = {"Statements": or_stmts}

        # 2) Keep body XSS in count mode as a safety net for other routes
        overrides = stmt.get("RuleActionOverrides") or []
        override_names = {o["Name"] for o in overrides}
        for name in ("SizeRestrictions_BODY", "CrossSiteScripting_BODY"):
            if name in override_names:
                continue
            overrides.append({"Name": name, "ActionToUse": {"Count": {}}})
            changed = True
            print("Added Count override:", name)
        stmt["RuleActionOverrides"] = overrides

    if not changed:
        print("No change needed")
        return

    update = {
        "Name": acl["Name"],
        "Scope": SCOPE,
        "Id": acl["Id"],
        "DefaultAction": acl["DefaultAction"],
        "Description": acl.get("Description") or "SoluSphere CloudFront WAF",
        "Rules": rules,
        "VisibilityConfig": acl["VisibilityConfig"],
        "LockToken": lock,
    }
    for optional in ("CustomResponseBodies", "CaptchaConfig", "ChallengeConfig", "TokenDomains"):
        if optional in acl:
            update[optional] = acl[optional]

    tmp = Path(tempfile.gettempdir()) / "solusphere-waf-update.json"
    tmp.write_text(json.dumps(update), encoding="utf-8")
    result = aws_json([
        "aws", "wafv2", "update-web-acl",
        "--cli-input-json", f"file://{tmp}",
        "--region", REGION,
        "--output", "json",
    ])
    print("Updated OK. NextLockToken:", bool(result.get("NextLockToken")))


if __name__ == "__main__":
    main()
