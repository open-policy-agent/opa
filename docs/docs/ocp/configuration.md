---
sidebar_position: 4
title: Configuration
---

## Database Configuration

OCP uses a SQL-compatible database to store its internal state. By default, OCP runs with an in-memory SQLite3 database, which is ideal for development or testing. For production, you can configure it to use external databases such as PostgreSQL, MySQL, or Amazon RDS.

### Supported Database Backends

OCP supports the following database drivers:

- `sqlite3` (in-memory or file-based)
- `postgres`
- `mysql`
- `aws_rds` (PostgreSQL or MySQL)

### **Basic SQL Database Configuration**

To configure OCP to use a standard SQL database, create a `database.yaml` file in your config.d directory and specify the driver and DSN (Data Source Name):

```yaml
database:
  sql:
    driver: postgres
    dsn: postgres://user:password@db.example.com:5432/ocpdb?sslmode=disable
```

Replace:

- `driver` with `postgres`, `mysql`, or `sqlite3`
- `dsn` with your database connection string

**Example: SQLite3 (file-based)**

```
database:
  sql:
    driver: sqlite3
    dsn: /var/lib/ocp/ocp.db
```

### **Amazon RDS Configuration**

OCP supports direct configuration for Amazon RDS with optional high-availability setups. It also currently supports the postgres, pgx, or mysql drivers for AWS RDS.

```yaml
database:
  aws_rds:
    region: us-east-1
    endpoint: mydb.cluster-abcdefghijkl.us-east-1.rds.amazonaws.com:5432
    driver: postgres
    database_user: ocpuser
    database_name: ocpdb
    credentials:
      name: rds-credentials
    root_certificates: /etc/ssl/certs/rds-combined-ca-bundle.pem
```

**Key fields:**

- `region`: AWS region of the RDS instance
- `endpoint`: Hostname and port for your RDS cluster or instance
- `driver`: `postgres, pgx` or `mysql`
- `database_user`: Database user for OCP
- `database_name`: Database name for OCP
- `credentials`: Reference to a secret containing the password or AWS credentials
- `root_certificates`: Optional PEM-encoded root certificate bundle for TLS connections to RDS

### High Availability with AWS RDS

To run OCP with a highly available RDS setup:

1. **Use Multi-AZ Deployment**
   In the AWS Console or via Terraform/CloudFormation, enable Multi-AZ when creating your RDS instance or cluster. This provisions standby replicas in a different Availability Zone and enables automatic failover.
2. **Cluster Endpoints**
   Use the **RDS cluster endpoint** rather than an instance-specific endpoint. This ensures that OCP always connects to the primary writer node, even after a failover.
   Example:

```yaml
endpoint: mydb.cluster-abcdefghijkl.us-east-1.rds.amazonaws.com:5432
```

1. **Read Replicas for Scaling**
   If your OCP deployment requires read scaling, you can configure read replicas in RDS. This is generally not required for OCP itself, but can be useful for analytics or reporting workloads.
2. **TLS Encryption**
   Download the latest Amazon RDS CA bundle from: [https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html) Store it in your container or host, and reference it in `root_certificates`.
3. **Secrets Management**
   Store the database password as well as AWS credentials in an OCP secret, such as:

```yaml
secrets:
  rds-credentials:
    password: ${RDS_PASSWORD}
```

**Additional References:**

- [Amazon RDS Multi-AZ Deployments](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.MultiAZ.html)

- [Amazon RDS Cluster Endpoints](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.Overview.Endpoints.html)

- [Amazon RDS SSL Support](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html)
