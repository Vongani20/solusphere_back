# Solusphere Backend AWS ECS Deployment

This repo is prepared for ECS Fargate while keeping local development secrets in `.env`.

## Local Development

- Keep `.env` on your machine only.
- `.env.example` is the safe template.
- `docker-compose.yml` remains for local MySQL and local app testing.

## Production Architecture

- ECS Fargate runs the backend container.
- ECR stores the Docker image.
- RDS MySQL stores application data.
- S3 stores face images and uploads.
- Rekognition handles face indexing/search.
- SNS sends password reset SMS messages.
- Secrets Manager stores app secrets.
- CloudWatch Logs stores app logs.
- An Application Load Balancer should point to container port `2080`.

## Required AWS Resources

1. ECR repository: `solusphere-backend`
2. ECS cluster and Fargate service
3. RDS MySQL database
4. S3 bucket for uploads/faces
5. CloudWatch log group: `/ecs/solusphere-backend`
6. Secrets Manager secret: `solusphere/prod`
7. ECS task execution role
8. ECS task role
9. Application Load Balancer target group health check: `/health`

## Secrets Manager

Create a JSON secret using:

```bash
aws secretsmanager create-secret \
  --name solusphere/prod \
  --secret-string file://aws/secrets/solusphere-prod.example.json
```

Replace every placeholder first. Do not store real secrets in Git.

## ECS Task Definition

Use `aws/ecs/task-definition.json` as the base template.

Replace:

- `<account-id>`
- `<region>`
- `<s3-bucket-name>`
- IAM role ARNs
- ECR image URI

The app uses `USE_AWS_SECRETS_MANAGER=true` in ECS and reads app secrets from `solusphere/prod`.

## IAM

Attach `aws/iam/ecs-task-role-policy.json` to the ECS task role.

Attach `aws/iam/ecs-execution-role-policy.json` to the ECS execution role, or use the AWS-managed `AmazonECSTaskExecutionRolePolicy` and add log/ECR permissions as needed.

Production AWS credentials should come from the ECS task role, not `AWS_ACCESS_KEY_ID` or `AWS_SECRET_ACCESS_KEY`.

## GitHub Actions

The workflow at `.github/workflows/deploy-ecs.yml` builds and deploys on pushes to `main`.

Set this GitHub secret:

- `AWS_GITHUB_DEPLOY_ROLE_ARN`

Set these GitHub repository variables:

- `AWS_REGION`
- `ECS_CLUSTER`
- `ECS_SERVICE`

The GitHub deploy role should trust GitHub OIDC for this repository and have permission to push ECR images and update the ECS service.

## Health Check

Use:

```text
/health
```

Expected healthy response includes database and OpenAI/AWS configuration status.

## Before Production

- Rotate any secrets that were ever stored locally.
- Confirm RDS security group allows inbound MySQL only from the ECS service security group.
- Confirm the ALB security group only exposes HTTPS.
- Confirm S3 bucket public access is blocked unless you intentionally serve public assets.
- Confirm SNS SMS spend limits and region support.
