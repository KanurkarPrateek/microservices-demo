# Database Persistence Layer - Deployment Guide

This guide explains how to deploy the microservices-demo application with PostgreSQL database persistence for order data.

## Overview

The checkout service has been enhanced to persist order data to a PostgreSQL database. This enables:
- Permanent storage of all order information
- Order history tracking per user
- Audit trail for transactions
- Data analytics capabilities

## Architecture Changes

### Database Schema

Two tables have been added:

1. **orders** - Stores order header information:
   - order_id (unique identifier)
   - user_id, user_email, user_currency
   - shipping details (tracking ID, cost, address)
   - total amount
   - timestamps (created_at, updated_at)

2. **order_items** - Stores line items for each order:
   - product_id, quantity, cost
   - Links to orders table via order_id foreign key

### Modified Components

1. **checkoutservice** - Enhanced with database persistence:
   - New file: `database.go` - Database access layer
   - Modified: `main.go` - Integration with order persistence
   - Modified: `go.mod` - Added PostgreSQL driver dependency

2. **Kubernetes manifests**:
   - New: `postgres.yaml` - PostgreSQL deployment
   - Modified: `checkoutservice.yaml` - Database connection configuration

## Deployment Instructions

### Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl configured to access your cluster
- Sufficient storage for PostgreSQL PersistentVolume (5Gi default)

### Step 1: Deploy PostgreSQL Database

Deploy the PostgreSQL database first:

```bash
kubectl apply -f kubernetes-manifests/postgres.yaml
```

This creates:
- PostgreSQL deployment (single replica)
- PersistentVolumeClaim (5Gi)
- Service (postgres:5432)
- ConfigMaps for database credentials and initialization script

Wait for the database to be ready:

```bash
kubectl wait --for=condition=ready pod -l app=postgres --timeout=120s
```

Verify the database is running:

```bash
kubectl get pods -l app=postgres
kubectl logs -l app=postgres
```

### Step 2: Rebuild Checkout Service

The checkout service needs to be rebuilt with the updated dependencies:

```bash
cd src/checkoutservice
docker build -t <your-registry>/checkoutservice:latest .
docker push <your-registry>/checkoutservice:latest
```

Or if using Skaffold:

```bash
cd microservices-demo
skaffold build -p dev
```

### Step 3: Deploy/Update Services

Deploy all services including the updated checkout service:

```bash
kubectl apply -f kubernetes-manifests/
```

Or if using existing deployments, just update the checkout service:

```bash
kubectl apply -f kubernetes-manifests/checkoutservice.yaml
kubectl rollout restart deployment/checkoutservice
```

### Step 4: Verify Deployment

Check all pods are running:

```bash
kubectl get pods
```

Check checkout service logs for database connection:

```bash
kubectl logs -l app=checkoutservice -f
```

You should see log entries like:
```
{"severity":"info","message":"Database URL provided, initializing database connection"}
{"severity":"info","message":"Successfully connected to PostgreSQL database"}
{"severity":"info","message":"Database connection established successfully"}
```

### Step 5: Test Order Persistence

1. Access the frontend service
2. Place a test order
3. Check the checkout service logs:

```bash
kubectl logs -l app=checkoutservice | grep "saved to database"
```

You should see:
```
{"severity":"info","message":"order <order-id> saved to database successfully"}
```

4. Verify data in the database:

```bash
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "SELECT order_id, user_id, created_at FROM orders ORDER BY created_at DESC LIMIT 5;"
```

## Database Configuration

### Environment Variables (checkoutservice)

- `DATABASE_URL`: PostgreSQL connection string
  - Format: `postgres://username:password@host:port/database?sslmode=disable`
  - Default: `postgres://orderservice:orderpass123@postgres:5432/ordersdb?sslmode=disable`

### Database Credentials

Default credentials (defined in `postgres.yaml`):
- Database: `ordersdb`
- User: `orderservice`
- Password: `orderpass123`

**⚠️ IMPORTANT**: For production deployments:
1. Use Kubernetes Secrets instead of ConfigMaps for credentials
2. Change default passwords
3. Enable SSL/TLS connections
4. Use managed database services (Cloud SQL, RDS, etc.)

### Example using Kubernetes Secrets:

```bash
# Create secret
kubectl create secret generic postgres-credentials \
  --from-literal=username=orderservice \
  --from-literal=password=<strong-password> \
  --from-literal=database=ordersdb

# Update checkoutservice.yaml to reference the secret
# (replace the DATABASE_URL value with secretKeyRef)
```

## Storage Configuration

### PersistentVolumeClaim

Default storage request: 5Gi

To modify storage size, edit `postgres.yaml`:

```yaml
spec:
  resources:
    requests:
      storage: 10Gi  # Change this value
```

### Storage Classes

The PVC uses the default storage class. To specify a different storage class:

```yaml
spec:
  storageClassName: fast-ssd  # Add this line
  accessModes:
    - ReadWriteOnce
```

## High Availability Considerations

The current deployment uses a single PostgreSQL instance. For production:

### Option 1: Use Managed Database Services
- Google Cloud SQL
- AWS RDS
- Azure Database for PostgreSQL
- Simply update the `DATABASE_URL` to point to the managed instance

### Option 2: Deploy PostgreSQL with Replication
- Use a PostgreSQL operator (e.g., Zalando PostgreSQL Operator)
- Configure streaming replication
- Set up automatic failover

## Backup and Recovery

### Manual Backup

```bash
# Backup
kubectl exec deployment/postgres -- pg_dump -U orderservice ordersdb > backup.sql

# Restore
kubectl exec -i deployment/postgres -- psql -U orderservice ordersdb < backup.sql
```

### Automated Backups

Consider implementing:
- Kubernetes CronJobs for scheduled backups
- Velero for cluster-wide backup/restore
- Cloud provider backup solutions

## Monitoring

### Database Metrics

Monitor these key metrics:
- Connection count
- Query performance
- Disk usage
- Replication lag (if using HA setup)

### Query Database Statistics

```bash
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "
SELECT 
    schemaname,
    tablename,
    n_tup_ins AS inserts,
    n_tup_upd AS updates,
    n_tup_del AS deletes
FROM pg_stat_user_tables 
WHERE schemaname = 'public';
"
```

## Troubleshooting

### Checkout Service Can't Connect to Database

1. Check if PostgreSQL is running:
   ```bash
   kubectl get pods -l app=postgres
   ```

2. Check PostgreSQL logs:
   ```bash
   kubectl logs -l app=postgres
   ```

3. Verify service DNS resolution:
   ```bash
   kubectl exec -it deployment/checkoutservice -- nslookup postgres
   ```

4. Test database connectivity:
   ```bash
   kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "SELECT 1;"
   ```

### Orders Not Being Saved

1. Check checkout service logs:
   ```bash
   kubectl logs -l app=checkoutservice | grep -i "database\|order"
   ```

2. Verify DATABASE_URL environment variable:
   ```bash
   kubectl get deployment checkoutservice -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="DATABASE_URL")].value}'
   ```

3. Check database tables exist:
   ```bash
   kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "\dt"
   ```

### Database Connection Pool Exhausted

Adjust connection pool settings in `database.go`:
```go
db.SetMaxOpenConns(50)  // Increase from 25
db.SetMaxIdleConns(10)  // Increase from 5
```

## Security Best Practices

1. **Use Secrets**: Store credentials in Kubernetes Secrets, not ConfigMaps
2. **Network Policies**: Restrict database access to only checkoutservice
3. **SSL/TLS**: Enable encrypted connections (set `sslmode=require`)
4. **RBAC**: Limit database user permissions to only required operations
5. **Regular Updates**: Keep PostgreSQL version updated for security patches

## Performance Tuning

### PostgreSQL Configuration

For production workloads, tune PostgreSQL parameters:

```yaml
env:
  - name: POSTGRES_INITDB_ARGS
    value: "-E UTF8 --locale=C"
  - name: POSTGRES_MAX_CONNECTIONS
    value: "100"
  - name: POSTGRES_SHARED_BUFFERS
    value: "256MB"
```

### Indexing

The schema includes indexes on:
- `orders.user_id` - For user order history queries
- `orders.order_id` - For order lookup
- `orders.created_at` - For time-based queries
- `order_items.order_id` - For join performance

## Migration from Existing Deployment

If you have an existing deployment without the database:

1. Deploy PostgreSQL first
2. Update checkoutservice with the new code
3. All new orders will be persisted
4. Historical orders (before the update) won't be in the database

To backfill historical data, you'd need to:
1. Extract order data from logs or other sources
2. Write a migration script to insert into the database

## Cleanup

To remove the database and all data:

```bash
kubectl delete -f kubernetes-manifests/postgres.yaml
kubectl delete pvc postgres-pv-claim
```

**⚠️ WARNING**: This will permanently delete all order data!

## Additional Resources

- [PostgreSQL Documentation](https://www.postgresql.org/docs/)
- [Kubernetes Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Go PostgreSQL Driver (lib/pq)](https://github.com/lib/pq)

## Support

For issues or questions:
1. Check the troubleshooting section above
2. Review logs from both checkoutservice and postgres pods
3. Verify network connectivity between services
