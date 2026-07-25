"""Load AWS-related keys from repo .env into download/aws-session.env (never prints values)."""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ENV_PATH = ROOT / ".env"
OUT_PATH = ROOT / "download" / "aws-session.env"
WANTED = (
    "AWS_ACCESS_KEY_ID",
    "AWS_SECRET_ACCESS_KEY",
    "AWS_SESSION_TOKEN",
    "AWS_REGION",
    "AWS_DEFAULT_REGION",
    "AWS_BUCKET_NAME",
    "EC2_INSTANCE_ID",
    "EC2_HOST",
    "EC2_USER",
)

def main() -> None:
    if not ENV_PATH.exists():
        raise SystemExit(f"missing {ENV_PATH}")

    values: dict[str, str] = {}
    for line in ENV_PATH.read_text(encoding="utf-8", errors="ignore").splitlines():
        s = line.strip()
        if not s or s.startswith("#") or "=" not in s:
            continue
        key, raw = s.split("=", 1)
        key = key.strip()
        if key not in WANTED:
            continue
        val = raw.strip().strip('"').strip("'")
        if val:
            values[key] = val

    if "AWS_REGION" in values and "AWS_DEFAULT_REGION" not in values:
        values["AWS_DEFAULT_REGION"] = values["AWS_REGION"]
    if "AWS_DEFAULT_REGION" in values and "AWS_REGION" not in values:
        values["AWS_REGION"] = values["AWS_DEFAULT_REGION"]
    if "AWS_REGION" not in values:
        values["AWS_REGION"] = "eu-west-1"
        values["AWS_DEFAULT_REGION"] = "eu-west-1"

    OUT_PATH.parent.mkdir(parents=True, exist_ok=True)
    OUT_PATH.write_text("".join(f"{k}={values[k]}\n" for k in WANTED if k in values), encoding="utf-8")

    print("LOADED:", ",".join(k for k in WANTED if k in values) or "(none)")
    print("HAS_KEY_ID:", "yes" if "AWS_ACCESS_KEY_ID" in values else "no")
    print("HAS_SECRET:", "yes" if "AWS_SECRET_ACCESS_KEY" in values else "no")
    print("REGION:", values.get("AWS_REGION") or values.get("AWS_DEFAULT_REGION") or "(unset)")
    print("OUT:", OUT_PATH)

if __name__ == "__main__":
    main()
