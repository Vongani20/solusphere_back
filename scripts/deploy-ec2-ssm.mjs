import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const instanceId = process.env.EC2_INSTANCE_ID || "i-081024e774e826baa";
const region = process.env.AWS_REGION || process.env.AWS_DEFAULT_REGION || "eu-west-1";

const deployScript = `#!/bin/bash
set -euo pipefail
export HOME=/home/ec2-user
for d in /home/ec2-user/solusphere_backend /home/ec2-user/solusphere_back /home/ubuntu/solusphere_backend /home/ubuntu/solusphere_back /opt/solusphere_backend /opt/solusphere_back; do
  if [ -d "$d/.git" ]; then
    REPO="$d"
    break
  fi
done
if [ -z "$REPO" ]; then
  echo "REPO_NOT_FOUND"
  find /home /opt -maxdepth 3 -name .git -type d 2>/dev/null || true
  exit 1
fi
echo "Using repo: $REPO"
git config --global --add safe.directory "$REPO"
cd "$REPO"
git fetch origin main
git checkout main
git pull --ff-only origin main
git log -1 --oneline
if docker compose version >/dev/null 2>&1; then
  COMPOSE="docker compose"
elif docker-compose version >/dev/null 2>&1; then
  COMPOSE="docker-compose"
else
  echo "DOCKER_COMPOSE_NOT_FOUND"
  exit 1
fi
$COMPOSE build app
$COMPOSE up -d app
$COMPOSE ps
curl -s http://localhost:2080/health
`;

function run(command, args) {
  const result = spawnSync(command, args, {
    encoding: "utf8",
    shell: false,
  });

  if (result.status !== 0) {
    console.error(result.stderr || result.stdout);
    process.exit(result.status ?? 1);
  }

  return result.stdout;
}

const paramsPath = path.join(__dirname, "ssm-deploy-backend.json");
fs.writeFileSync(
  paramsPath,
  JSON.stringify({
    commands: [deployScript],
  })
);

console.log(`Deploying backend on EC2 ${instanceId} via SSM...`);

const sendOut = JSON.parse(
  run("aws", [
    "ssm",
    "send-command",
    "--region",
    region,
    "--instance-ids",
    instanceId,
    "--document-name",
    "AWS-RunShellScript",
    "--comment",
    "Deploy solusphere backend",
    "--parameters",
    `file://${paramsPath.replace(/\\/g, "/")}`,
    "--output",
    "json",
  ])
);

const commandId = sendOut.Command?.CommandId;
if (!commandId) {
  console.error("Failed to start SSM command", sendOut);
  process.exit(1);
}

console.log("SSM command started:", commandId);

for (let attempt = 0; attempt < 24; attempt += 1) {
  const statusOut = JSON.parse(
    run("aws", [
      "ssm",
      "get-command-invocation",
      "--region",
      region,
      "--command-id",
      commandId,
      "--instance-id",
      instanceId,
      "--output",
      "json",
    ])
  );

  if (statusOut.Status === "Success") {
    console.log(statusOut.StandardOutputContent || "(no stdout)");
    console.log("Backend deploy completed.");
    process.exit(0);
  }

  if (statusOut.Status === "Failed" || statusOut.Status === "Cancelled" || statusOut.Status === "TimedOut") {
    console.error(statusOut.StandardErrorContent || statusOut.StandardOutputContent || statusOut.Status);
    process.exit(1);
  }

  run("powershell", ["-Command", "Start-Sleep -Seconds 5"]);
}

console.error("Timed out waiting for backend deploy.");
process.exit(1);
