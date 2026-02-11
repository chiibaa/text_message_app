# Text Messaging App - AWS Infrastructure Architecture

This diagram illustrates the complete AWS infrastructure for the Text Messaging App, including network topology, compute resources, database, container registry, CI/CD pipeline, and monitoring.

```mermaid
graph TB
    subgraph GitHub["GitHub"]
        GH["GitHub Repository"]
        GHA["GitHub Actions"]
        GHCI["CI Pipeline<br/>Test, Lint, Build"]
        GHCD["CD Pipeline<br/>ECR Push, ECS Update,<br/>CodeDeploy Blue/Green"]
    end

    subgraph Internet["Internet"]
        IGW["Internet Gateway"]
        CLIENT["Client/Browser"]
    end

    subgraph AWS_VPC["AWS VPC"]
        subgraph AZ_A["Availability Zone A"]
            subgraph PubSubnetA["Public Subnet A"]
                ALB["Application Load Balancer<br/>Port: 80, 8080<br/>Internet-facing"]
            end

            subgraph PrivSubnetA["Private Subnet A"]
                ECS_A["ECS Fargate Task<br/>0.25 vCPU, 512MB RAM<br/>Port: 8080"]
            end

            subgraph IsolSubnetA["Isolated Subnet A"]
                RDS_Primary["RDS PostgreSQL 16.4<br/>db.t3.micro<br/>Primary"]
            end
        end

        subgraph AZ_C["Availability Zone C"]
            subgraph PubSubnetC["Public Subnet C"]
                NAT["NAT Gateway"]
            end

            subgraph PrivSubnetC["Private Subnet C"]
                ECS_C["ECS Fargate Task<br/>0.25 vCPU, 512MB RAM<br/>Port: 8080"]
            end

            subgraph IsolSubnetC["Isolated Subnet C"]
                RDS_Standby["RDS Standby Replica<br/>or Read Replica"]
            end
        end

        subgraph SecurityGroups["Security Groups"]
            SG_ALB["ALB SG<br/>IN: TCP 80, 443<br/>from Internet"]
            SG_ECS["ECS SG<br/>IN: TCP 8080<br/>from ALB SG"]
            SG_RDS["RDS SG<br/>IN: TCP 5432<br/>from ECS SG"]
        end

        subgraph TargetGroups["Target Groups"]
            TG_Blue["Blue Target Group<br/>Port 8080"]
            TG_Green["Green Target Group<br/>Port 8080"]
        end

        subgraph AutoScaling["Auto Scaling"]
            ASG["ECS Service Auto Scaling<br/>Min: 1, Max: 4<br/>Target: CPU/Memory 70%<br/>Cooldown: 300s"]
        end

        subgraph SecretsManager["Secrets Manager"]
            DB_Creds["Database Credentials<br/>RDS User/Pass"]
        end

        subgraph CodeDeploy["CodeDeploy"]
            CD_Config["Blue/Green Deployment<br/>Config: ECSAllAtOnce<br/>Traffic Switch: Immediate<br/>Auto Rollback: ON"]
        end
    end

    subgraph Container_Registry["Container Registry"]
        ECR["ECR Repository<br/>text-messaging-app-dev<br/>Scan on Push: Enabled"]
    end

    subgraph Monitoring["Monitoring & Logging"]
        CloudWatch["CloudWatch Container Insights<br/>/ecs/text-messaging-dev"]
        Logs["CloudWatch Logs"]
    end

    %% Traffic Flow
    CLIENT -->|HTTP/HTTPS| IGW
    IGW -->|Route| ALB
    ALB -->|Port 80| TG_Blue
    ALB -->|Port 8080| TG_Green
    TG_Blue -->|Route| ECS_A
    TG_Blue -->|Route| ECS_C
    TG_Green -->|Route| ECS_A
    TG_Green -->|Route| ECS_C
    ECS_A -->|TCP 5432| RDS_Primary
    ECS_C -->|TCP 5432| RDS_Primary
    ECS_A -->|Logs| CloudWatch
    ECS_C -->|Logs| CloudWatch
    ECS_A -->|Read| DB_Creds
    ECS_C -->|Read| DB_Creds

    %% NAT Gateway for outbound traffic
    ECS_A -->|Outbound| NAT
    ECS_C -->|Outbound| NAT
    NAT -->|Internet Access| IGW

    %% Auto Scaling
    ASG -->|Scale| ECS_A
    ASG -->|Scale| ECS_C

    %% CI/CD Pipeline
    GH -->|Trigger| GHCI
    GHCI -->|Build Image| ECR
    GHCI -->|Trigger| GHCD
    GHCD -->|Push Image| ECR
    GHCD -->|Update Task Def| TargetGroups
    GHCD -->|Initiate| CD_Config
    CD_Config -->|Deploy| TG_Green
    CD_Config -->|Traffic Switch| ALB

    %% Security Groups Relationships
    ALB -.->|Associated| SG_ALB
    ECS_A -.->|Associated| SG_ECS
    ECS_C -.->|Associated| SG_ECS
    RDS_Primary -.->|Associated| SG_RDS
    RDS_Standby -.->|Associated| SG_RDS
    SG_ALB -.->|Allows| SG_ECS
    SG_ECS -.->|Allows| SG_RDS

    %% Styling
    classDef internet fill:#FFE6E6,stroke:#CC0000,stroke-width:2px,color:#000
    classDef vpc fill:#E6F2FF,stroke:#0066CC,stroke-width:2px,color:#000
    classDef public fill:#FFE6CC,stroke:#FF9900,stroke-width:2px,color:#000
    classDef private fill:#E6CCFF,stroke:#9900CC,stroke-width:2px,color:#000
    classDef isolated fill:#CCE6FF,stroke:#0099FF,stroke-width:2px,color:#000
    classDef compute fill:#E6FFE6,stroke:#00CC00,stroke-width:2px,color:#000
    classDef database fill:#FFE6F0,stroke:#CC0066,stroke-width:2px,color:#000
    classDef network fill:#FFFFE6,stroke:#CCCC00,stroke-width:2px,color:#000
    classDef security fill:#F0E6FF,stroke:#6600CC,stroke-width:2px,color:#000
    classDef cicd fill:#FFE6D9,stroke:#CC6600,stroke-width:2px,color:#000
    classDef monitoring fill:#E6F9FF,stroke:#00CCCC,stroke-width:2px,color:#000

    class IGW,CLIENT internet
    class AWS_VPC vpc
    class PubSubnetA,PubSubnetC,NAT public
    class PrivSubnetA,PrivSubnetC,ECS_A,ECS_C private
    class IsolSubnetA,IsolSubnetC isolated
    class ALB,TG_Blue,TG_Green network
    class ECS_A,ECS_C,ASG compute
    class RDS_Primary,RDS_Standby,DB_Creds database
    class SG_ALB,SG_ECS,SG_RDS,SecurityGroups security
    class GH,GHCI,GHCD,ECR,CD_Config cicd
    class CloudWatch,Logs monitoring
```

## Architecture Components

### Network Layer
- **VPC**: Spans 2 Availability Zones for high availability
- **Public Subnets**: Host ALB and NAT Gateway for external traffic routing
- **Private Subnets**: Host ECS Fargate tasks with outbound NAT access
- **Isolated Subnets**: Host RDS PostgreSQL with no internet access
- **Internet Gateway**: Routes inbound traffic from clients
- **NAT Gateway**: Provides outbound internet access for ECS tasks

### Security Layer
- **ALB Security Group**: Accepts HTTP/HTTPS from internet
- **ECS Security Group**: Accepts port 8080 only from ALB
- **RDS Security Group**: Accepts port 5432 only from ECS tasks
- **Secrets Manager**: Manages RDS database credentials securely

### Compute Layer
- **ECS Fargate**: Serverless container execution (0.25 vCPU, 512MB RAM)
- **Auto Scaling**: Scales between 1-4 tasks based on CPU/Memory (target 70%)
- **Task Distribution**: Distributed across 2 AZs for fault tolerance

### Load Balancing
- **ALB**: Internet-facing with dual listeners
- **Blue Target Group**: Production traffic on port 80
- **Green Target Group**: Staging/test traffic on port 8080
- **Blue/Green Deployment**: Enables safe, zero-downtime deployments

### Database Layer
- **RDS PostgreSQL 16.4**: db.t3.micro instance
- **Multi-AZ**: Configured for production reliability
- **Isolated Subnets**: No direct internet access
- **Credentials**: Managed by AWS Secrets Manager

### Container Registry
- **ECR Repository**: Stores Docker images for the application
- **Scan on Push**: Automatically scans images for vulnerabilities
- **ImagePullCredentials**: Used by ECS tasks to pull container images

### CI/CD Pipeline
- **GitHub Actions CI**: Runs tests, linting, and builds Docker images
- **GitHub Actions CD**: Uses OIDC for secure AWS authentication
- **ECR Push**: Publishes built images to container registry
- **ECS Task Definition**: Updated with new image reference
- **CodeDeploy Blue/Green**: Orchestrates safe deployment with traffic switching
- **Auto Rollback**: Reverts to Blue environment if deployment fails

### Monitoring & Logging
- **CloudWatch Container Insights**: Monitors ECS cluster performance
- **CloudWatch Logs**: Centralized logging for all ECS tasks
- **Log Group**: `/ecs/text-messaging-dev` aggregates all application logs

## Traffic Flow

1. **Inbound**: Client → Internet Gateway → ALB (Port 80/8080)
2. **Routing**: ALB → Blue/Green Target Groups → ECS Tasks
3. **Database**: ECS Tasks → RDS PostgreSQL (Port 5432)
4. **Outbound**: ECS Tasks → NAT Gateway → Internet Gateway
5. **Deployment**: GitHub Actions → ECR → ECS Task Definition → CodeDeploy → ALB Traffic Switch

## Key Features

- **High Availability**: Multi-AZ deployment with auto-scaling
- **Zero-Downtime Deployments**: Blue/Green strategy via CodeDeploy
- **Security**: Network segmentation with security groups and isolated database subnet
- **Cost Optimization**: Single NAT Gateway, Fargate spot instances, minimal task sizing
- **Observability**: Comprehensive logging and monitoring via CloudWatch
- **Automation**: CI/CD pipeline fully automated with GitHub Actions
